package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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
	Kind    string // "thinking" | "tool_call" | "done"
	Message string // human-readable description
}

// ProgressFunc receives events as the agent works.
type ProgressFunc func(Event)

const systemPrompt = `You evaluate whether a condition holds for a codebase. Be efficient.

Tools: read_file, glob, grep, list_dir. Use grep to locate relevant code quickly rather than reading entire files.

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

// RunOptions configures the agent loop.
type RunOptions struct {
	MaxIterations int
	MaxTokens     int
}

func Run(ctx context.Context, apiKey string, model string, condition string, onGlobs []string, repoRoot string, progress ProgressFunc, opts RunOptions) (Result, error) {
	if progress == nil {
		progress = func(Event) {}
	}
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 30
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 10000
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

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

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg.String())),
	}
	tools := ToolDefs()

	var usage Usage

	maxIterations := opts.MaxIterations
	for i := 0; i < maxIterations; i++ {
		progress(Event{Kind: "thinking", Message: fmt.Sprintf("thinking (turn %d/%d)", i+1, maxIterations)})

		resp, err := callWithRetry(ctx, func() (*anthropic.Message, error) {
			return client.Messages.New(ctx, anthropic.MessageNewParams{
				Model:     anthropic.Model(model),
				MaxTokens: int64(opts.MaxTokens),
				System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
				Messages:  messages,
				Tools:     tools,
			})
		})
		if err != nil {
			return Result{Usage: usage}, fmt.Errorf("API call failed: %w", err)
		}

		usage.APICalls++
		usage.InputTokens += int(resp.Usage.InputTokens)
		usage.OutputTokens += int(resp.Usage.OutputTokens)

		var toolResults []anthropic.ContentBlockParamUnion
		var finalText strings.Builder

		for _, block := range resp.Content {
			switch v := block.AsAny().(type) {
			case anthropic.ToolUseBlock:
				summary := formatToolCall(v.Name, v.Input)
				progress(Event{Kind: "tool_call", Message: summary})
				result, isError := ExecuteTool(v.Name, v.Input, repoRoot)
				toolResults = append(toolResults, anthropic.NewToolResultBlock(v.ID, result, isError))
			case anthropic.TextBlock:
				finalText.WriteString(v.Text)
			}
		}

		if len(toolResults) > 0 {
			messages = append(messages, resp.ToParam())
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
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

// callWithRetry retries the API call on 429 rate limit errors with exponential backoff.
func callWithRetry(ctx context.Context, fn func() (*anthropic.Message, error)) (*anthropic.Message, error) {
	delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second}
	for attempt, maxAttempts := 0, len(delays)+1; attempt < maxAttempts; attempt++ {
		msg, err := fn()
		if err == nil {
			return msg, nil
		}
		errStr := err.Error()
		isRateLimit := strings.Contains(errStr, "429") || strings.Contains(errStr, "rate_limit_error")
		if !isRateLimit || attempt == maxAttempts-1 {
			return nil, err
		}
		delay := delays[attempt]
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("unreachable")
}

// formatToolCall produces a readable summary like: grep("UPDATE.*WHERE status")
func formatToolCall(name string, input json.RawMessage) string {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return name + "(…)"
	}

	switch name {
	case "read_file":
		return fmt.Sprintf("read  %s", args["path"])
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
