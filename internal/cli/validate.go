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
			cfg, err := config.Load(config.LoadOpts{TestDir: dir})
			if err != nil {
				return &SetupError{Err: err}
			}

			tests, err := discovery.Discover(cfg.TestDir)
			if err != nil {
				return &SetupError{Err: fmt.Errorf("failed to load test files: %w", err)}
			}
			if len(tests) == 0 {
				fmt.Println("No tests found. Run `axiom add` to create your first test.")
				return nil
			}

			type issue struct {
				kind    string // "error" or "warning"
				message string
			}
			testIssues := make(map[string][]issue)

			for _, t := range tests {
				var issues []issue

				// Check 1: glob syntax validity and dangerous patterns
				for _, pattern := range t.On {
					if err := validateGlobSyntax(pattern); err != nil {
						issues = append(issues, issue{"error", fmt.Sprintf("invalid glob %q: %v — fix the pattern syntax (e.g. 'src/**/*.go')", pattern, err)})
					}
					normalized := filepath.ToSlash(pattern)
					if strings.HasPrefix(normalized, "/") {
						issues = append(issues, issue{"warning", fmt.Sprintf("glob pattern %q uses absolute path and will never match project files", pattern)})
					} else if strings.Contains(normalized, "../") {
						issues = append(issues, issue{"warning", fmt.Sprintf("glob pattern %q contains '../' and may reference files outside the project", pattern)})
					}
				}

				// Check 2: missing on patterns (test can never be cached)
				if len(t.On) == 0 {
					issues = append(issues, issue{"warning", "no 'on' globs — test always runs and can never be cached. Add `on: [\"src/**/*.go\"]` to scope it"})
				}

				// Check 3: empty/vague condition
				trimmed := strings.TrimSpace(t.Condition)
				if len(trimmed) < 20 {
					issues = append(issues, issue{"warning", fmt.Sprintf("condition is very short (%d chars) — be specific about what to verify, e.g. 'all exported functions in pkg/auth have doc comments'", len(trimmed))})
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
					fmt.Printf("\n  %s\n", filepath.Join(cfg.TestDir, currentFile))
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
	for seg := range strings.SplitSeq(filepath.ToSlash(pattern), "/") {
		if seg == "**" {
			continue
		}
		if _, err := filepath.Match(seg, ""); err != nil {
			return err
		}
	}
	return nil
}

// preflightValidate checks discovered tests for errors that would cause
// runtime failures: invalid glob syntax in 'on:' patterns. It returns a
// non-nil error listing all problems found so the user can fix them before
// any LLM API calls are made.
//
// Note: missing conditions and duplicate test names are already caught by
// discovery.Discover(), so they do not need to be rechecked here.
func preflightValidate(tests []discovery.Test) error {
	var errs []string
	for _, t := range tests {
		for _, pattern := range t.On {
			if err := validateGlobSyntax(pattern); err != nil {
				errs = append(errs, fmt.Sprintf("  test %q (in %s): invalid glob %q: %v", t.Name, t.SourceFile, pattern, err))
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("pre-flight validation failed:\n%s", strings.Join(errs, "\n"))
}
