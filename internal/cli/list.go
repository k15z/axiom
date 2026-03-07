package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/k15z/axiom/internal/cache"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/runner"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tests and their cached status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithoutKey(dir)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			tests, err := discovery.Discover(cfg.TestDir)
			if err != nil {
				return fmt.Errorf("discovery: %w", err)
			}
			if len(tests) == 0 {
				fmt.Println("No tests found.")
				return nil
			}

			repoRoot, _ := filepath.Abs(".")
			configHash := cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens)
			statuses := runner.GetStatuses(tests, cfg.Cache.Dir, repoRoot, configHash)

			gray := color.New(color.FgHiBlack)
			green := color.New(color.FgGreen)
			red := color.New(color.FgRed)

			currentFile := ""
			for _, s := range statuses {
				if s.Test.SourceFile != currentFile {
					currentFile = s.Test.SourceFile
					fmt.Printf("\n  %s%s\n", cfg.TestDir, currentFile)
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
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory")
	return cmd
}
