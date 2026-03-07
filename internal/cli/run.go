package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/k15z/axiom/internal/cache"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/output"
	"github.com/k15z/axiom/internal/runner"
	"github.com/k15z/axiom/internal/types"
	"github.com/k15z/axiom/internal/watch"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var (
		all         bool
		filter      string
		tag         string
		dir         string
		verbose     bool
		model       string
		prov        string
		bail        bool
		jsonOut     bool
		format      string
		concurrency int
		retries     int
		dryRun      bool
		watchMode   bool
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

			cfg, err := config.LoadWithoutKey(dir)
			if err != nil {
				return &SetupError{Err: err}
			}

			if model != "" {
				cfg.Model = model
			}
			if prov != "" {
				cfg.Provider = prov
			}

			if !dryRun {
				if err := cfg.ResolveKey(); err != nil {
					return &SetupError{Err: err}
				}
			}

			// Watch mode: run tests then watch for changes
			if watchMode {
				if !isatty.IsTerminal(os.Stderr.Fd()) {
					fmt.Fprintln(os.Stderr, "warning: --watch requires a TTY; falling back to single run")
				} else {
					ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
					defer stop()
					return watch.Run(ctx, cfg, runner.Options{
						All:         all,
						Filter:      filter,
						Tag:         tag,
						Bail:        bail,
						Verbose:     verbose,
						Concurrency: concurrency,
						Retries:     retries,
					})
				}
			}

			tests, err := discovery.Discover(cfg.TestDir)
			if err != nil {
				return &SetupError{Err: fmt.Errorf("discovery: %w", err)}
			}

			if len(tests) == 0 {
				fmt.Println("No tests found. Run 'axiom init' to create sample tests.")
				return nil
			}

			if dryRun {
				repoRoot, _ := filepath.Abs(".")
				statuses := runner.GetStatuses(tests, cfg.Cache.Dir, repoRoot, cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens))
				if filter != "" || tag != "" {
					var filtered []types.TestStatus
					for _, s := range statuses {
						if filter != "" {
							if matched, _ := filepath.Match(filter, s.Test.Name); !matched {
								continue
							}
						}
						if !runner.MatchesTag(s.Test, tag) {
							continue
						}
						filtered = append(filtered, s)
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
				Tag:         tag,
				Bail:        bail,
				Verbose:     verbose,
				Concurrency: concurrency,
				Retries:     retries,
			})
			if err != nil {
				return err
			}

			// --json is shorthand for --format json
			outputFormat := format
			if jsonOut && outputFormat == "text" {
				outputFormat = "json"
			}

			switch outputFormat {
			case "json":
				if err := output.PrintJSON(results, cfg.Model); err != nil {
					return err
				}
			case "github":
				output.PrintGitHub(results, cfg.Model)
			default:
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
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Run tests matching any of the given tags (comma-separated)")
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory (default: .axiom/)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show full agent reasoning (default: one-line summary)")
	cmd.Flags().StringVarP(&model, "model", "m", "", "LLM model to use")
	cmd.Flags().StringVarP(&prov, "provider", "p", "", "LLM provider: anthropic, openai, or gemini")
	cmd.Flags().BoolVarP(&bail, "bail", "b", false, "Stop on first failure")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output results as JSON")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text, json, or github")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "c", 0, "Number of tests to run in parallel (0 = auto)")
	cmd.Flags().IntVar(&retries, "retries", 0, "Re-run failed tests up to N times; if a retry passes, mark as flaky")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview which tests would run vs be skipped and estimate token cost, without calling the API")
	cmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "Watch for file changes and re-run affected tests")

	return cmd
}
