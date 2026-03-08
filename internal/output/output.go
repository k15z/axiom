package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/k15z/axiom/internal/types"
)

var (
	green  = color.New(color.FgGreen)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	gray   = color.New(color.FgHiBlack)
	bold   = color.New(color.Bold)
)

func Print(results []types.TestResult, model string, verbose bool, testDir string) {
	fmt.Println()
	bold.Println("  axiom")
	fmt.Println()

	// Group by source file
	groups := make(map[string][]types.TestResult)
	var order []string
	for _, r := range results {
		if _, seen := groups[r.Test.SourceFile]; !seen {
			order = append(order, r.Test.SourceFile)
		}
		groups[r.Test.SourceFile] = append(groups[r.Test.SourceFile], r)
	}

	for _, file := range order {
		gray.Printf("  %s%s\n", testDir, file)
		for _, r := range groups[file] {
			switch {
			case r.Cached:
				gray.Printf("    ○ %s (cached)\n", r.Test.Name)
			case r.Skipped:
				gray.Printf("    ○ %s (skipped)\n", r.Test.Name)
			case r.Errored:
				red.Printf("    ! ")
				fmt.Printf("%s", r.Test.Name)
				gray.Printf(" (error, %.1fs)\n", r.Duration.Seconds())
				printCondition(r.Test.Condition, "      ")
				if r.Reasoning != "" {
					printFailureReasoning(r.Reasoning, "      ")
				}
			case r.Flaky:
				yellow.Printf("    ⚠ ")
				fmt.Printf("%s", r.Test.Name)
				gray.Printf(" (flaky, passed on retry %d, %.1fs)\n", r.Retries, r.Duration.Seconds())
				if r.Reasoning != "" {
					if verbose {
						printReasoning(r.Reasoning, "      ")
					} else {
						gray.Printf("      %s\n", firstLine(r.Reasoning))
					}
				}
			case r.Passed:
				green.Printf("    ✓ ")
				fmt.Printf("%s", r.Test.Name)
				gray.Printf(" (%.1fs)\n", r.Duration.Seconds())
				if r.Reasoning != "" {
					if verbose {
						printReasoning(r.Reasoning, "      ")
					} else {
						gray.Printf("      %s\n", firstLine(r.Reasoning))
					}
				}
			default:
				red.Printf("    ✗ ")
				fmt.Printf("%s", r.Test.Name)
				gray.Printf(" (%.1fs)\n", r.Duration.Seconds())
				printCondition(r.Test.Condition, "      ")
				if r.Reasoning != "" {
					printFailureReasoning(r.Reasoning, "      ")
				}
			}
			if r.Usage.APICalls > 0 {
				cost := estimateCost(model, r.Usage.InputTokens, r.Usage.OutputTokens)
				gray.Printf("      %d calls · %s in · %s out · ~$%.4f\n",
					r.Usage.APICalls,
					formatTokens(r.Usage.InputTokens),
					formatTokens(r.Usage.OutputTokens),
					cost)
			}
		}
		fmt.Println()
	}

	// Summary
	passed, failed, cached, skipped, flaky, errored := 0, 0, 0, 0, 0, 0
	var totalUsage types.Usage
	for _, r := range results {
		switch {
		case r.Cached:
			cached++
		case r.Skipped:
			skipped++
		case r.Errored:
			errored++
		case r.Flaky:
			flaky++
		case r.Passed:
			passed++
		default:
			failed++
		}
		totalUsage.InputTokens += r.Usage.InputTokens
		totalUsage.OutputTokens += r.Usage.OutputTokens
		totalUsage.APICalls += r.Usage.APICalls
	}

	fmt.Print("  ")
	parts := 0
	if passed > 0 {
		green.Printf("%d passed", passed)
		parts++
	}
	if flaky > 0 {
		if parts > 0 {
			fmt.Print(" · ")
		}
		yellow.Printf("%d flaky", flaky)
		parts++
	}
	if failed > 0 {
		if parts > 0 {
			fmt.Print(" · ")
		}
		red.Printf("%d failed", failed)
		parts++
	}
	if errored > 0 {
		if parts > 0 {
			fmt.Print(" · ")
		}
		red.Printf("%d errored", errored)
		parts++
	}
	if cached > 0 {
		if parts > 0 {
			fmt.Print(" · ")
		}
		gray.Printf("%d cached", cached)
		parts++
	}
	if skipped > 0 {
		if parts > 0 {
			fmt.Print(" · ")
		}
		gray.Printf("%d skipped", skipped)
	}
	fmt.Println()

	// Token/cost summary
	if totalUsage.APICalls > 0 {
		cost := estimateCost(model, totalUsage.InputTokens, totalUsage.OutputTokens)
		gray.Printf("  %d API calls · %s tokens · ~$%.4f\n",
			totalUsage.APICalls,
			formatTokens(totalUsage.InputTokens+totalUsage.OutputTokens),
			cost,
		)
	}
	fmt.Println()

	// CI-friendly summary on stderr (single greppable line)
	fmt.Fprintf(os.Stderr, "axiom: %s\n", CISummary(passed, failed, errored, cached, skipped, flaky))
}

