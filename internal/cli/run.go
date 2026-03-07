package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k15z/axiom/internal/cache"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/output"
	"github.com/k15z/axiom/internal/runner"
	"github.com/k15z/axiom/internal/types"
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
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "run [test-name]",
		Short: "Run behavioral tests",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Positional arg overrides --filter
			if len(args) == 1 {
				filter = args[0]
			}

			var cfg config.Config
			var err error
			if dryRun {
				cfg, err = config.LoadForDryRun(dir)
			} else {
				cfg, err = config.Load(dir)
			}
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

			if dryRun {
				repoRoot, _ := filepath.Abs(".")
				statuses := runner.GetStatuses(tests, cfg.Cache.Dir, repoRoot, cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens))
				if filter != "" {
					var filtered []types.TestStatus
					for _, s := range statuses {
						if matched, _ := filepath.Match(filter, s.Test.Name); matched {
							filtered = append(filtered, s)
						}
					}
					statuses = filtered
				}
				if all {
					for i := range statuses {
						statuses[i].Status = "pending"
					}
				}
				output.PrintDryRun(statuses, cfg.Model, cfg.Agent.MaxTokens)
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview which tests would run vs be skipped and estimate token cost, without calling the API")

	return cmd
}
