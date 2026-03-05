package scaffold

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/k15z/axiom/internal/agent"
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
func GenerateTest(ctx context.Context, apiKey, model, repoRoot, intent string, progress ProgressFunc) (string, error) {
	if progress == nil {
		progress = func(string) {}
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	tools := agent.ToolDefs()

	projectContext := DetectContext(repoRoot)
	userMsg := fmt.Sprintf("Generate a behavioral test for the following intent:\n\n%s\n\nProject context:\n%s", intent, projectContext)

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
	}

	maxIterations := 20
	for i := 0; i < maxIterations; i++ {
		progress(fmt.Sprintf("thinking (turn %d/%d)", i+1, maxIterations))

		resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: int64(16000),
			System:    []anthropic.TextBlockParam{{Text: addSystemPrompt}},
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			return "", fmt.Errorf("API call failed: %w", err)
		}

		var toolResults []anthropic.ContentBlockParamUnion
		var finalText strings.Builder

		for _, block := range resp.Content {
			switch v := block.AsAny().(type) {
			case anthropic.ToolUseBlock:
				summary := formatToolCall(v.Name, v.Input)
				progress(summary)
				result, isError := agent.ExecuteTool(v.Name, v.Input, repoRoot)
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

		yaml := extractYAML(finalText.String())
		if yaml == "" {
			return "", fmt.Errorf("LLM did not produce a YAML block")
		}
		return yaml, nil
	}

	return "", fmt.Errorf("exceeded maximum iterations without generating a test")
}