// CISummary returns a single-line summary string suitable for CI logs.
func CISummary(passed, failed, errored, cached, skipped, flaky int) string {
	var parts []string
	if passed > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", passed))
	}
	if flaky > 0 {
		parts = append(parts, fmt.Sprintf("%d flaky", flaky))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if errored > 0 {
		parts = append(parts, fmt.Sprintf("%d errored", errored))
	}
	if cached > 0 {
		parts = append(parts, fmt.Sprintf("%d cached", cached))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	if len(parts) == 0 {
		return "no tests ran"
	}
	return strings.Join(parts, ", ")
}

func PrintJSON(results []types.TestResult, model string) error {
	type jsonUsage struct {
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		APICalls     int     `json:"api_calls"`
		CostUSD      float64 `json:"cost_usd"`
	}
	type jsonResult struct {
		Name      string    `json:"name"`
		File      string    `json:"file"`
		Passed    bool      `json:"passed"`
		Cached    bool      `json:"cached"`
		Errored   bool      `json:"errored,omitempty"`
		Flaky     bool      `json:"flaky,omitempty"`
		Retries   int       `json:"retries,omitempty"`
		Reasoning string    `json:"reasoning,omitempty"`
		Duration  float64   `json:"duration_seconds,omitempty"`
		Usage     jsonUsage `json:"usage"`
	}

	var out []jsonResult
	for _, r := range results {
		out = append(out, jsonResult{
			Name:      r.Test.Name,
			File:      r.Test.SourceFile,
			Passed:    r.Passed,
			Cached:    r.Cached,
			Errored:   r.Errored,
			Flaky:     r.Flaky,
			Retries:   r.Retries,
			Reasoning: r.Reasoning,
			Duration:  r.Duration.Seconds(),
			Usage: jsonUsage{
				InputTokens:  r.Usage.InputTokens,
				OutputTokens: r.Usage.OutputTokens,
				APICalls:     r.Usage.APICalls,
				CostUSD:      estimateCost(model, r.Usage.InputTokens, r.Usage.OutputTokens),
			},
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx != -1 {
		s = s[:idx]
	}
	if len(s) > 100 {
		s = s[:97] + "..."
	}
	return s
}

func printReasoning(reasoning, indent string) {
	for line := range strings.SplitSeq(strings.TrimSpace(reasoning), "\n") {
		gray.Printf("%s%s\n", indent, line)
	}
}

// printFailureReasoning prints reasoning in default text color instead of gray,
// so failure details stand out from cached/skipped/passed output.
func printFailureReasoning(reasoning, indent string) {
	for line := range strings.SplitSeq(strings.TrimSpace(reasoning), "\n") {
		fmt.Printf("%s%s\n", indent, line)
	}
}

// printCondition prints the test condition in default text color, indented,
// so the user can see what was being evaluated alongside the verdict.
// Uses default color (not gray) because the condition is critical context
// for understanding failures and errors.
func printCondition(condition, indent string) {
	if condition == "" {
		return
	}
	// Collapse the condition to a single line for compact display
	collapsed := strings.Join(strings.Fields(condition), " ")
	if len(collapsed) > 120 {
		collapsed = collapsed[:117] + "..."
	}
	fmt.Printf("%scondition: %s\n", indent, collapsed)
}

// estimateCost returns estimated cost in USD based on model pricing.
// Prices are per million tokens.
func estimateCost(model string, inputTokens, outputTokens int) float64 {
	type pricing struct{ input, output float64 }

	// Anthropic: https://docs.anthropic.com/en/docs/about-claude/models
	// OpenAI: https://openai.com/api/pricing/
	// Gemini: https://ai.google.dev/pricing
	prices := map[string]pricing{
		// Anthropic
		"claude-haiku-4-5":           {1.00, 5.00},
		"claude-haiku-4-5-20251001":  {1.00, 5.00},
		"claude-sonnet-4-5":          {3.00, 15.00},
		"claude-sonnet-4-5-20250514": {3.00, 15.00},
		"claude-sonnet-4-6":          {3.00, 15.00},
		"claude-opus-4-6":            {15.00, 75.00},
		// OpenAI
		"gpt-4o":        {2.50, 10.00},
		"gpt-4o-mini":   {0.15, 0.60},
		"gpt-4.1":       {2.00, 8.00},
		"gpt-4.1-mini":  {0.40, 1.60},
		"gpt-4.1-nano":  {0.10, 0.40},
		"o3-mini":       {1.10, 4.40},
		// Gemini
		"gemini-2.0-flash": {0.10, 0.40},
		"gemini-1.5-pro":   {1.25, 5.00},
		"gemini-1.5-flash": {0.075, 0.30},
	}

	p, ok := prices[model]
	if !ok {
		// Default to haiku pricing as a conservative estimate
		p = pricing{1.00, 5.00}
	}

	return (float64(inputTokens)*p.input + float64(outputTokens)*p.output) / 1_000_000
}

// formatTokens formats a token count with K/M suffixes for readability.
func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// PrintDryRun displays which tests would run vs be skipped (cached) and
// estimates the maximum token cost without calling the API.
func PrintDryRun(statuses []types.TestStatus, model string, maxTokensPerTest int, testDir string) {
	fmt.Println()
	bold.Println("  axiom --dry-run")
	fmt.Println()

	// Group by source file (same pattern as Print)
	groups := make(map[string][]types.TestStatus)
	var order []string
	for _, s := range statuses {
		if _, seen := groups[s.Test.SourceFile]; !seen {
			order = append(order, s.Test.SourceFile)
		}
		groups[s.Test.SourceFile] = append(groups[s.Test.SourceFile], s)
	}

	wouldRun := 0
	wouldSkip := 0
	for _, file := range order {
		gray.Printf("  %s%s\n", testDir, file)
		for _, s := range groups[file] {
			if strings.HasPrefix(s.Status, "cached-") {
				gray.Printf("    ○ %s (cached)\n", s.Test.Name)
				wouldSkip++
			} else {
				fmt.Printf("    → %s", s.Test.Name)
				gray.Printf(" (would run)\n")
				wouldRun++
			}
		}
		fmt.Println()
	}

	// Summary line
	fmt.Print("  ")
	parts := 0
	if wouldRun > 0 {
		fmt.Printf("%d would run", wouldRun)
		parts++
	}
	if wouldSkip > 0 {
		if parts > 0 {
			fmt.Print(" · ")
		}
		gray.Printf("%d cached", wouldSkip)
	}
	fmt.Println()

	// Cost estimate: input is typically ~3× maxTokens (file reads + system prompt);
	// output is typically ~0.5× maxTokens (agent rarely hits the ceiling).
	// Labelled as worst-case so users understand this is an upper bound.
	if wouldRun > 0 && maxTokensPerTest > 0 {
		estInput := wouldRun * maxTokensPerTest * 3
		estOutput := wouldRun * maxTokensPerTest / 2
		cost := estimateCost(model, estInput, estOutput)
		gray.Printf("  worst-case cost estimate: ~$%.4f (%s tokens/test × %d tests)\n",
			cost, formatTokens(maxTokensPerTest*3+maxTokensPerTest/2), wouldRun)
	}
	fmt.Println()
}

// PrintGitHub outputs a Markdown summary suitable for posting as a GitHub PR comment.
func PrintGitHub(results []types.TestResult, model string) {
	fmt.Print(FormatGitHub(results, model))
}

// FormatGitHub generates a Markdown summary of test results for GitHub PR comments.
func FormatGitHub(results []types.TestResult, model string) string {
	var b strings.Builder

	b.WriteString("<!-- axiom-summary -->\n")
	b.WriteString("## Axiom Test Results\n\n")
	b.WriteString("| Test | File | Result | Duration |\n")
	b.WriteString("|------|------|--------|----------|\n")

	passed, failed, cached, skipped, flaky, errored := 0, 0, 0, 0, 0, 0
	var totalUsage types.Usage
	var failures []types.TestResult
	var flakyTests []types.TestResult
	var errors []types.TestResult

	for _, r := range results {
		icon := ":white_check_mark: Pass"
		dur := fmt.Sprintf("%.1fs", r.Duration.Seconds())

		switch {
		case r.Cached:
			icon = ":grey_question: Cached"
			dur = "-"
			cached++
		case r.Skipped:
			icon = ":fast_forward: Skipped"
			dur = "-"
			skipped++
		case r.Errored:
			icon = ":boom: Error"
			errored++
			errors = append(errors, r)
		case r.Flaky:
			icon = ":warning: Flaky"
			flaky++
			flakyTests = append(flakyTests, r)
		case r.Passed:
			passed++
		default:
			icon = ":x: Fail"
			failed++
			failures = append(failures, r)
		}

		totalUsage.InputTokens += r.Usage.InputTokens
		totalUsage.OutputTokens += r.Usage.OutputTokens
		totalUsage.APICalls += r.Usage.APICalls

		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", r.Test.Name, r.Test.SourceFile, icon, dur))
	}

	b.WriteString("\n")

	// Summary line
	var parts []string
	if passed > 0 {
		parts = append(parts, fmt.Sprintf("**%d passed**", passed))
	}
	if flaky > 0 {
		parts = append(parts, fmt.Sprintf("**%d flaky**", flaky))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("**%d failed**", failed))
	}
	if errored > 0 {
		parts = append(parts, fmt.Sprintf("**%d errored**", errored))
	}
	if cached > 0 {
		parts = append(parts, fmt.Sprintf("**%d cached**", cached))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("**%d skipped**", skipped))
	}
	if len(parts) > 0 {
		b.WriteString(strings.Join(parts, " · "))
		b.WriteString("\n")
	}

	// Failures details
	if len(failures) > 0 {
		b.WriteString("\n<details>\n<summary>Failures</summary>\n\n")
		for _, r := range failures {
			b.WriteString(fmt.Sprintf("### %s\n", r.Test.Name))
			if r.Reasoning != "" {
				b.WriteString(fmt.Sprintf("> %s\n", strings.ReplaceAll(strings.TrimSpace(r.Reasoning), "\n", "\n> ")))
			}
			b.WriteString("\n")
		}
		b.WriteString("</details>\n")
	}

	// Errors details
	if len(errors) > 0 {
		b.WriteString("\n<details>\n<summary>Errors</summary>\n\n")
		for _, r := range errors {
			b.WriteString(fmt.Sprintf("### %s\n", r.Test.Name))
			if r.Reasoning != "" {
				b.WriteString(fmt.Sprintf("> %s\n", strings.ReplaceAll(strings.TrimSpace(r.Reasoning), "\n", "\n> ")))
			}
			b.WriteString("\n")
		}
		b.WriteString("</details>\n")
	}

	// Flaky test details
	if len(flakyTests) > 0 {
		b.WriteString("\n<details>\n<summary>Flaky Tests</summary>\n\n")
		for _, r := range flakyTests {
			b.WriteString(fmt.Sprintf("### %s\n", r.Test.Name))
			b.WriteString(fmt.Sprintf("Passed on retry %d\n", r.Retries))
			if r.Reasoning != "" {
				b.WriteString(fmt.Sprintf("> %s\n", strings.ReplaceAll(strings.TrimSpace(r.Reasoning), "\n", "\n> ")))
			}
			b.WriteString("\n")
		}
		b.WriteString("</details>\n")
	}

	// Cost details
	if totalUsage.APICalls > 0 {
		cost := estimateCost(model, totalUsage.InputTokens, totalUsage.OutputTokens)
		b.WriteString("\n<details>\n<summary>Cost</summary>\n\n")
		b.WriteString(fmt.Sprintf("Model: %s · %d API calls · %s tokens · ~$%.4f\n\n",
			model, totalUsage.APICalls,
			formatTokens(totalUsage.InputTokens+totalUsage.OutputTokens),
			cost))
		b.WriteString("</details>\n")
	}

	return b.String()
}

// HasFailures returns true if any test result is a genuine test failure
// (not cached, skipped, or an infrastructure error).
func HasFailures(results []types.TestResult) bool {
	for _, r := range results {
		if !r.Passed && !r.Cached && !r.Skipped && !r.Errored {
			return true
		}
	}
	return false
}

// HasErrors returns true if any test result is an infrastructure error.
func HasErrors(results []types.TestResult) bool {
	for _, r := range results {
		if r.Errored {
			return true
		}
	}
	return false
}

// HasFlaky returns true if any test result was marked flaky (passed on retry).
func HasFlaky(results []types.TestResult) bool {
	for _, r := range results {
		if r.Flaky {
			return true
		}
	}
	return false
}
