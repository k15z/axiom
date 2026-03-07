package scaffold

import (
	"context"
	"fmt"
	"strings"

	"github.com/k15z/axiom/internal/agent"
	"github.com/k15z/axiom/internal/provider"
)

const addSystemPrompt = `You are exploring a codebase to generate a single behavioral test for axiom, an AI-driven test framework that verifies code intent.

The user will provide a natural-language description of what they want to test. Your job is to:
1. Explore the codebase to understand the relevant area
2. Generate exactly ONE test that captures the user's intent

Axiom tests verify architectural intent and invariants, NOT implementation details.

Output ONLY a YAML block wrapped in ` + "```yaml" + ` fences with exactly one test. Use this format:

` + "```yaml" + `
test_descriptive_name:
  on:
    - "glob/pattern/**/*.ext"
  condition: >
    A clear, specific description of what architectural property
    should hold. Reference concrete packages, layers, or patterns
    in the codebase.
` + "```" + `

The "on" field should contain glob patterns matching the files relevant to the test.
The "condition" field should be a plain-English assertion that an AI agent can verify by reading the code.
Make the condition specific and grounded in what you actually find in the codebase.`

// GenerateTest uses the LLM to explore the codebase and generate a single test from a natural-language intent.
func GenerateTest(ctx context.Context, p provider.Provider, model, repoRoot, intent string, progress ProgressFunc) (string, error) {
	if progress == nil {
		progress = func(string) {}
	}

	tools := agent.ToolDefs()

	projectContext := DetectContext(repoRoot)
	userMsg := fmt.Sprintf("Generate a behavioral test for the following intent:\n\n%s\n\nProject context:\n%s", intent, projectContext)

	messages := []provider.Message{
		{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: userMsg}}},
	}

	maxIterations := 20
	for i := 0; i < maxIterations; i++ {
		progress(fmt.Sprintf("thinking (turn %d/%d)", i+1, maxIterations))

		resp, err := p.Chat(ctx, provider.ChatParams{
			Model:     model,
			System:    addSystemPrompt,
			Messages:  messages,
			Tools:     tools,
			MaxTokens: 16000,
		})
		if err != nil {
			return "", fmt.Errorf("API call failed: %w", err)
		}

		var toolResults []provider.ContentBlock
		var assistantBlocks []provider.ContentBlock
		var finalText strings.Builder

		for _, block := range resp.Content {
			assistantBlocks = append(assistantBlocks, block)
			switch block.Type {
			case "tool_use":
				summary := formatToolCall(block.ToolName, block.Input)
				progress(summary)
				result, isError := agent.ExecuteTool(ctx, block.ToolName, block.Input, repoRoot, 0)
				toolResults = append(toolResults, provider.ContentBlock{
					Type:     "tool_result",
					ToolID:   block.ToolID,
					ToolName: block.ToolName,
					Text:     result,
					IsError:  isError,
				})
			case "text":
				finalText.WriteString(block.Text)
			}
		}

		if len(toolResults) > 0 {
			messages = append(messages, provider.Message{Role: "assistant", Content: assistantBlocks})
			messages = append(messages, provider.Message{Role: "user", Content: toolResults})
			continue
		}

		yaml := extractYAML(finalText.String())
		if yaml == "" {
			return "", fmt.Errorf("LLM did not produce a YAML block")
		}
		return yaml, nil
	}

	return "", fmt.Errorf("exceeded maximum iterations without generating a test")
}
