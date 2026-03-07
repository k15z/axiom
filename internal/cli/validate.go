package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Lint test YAML files for structural issues",
		Long:  "Checks glob syntax, warns on missing 'on' patterns (tests that can never cache), and flags short/vague conditions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.LoadMinimal(dir)

			tests, err := discovery.Discover(cfg.TestDir)
			if err != nil {
				return &SetupError{Err: fmt.Errorf("discovery: %w", err)}
			}
			if len(tests) == 0 {
				fmt.Println("No tests found.")
				return nil
			}

			type issue struct {
				kind    string // "error" or "warning"
				message string
			}
			testIssues := make(map[string][]issue)

			for _, t := range tests {
				var issues []issue

				// Check 1: glob syntax validity
				for _, pattern := range t.On {
					if err := validateGlobSyntax(pattern); err != nil {
						issues = append(issues, issue{"error", fmt.Sprintf("invalid glob %q: %v", pattern, err)})
					}
				}

				// Check 2: missing on patterns (test can never be cached)
				if len(t.On) == 0 {
					issues = append(issues, issue{"warning", "no 'on' globs — test always runs and can never be cached"})
				}

				// Check 3: empty/vague condition
				trimmed := strings.TrimSpace(t.Condition)
				if len(trimmed) < 20 {
					issues = append(issues, issue{"warning", fmt.Sprintf("condition is very short (%d chars) — may be too vague", len(trimmed))})
				}

				testIssues[t.Name] = issues
			}

			yellow := color.New(color.FgYellow)
			red := color.New(color.FgRed)
			green := color.New(color.FgGreen)

			totalErrors, totalWarnings := 0, 0
			currentFile := ""

			for _, t := range tests {
				if t.SourceFile != currentFile {
					currentFile = t.SourceFile
					fmt.Printf("\n  %s%s\n", cfg.TestDir, currentFile)
				}
				issues := testIssues[t.Name]
				if len(issues) == 0 {
					fmt.Printf("    %s ", t.Name)
					green.Println("[ok]")
					continue
				}
				fmt.Printf("    %s\n", t.Name)
				for _, iss := range issues {
					if iss.kind == "error" {
						fmt.Printf("      ")
						red.Printf("[error] ")
						fmt.Println(iss.message)
						totalErrors++
					} else {
						fmt.Printf("      ")
						yellow.Printf("[warn]  ")
						fmt.Println(iss.message)
						totalWarnings++
					}
				}
			}
			fmt.Println()

			if totalErrors > 0 {
				red.Printf("  %d error(s), %d warning(s)\n\n", totalErrors, totalWarnings)
				return fmt.Errorf("validation failed with %d error(s)", totalErrors)
			}
			if totalWarnings > 0 {
				yellow.Printf("  0 errors, %d warning(s)\n\n", totalWarnings)
			} else {
				green.Printf("  All %d test(s) valid\n\n", len(tests))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory")
	return cmd
}

// validateGlobSyntax checks each non-** segment of a glob pattern using
// filepath.Match, which returns ErrBadPattern for syntax errors like unclosed
// brackets.
func validateGlobSyntax(pattern string) error {
	for _, seg := range strings.Split(filepath.ToSlash(pattern), "/") {
		if seg == "**" {
			continue
		}
		if _, err := filepath.Match(seg, ""); err != nil {
			return err
		}
	}
	return nil
}
