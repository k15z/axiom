package scaffold

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/k15z/axiom/internal/agent"
	"github.com/k15z/axiom/internal/provider"
)

const initSystemPrompt = `You are exploring a codebase to generate behavioral tests for axiom, an AI-driven test framework that verifies code intent.

Axiom tests verify architectural intent and invariants, NOT implementation details.

Good tests check things like:
- Dependency boundaries (package A should never import package B)
- Security invariants (all user input is validated/sanitized before use)
- Consistency (all API endpoints follow the same error response format)
- Concurrency safety (shared state is protected by locks or atomic ops)
- Separation of concerns (no business logic in the transport layer)
- Resource cleanup (all opened connections/files are properly closed)

Bad tests (avoid these):
- Style/linting (naming conventions, formatting, comment presence)
- Trivial checks (files exist, functions are exported)
- Implementation details (specific variable names, exact line counts)
- Things a linter would catch

Explore the codebase using the provided tools. Understand the architecture, then generate 3-5 high-quality behavioral tests.

Output ONLY a YAML block wrapped in ` + "```yaml" + ` fences. Use this format:

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
The "condition" field should be a plain-English assertion that an AI agent can verify by reading the code.`

// ProgressFunc receives status updates during generation.
type ProgressFunc func(message string)

// DetectContext gathers lightweight project info to seed the LLM prompt.
func DetectContext(root string) string {
	var b strings.Builder

	// Check for language marker files
	markers := []struct {
		file string
		desc string
	}{
		{"go.mod", "Go module"},
		{"package.json", "Node.js/JavaScript project"},
		{"pyproject.toml", "Python project (pyproject.toml)"},
		{"requirements.txt", "Python project (requirements.txt)"},
		{"setup.py", "Python project (setup.py)"},
		{"Cargo.toml", "Rust project"},
		{"pom.xml", "Java/Maven project"},
		{"build.gradle", "Java/Gradle project"},
		{"Gemfile", "Ruby project"},
		{"mix.exs", "Elixir project"},
		{"composer.json", "PHP project"},
	}

	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(root, m.file)); err == nil {
			fmt.Fprintf(&b, "Project type: %s (%s found)\n", m.desc, m.file)
		}
	}

	// Top-level directory listing
	entries, err := os.ReadDir(root)
	if err == nil {
		b.WriteString("\nTop-level contents:\n")
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			suffix := ""
			if e.IsDir() {
				suffix = "/"
			}
			fmt.Fprintf(&b, "  %s%s\n", e.Name(), suffix)
		}
	}

	// Count file extensions for language detection
	extCounts := make(map[string]int)
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true,
		"__pycache__": true, "target": true, ".axiom": true,
		"dist": true, "build": true, ".next": true,
	}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			ext := filepath.Ext(d.Name())
			if ext != "" {
				extCounts[ext]++
			}
		}
		return nil
	})

	if len(extCounts) > 0 {
		b.WriteString("\nFile extension counts:\n")
		// Show top extensions
		type extCount struct {
			ext   string
			count int
		}
		var sorted []extCount
		for ext, count := range extCounts {
			sorted = append(sorted, extCount{ext, count})
		}
		// Simple sort by count desc
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].count > sorted[i].count {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		for i, ec := range sorted {
			if i >= 10 {
				break
			}
			fmt.Fprintf(&b, "  %s: %d\n", ec.ext, ec.count)
		}
	}

	return b.String()
}

// GenerateTests uses the LLM to explore the codebase and generate test YAML.
func GenerateTests(ctx context.Context, p provider.Provider, model, repoRoot string, progress ProgressFunc) (string, error) {
	if progress == nil {
		progress = func(string) {}
	}

	tools := agent.ToolDefs()

	projectContext := DetectContext(repoRoot)
	userMsg := fmt.Sprintf("Explore this codebase and generate behavioral tests.\n\nProject context:\n%s", projectContext)

	messages := []provider.Message{
		{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: userMsg}}},
	}

	maxIterations := 20
	for i := 0; i < maxIterations; i++ {
		progress(fmt.Sprintf("thinking (turn %d/%d)", i+1, maxIterations))

		resp, err := p.Chat(ctx, provider.ChatParams{
			Model:     model,
			System:    initSystemPrompt,
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

		// Extract YAML from the response
		yaml := extractYAML(finalText.String())
		if yaml == "" {
			return "", fmt.Errorf("LLM did not produce a YAML block")
		}
		return yaml, nil
	}

	return "", fmt.Errorf("exceeded maximum iterations without generating tests")
}

// extractYAML pulls the content between ```yaml and ``` fences.
func extractYAML(text string) string {
	re := regexp.MustCompile("(?s)```ya?ml\n(.*?)```")
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func formatToolCall(name string, input json.RawMessage) string {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return name + "(…)"
	}
	switch name {
	case "read_file":
		return fmt.Sprintf("reading %s", args["path"])
	case "list_dir":
		return fmt.Sprintf("listing %s", args["path"])
	case "glob":
		return fmt.Sprintf("glob %s", args["pattern"])
	case "grep":
		if g, ok := args["glob"].(string); ok && g != "" {
			return fmt.Sprintf("grep %s [%s]", args["pattern"], g)
		}
		return fmt.Sprintf("grep %s", args["pattern"])
	default:
		return name + "(…)"
	}
}
