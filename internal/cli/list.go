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
	"github.com/k15z/axiom/internal/runner"
	"github.com/k15z/axiom/internal/types"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		dir      string
		jsonFlag bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tests and their cached status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.LoadOpts{TestDir: dir})
			if err != nil {
				return &SetupError{Err: fmt.Errorf("loading config: %w", err)}
			}

			tests, err := discovery.Discover(cfg.TestDir)
			if err != nil {
				return &SetupError{Err: fmt.Errorf("failed to load test files: %w", err)}
			}
			if len(tests) == 0 {
				fmt.Println("No tests found. Run `axiom add` to create your first test, or `axiom init` to generate a starter suite.")
				return nil
			}

			repoRoot, _ := filepath.Abs(".")
			configHash := cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens, cfg.Provider, cfg.BaseURL)
			statuses := runner.GetStatuses(tests, cfg.Cache.Dir, repoRoot, configHash)

			if jsonFlag {
				return printListJSON(statuses)
			}

			gray := color.New(color.FgHiBlack)
			green := color.New(color.FgGreen)
			red := color.New(color.FgRed)

			currentFile := ""
			for _, s := range statuses {
				if s.Test.SourceFile != currentFile {
					currentFile = s.Test.SourceFile
					fmt.Printf("\n  %s\n", filepath.Join(cfg.TestDir, currentFile))
				}

				var label string
				var c *color.Color
				switch s.Status {
				case "cached-pass":
					label, c = "cached (pass)", gray
				case "cached-fail":
					label, c = "cached (fail)", gray
				case "stale-pass":
					label, c = "stale (pass)", green
				case "stale-fail":
					label, c = "stale (fail)", red
				default:
					label, c = "pending", gray
				}

				fmt.Printf("    %s ", s.Test.Name)
				if len(s.Test.Tags) > 0 {
					gray.Printf("[%s] ", strings.Join(s.Test.Tags, ", "))
				}
				c.Printf("[%s]\n", label)
			}

			fmt.Println()
			gray.Printf("  %d test(s)\n\n", len(statuses))
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
	return cmd
}

func printListJSON(statuses []types.TestStatus) error {
	type jsonEntry struct {
		Name   string   `json:"name"`
		File   string   `json:"file"`
		Status string   `json:"status"`
		Tags   []string `json:"tags,omitempty"`
		Globs  []string `json:"on,omitempty"`
	}

	var out []jsonEntry
	for _, s := range statuses {
		out = append(out, jsonEntry{
			Name:   s.Test.Name,
			File:   s.Test.SourceFile,
			Status: s.Status,
			Tags:   s.Test.Tags,
			Globs:  s.Test.On,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
