package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/k15z/axiom/internal/cache"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
	var (
		dir      string
		jsonFlag bool
		diffFlag bool
	)

	cmd := &cobra.Command{
		Use:   "show [test-name]",
		Short: "Show cached reasoning from the last run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.LoadOpts{TestDir: dir})
			if err != nil {
				return &SetupError{Err: fmt.Errorf("loading config: %w", err)}
			}

			tests, err := discovery.Discover(cfg.TestDir)
			if err != nil {
				return &SetupError{Err: fmt.Errorf("discovery: %w", err)}
			}

			configHash := cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens, cfg.Provider, cfg.BaseURL)
			c, err := cache.Load(cfg.Cache.Dir, configHash)
			if err != nil {
				return &SetupError{Err: fmt.Errorf("loading cache: %w", err)}
			}

			repoRoot, _ := filepath.Abs(".")

			// Filter to a single test if name provided
			var filterName string
			if len(args) > 0 {
				filterName = args[0]
			}

			type showEntry struct {
				Name          string `json:"name"`
				File          string `json:"file"`
				Result        string `json:"result"`
				Stale         bool   `json:"stale"`
				Reasoning     string `json:"reasoning"`
				PrevReasoning string `json:"prev_reasoning,omitempty"`
			}

			var entries []showEntry
			for _, t := range tests {
				if filterName != "" && t.Name != filterName {
					continue
				}
				entry, ok := c.GetEntry(t.Name)
				if !ok || entry.Reasoning == "" {
					continue
				}
				stale := true
				if skip, _ := c.ShouldSkip(t.Name, t.On, repoRoot); skip {
					stale = false
				}
				entries = append(entries, showEntry{
					Name:          t.Name,
					File:          t.SourceFile,
					Result:        entry.Result,
					Stale:         stale,
					Reasoning:     entry.Reasoning,
					PrevReasoning: entry.PrevReasoning,
				})
			}

			if filterName != "" && len(entries) == 0 {
				return fmt.Errorf("no cached reasoning for %q — run `axiom run %s` first", filterName, filterName)
			}

			if len(entries) == 0 {
				fmt.Println("No cached reasoning found. Run tests first with: axiom run")
				return nil
			}

			if jsonFlag {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			green := color.New(color.FgGreen)
			red := color.New(color.FgRed)
			gray := color.New(color.FgHiBlack)
			bold := color.New(color.Bold)

			fmt.Println()
			bold.Println("  axiom show")
			fmt.Println()

			currentFile := ""
			for _, e := range entries {
				if e.File != currentFile {
					currentFile = e.File
					gray.Printf("  %s%s\n", cfg.TestDir, currentFile)
				}

				var marker string
				var c *color.Color
				if e.Result == "pass" {
					marker, c = "✓", green
				} else {
					marker, c = "✗", red
				}

				c.Printf("    %s ", marker)
				fmt.Print(e.Name)
				if e.Stale {
					gray.Print(" (stale)")
				}
				fmt.Println()

				if diffFlag {
					if e.PrevReasoning == "" {
						gray.Println("      (no previous reasoning to compare)")
					} else {
						printDiff(e.PrevReasoning, e.Reasoning, green, red, gray)
					}
				} else {
					for _, line := range strings.Split(strings.TrimSpace(e.Reasoning), "\n") {
						gray.Printf("      %s\n", line)
					}
				}
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&diffFlag, "diff", false, "Show diff against previous reasoning")
	return cmd
}

// printDiff displays a simple line-by-line diff between old and new reasoning.
func printDiff(old, new string, green, red, gray *color.Color) {
	oldLines := strings.Split(strings.TrimSpace(old), "\n")
	newLines := strings.Split(strings.TrimSpace(new), "\n")

	// Simple LCS-based diff
	lcs := lcsTable(oldLines, newLines)
	i, j := len(oldLines), len(newLines)
	type diffLine struct {
		op   byte // ' ', '+', '-'
		text string
	}
	var result []diffLine
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			result = append(result, diffLine{' ', oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			result = append(result, diffLine{'+', newLines[j-1]})
			j--
		} else {
			result = append(result, diffLine{'-', oldLines[i-1]})
			i--
		}
	}
	// Reverse to get correct order
	for l, r := 0, len(result)-1; l < r; l, r = l+1, r-1 {
		result[l], result[r] = result[r], result[l]
	}
	for _, d := range result {
		switch d.op {
		case '+':
			green.Printf("      + %s\n", d.text)
		case '-':
			red.Printf("      - %s\n", d.text)
		default:
			gray.Printf("        %s\n", d.text)
		}
	}
}

func lcsTable(a, b []string) [][]int {
	m, n := len(a), len(b)
	t := make([][]int, m+1)
	for i := range t {
		t[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				t[i][j] = t[i-1][j-1] + 1
			} else if t[i-1][j] >= t[i][j-1] {
				t[i][j] = t[i-1][j]
			} else {
				t[i][j] = t[i][j-1]
			}
		}
	}
	return t
}
