package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/k15z/axiom/internal/provider"
)

type Usage struct {
	InputTokens  int
	OutputTokens int
	APICalls     int
}

type Result struct {
	Passed    bool
	Reasoning string
	Usage     Usage
}

// Event is emitted by the agent during execution to report progress.
type Event struct {
	Kind    string // "thinking" | "tool_call" | "text"
	Message string // human-readable description
}

// ProgressFunc receives events as the agent works.
type ProgressFunc func(Event)

const systemPrompt = `You evaluate whether a condition holds for a codebase. Be efficient.

Tools: read_file (supports start_line/end_line for partial reads), glob, grep, list_dir, tree. Use tree to understand project structure, grep to locate relevant code quickly, then read_file with line ranges instead of reading entire files.

Strategy:
1. Start with the hinted files/patterns
2. Grep first, then read only what you need
3. Decide as soon as you have enough evidence

Respond with exactly:

VERDICT: PASS
<brief reasoning with file paths and line numbers>

or

VERDICT: FAIL
<brief reasoning explaining what's missing or wrong>`

const tokenBudgetHint = "You are running low on your token budget. Please state your verdict now using the VERDICT: PASS or VERDICT: FAIL format, with brief reasoning."

// shouldInjectBudgetHint returns true when total token usage has reached >=75%
// of the estimated budget. The budget estimate is maxTokens * 3 to account for
// both input (~2x) and output (~1x) token usage.
func shouldInjectBudgetHint(inputTokens, outputTokens, maxTokens int) bool {
	if maxTokens <= 0 {
		return false
	}
	totalTokens := inputTokens + outputTokens
	budget := maxTokens * 3
	return totalTokens >= budget*3/4
}

// RunOptions configures the agent loop.
type RunOptions struct {
	MaxIterations int
	MaxTokens     int
	ToolTimeout   time.Duration // per-tool timeout; 0 means no timeout
}

func Run(ctx context.Context, p provider.Provider, model string, condition string, onGlobs []string, repoRoot string, progress ProgressFunc, opts RunOptions) (Result, error) {
	if progress == nil {
		progress = func(Event) {}
	}
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 30
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 10000
	}

	var userMsg strings.Builder
	userMsg.WriteString("Condition: ")
	userMsg.WriteString(condition)
	if len(onGlobs) > 0 {
		userMsg.WriteString("\n\nStart with: ")
		for i, g := range onGlobs {
			if i > 0 {
				userMsg.WriteString(", ")
			}
			userMsg.WriteString(g)
		}
	}

	messages := []provider.Message{
		{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: userMsg.String()}}},
	}
	tools := ToolDefs()

	var usage Usage
	budgetHintInjected := false

	maxIterations := opts.MaxIterations
	for i := 0; i < maxIterations; i++ {
		progress(Event{Kind: "thinking", Message: fmt.Sprintf("thinking (turn %d/%d)", i+1, maxIterations)})

		resp, err := p.Chat(ctx, provider.ChatParams{
			Model:     model,
			System:    systemPrompt,
			Messages:  messages,
			Tools:     tools,
			MaxTokens: opts.MaxTokens,
		})
		if err != nil {
			return Result{Usage: usage}, fmt.Errorf("API call failed: %w", err)
		}

		usage.APICalls++
		usage.InputTokens += resp.Usage.InputTokens
		usage.OutputTokens += resp.Usage.OutputTokens

		var toolResults []provider.ContentBlock
		var finalText strings.Builder

		// Build the assistant message for conversation history
		var assistantBlocks []provider.ContentBlock

		for _, block := range resp.Content {
			assistantBlocks = append(assistantBlocks, block)
			switch block.Type {
			case "tool_use":
				summary := formatToolCall(block.ToolName, block.Input)
				progress(Event{Kind: "tool_call", Message: summary})
				result, isError := ExecuteTool(ctx, block.ToolName, block.Input, repoRoot, opts.ToolTimeout)
				toolResults = append(toolResults, provider.ContentBlock{
					Type:     "tool_result",
					ToolID:   block.ToolID,
					ToolName: block.ToolName,
					Text:     result,
					IsError:  isError,
				})
			case "text":
				progress(Event{Kind: "text", Message: block.Text})
				finalText.WriteString(block.Text)
			}
		}

		if len(toolResults) > 0 {
			messages = append(messages, provider.Message{
				Role:    "assistant",
				Content: assistantBlocks,
			})

			// Inject a budget hint when the agent has used >=75% of its
			// total token budget and is still making tool calls.
			if !budgetHintInjected && shouldInjectBudgetHint(usage.InputTokens, usage.OutputTokens, opts.MaxTokens) {
				budgetHintInjected = true
				toolResults = append(toolResults, provider.ContentBlock{
					Type: "text",
					Text: tokenBudgetHint,
				})
			}

			messages = append(messages, provider.Message{
				Role:    "user",
				Content: toolResults,
			})
			continue
		}

		r := parseVerdict(finalText.String())
		r.Usage = usage
		return r, nil
	}

	return Result{
		Passed:    false,
		Reasoning: "Agent exceeded maximum iterations without reaching a verdict",
		Usage:     usage,
	}, nil
}

// formatToolCall produces a readable summary like: grep("UPDATE.*WHERE status")
func formatToolCall(name string, input json.RawMessage) string {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return name + "(…)"
	}

	switch name {
	case "read_file":
		s := fmt.Sprintf("read  %s", args["path"])
		if start, ok := args["start_line"].(float64); ok {
			if end, ok := args["end_line"].(float64); ok {
				s += fmt.Sprintf(":%d-%d", int(start), int(end))
			} else {
				s += fmt.Sprintf(":%d-", int(start))
			}
		}
		return s
	case "list_dir":
		return fmt.Sprintf("ls    %s", args["path"])
	case "glob":
		return fmt.Sprintf("glob  %s", args["pattern"])
	case "grep":
		if g, ok := args["glob"].(string); ok && g != "" {
			return fmt.Sprintf("grep  %s  [%s]", args["pattern"], g)
		}
		return fmt.Sprintf("grep  %s", args["pattern"])
	default:
		return name + "(…)"
	}
}

func parseVerdict(text string) Result {
	upper := strings.ToUpper(text)

	if idx := strings.Index(upper, "VERDICT: PASS"); idx != -1 {
		reasoning := strings.TrimSpace(text[idx+len("VERDICT: PASS"):])
		return Result{Passed: true, Reasoning: reasoning}
	}
	if idx := strings.Index(upper, "VERDICT: FAIL"); idx != -1 {
		reasoning := strings.TrimSpace(text[idx+len("VERDICT: FAIL"):])
		return Result{Passed: false, Reasoning: reasoning}
	}

	return Result{
		Passed:    false,
		Reasoning: fmt.Sprintf("Could not parse verdict from agent response:\n%s", text),
	}
}
