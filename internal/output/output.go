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

func Print(results []types.TestResult, verbose bool) {
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
	fmt.Println()
}

func PrintJSON(results []types.TestResult) error {
	type jsonResult struct {
		Name      string  `json:"name"`
		File      string  `json:"file"`
		Passed    bool    `json:"passed"`
		Cached    bool    `json:"cached"`
		Reasoning string  `json:"reasoning,omitempty"`
		Duration  float64 `json:"duration_seconds,omitempty"`
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

func HasFailures(results []types.TestResult) bool {
	for _, r := range results {
		if !r.Passed && !r.Cached && !r.Skipped {
			return true
		}
	}
	return false
}
