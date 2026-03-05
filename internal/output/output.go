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
	green = color.New(color.FgGreen)
	red   = color.New(color.FgRed)
	gray  = color.New(color.FgHiBlack)
	bold  = color.New(color.Bold)
)

func Print(results []types.TestResult, model string, verbose bool) {
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
		gray.Printf("  .axiom/%s\n", file)
		for _, r := range groups[file] {
			switch {
			case r.Cached:
				gray.Printf("    ○ %s (cached)\n", r.Test.Name)
			case r.Skipped:
				gray.Printf("    ○ %s (skipped)\n", r.Test.Name)
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
				if r.Reasoning != "" {
					printReasoning(r.Reasoning, "      ")
				}
			}
		}
		fmt.Println()
	}

	// Summary
	passed, failed, cached, skipped := 0, 0, 0, 0
	var totalUsage types.Usage
	for _, r := range results {
		switch {
		case r.Cached:
			cached++
		case r.Skipped:
			skipped++
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
	if failed > 0 {
		if parts > 0 {
			fmt.Print(" · ")
		}
		red.Printf("%d failed", failed)
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
		Reasoning string    `json:"reasoning,omitempty"`
		Duration  float64   `json:"duration_seconds,omitempty"`
		Usage     jsonUsage `json:"usage,omitempty"`
	}

	var out []jsonResult
	for _, r := range results {
		out = append(out, jsonResult{
			Name:      r.Test.Name,
			File:      r.Test.SourceFile,
			Passed:    r.Passed,
			Cached:    r.Cached,
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
	lines := strings.Split(strings.TrimSpace(reasoning), "\n")
	for _, line := range lines {
		gray.Printf("%s%s\n", indent, line)
	}
}

// estimateCost returns estimated cost in USD based on model pricing.
// Prices are per million tokens.
func estimateCost(model string, inputTokens, outputTokens int) float64 {
	type pricing struct{ input, output float64 }

	// https://docs.anthropic.com/en/docs/about-claude/models
	prices := map[string]pricing{
		"claude-haiku-4-5":          {1.00, 5.00},
		"claude-haiku-4-5-20251001": {1.00, 5.00},
		"claude-sonnet-4-5":         {3.00, 15.00},
		"claude-sonnet-4-5-20250514": {3.00, 15.00},
		"claude-sonnet-4-6":         {3.00, 15.00},
		"claude-opus-4-6":           {15.00, 75.00},
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

func HasFailures(results []types.TestResult) bool {
	for _, r := range results {
		if !r.Passed && !r.Cached && !r.Skipped {
			return true
		}
	}
	return false
}
