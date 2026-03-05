package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/output"
	"github.com/k15z/axiom/internal/runner"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var (
		all         bool
		filter      string
		dir         string
		verbose     bool
		model       string
		bail        bool
		jsonOut     bool
		concurrency int
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run behavioral tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(dir)
			if err != nil {
				return err
			}

			if model != "" {
				cfg.Model = model
			}

			tests, err := discovery.Discover(cfg.TestDir)
			if err != nil {
				return fmt.Errorf("discovery: %w", err)
			}

			if len(tests) == 0 {
				fmt.Println("No tests found. Run 'axiom init' to create sample tests.")
				return nil
			}

			results, err := runner.Run(context.Background(), cfg, tests, runner.Options{
				All:         all,
				Filter:      filter,
				Bail:        bail,
				Verbose:     verbose,
				Concurrency: concurrency,
			})
			if err != nil {
				return err
			}

			if jsonOut {
				if err := output.PrintJSON(results, cfg.Model); err != nil {
					return err
				}
			} else {
				output.Print(results, cfg.Model, verbose)
			}

			if output.HasFailures(results) {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Run all tests, ignoring cache")
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "Run tests matching a glob pattern")
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory (default: .axiom/)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show full agent reasoning (default: one-line summary)")
	cmd.Flags().StringVarP(&model, "model", "m", "", "LLM model to use")
	cmd.Flags().BoolVarP(&bail, "bail", "b", false, "Stop on first failure")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output results as JSON")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "c", 1, "Number of tests to run in parallel")

	return cmd
}
