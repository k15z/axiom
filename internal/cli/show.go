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
	)

	cmd := &cobra.Command{
		Use:   "show [test-name]",
		Short: "Show cached reasoning from the last run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithoutKey(dir)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			tests, err := discovery.Discover(cfg.TestDir)
			if err != nil {
				return fmt.Errorf("discovery: %w", err)
			}

			configHash := cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens)
			c, err := cache.Load(cfg.Cache.Dir, configHash)
			if err != nil {
				return fmt.Errorf("loading cache: %w", err)
			}

			repoRoot, _ := filepath.Abs(".")

			// Filter to a single test if name provided
			var filterName string
			if len(args) > 0 {
				filterName = args[0]
			}

			type showEntry struct {
				Name      string `json:"name"`
				File      string `json:"file"`
				Result    string `json:"result"`
				Stale     bool   `json:"stale"`
				Reasoning string `json:"reasoning"`
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
					Name:      t.Name,
					File:      t.SourceFile,
					Result:    entry.Result,
					Stale:     stale,
					Reasoning: entry.Reasoning,
				})
			}

			if filterName != "" && len(entries) == 0 {
				return fmt.Errorf("no cached reasoning for %q", filterName)
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

				for _, line := range strings.Split(strings.TrimSpace(e.Reasoning), "\n") {
					gray.Printf("      %s\n", line)
				}
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
	return cmd
}
