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
		strict      bool
		quiet       bool
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

			if concurrency < 0 {
				return fmt.Errorf("--concurrency must be non-negative, got %d", concurrency)
			}
			if retries < 0 {
				return fmt.Errorf("--retries must be non-negative, got %d", retries)
			}

			if jsonOut && cmd.Flags().Changed("format") {
				return fmt.Errorf("cannot use --json with --format; they are mutually exclusive")
			}

			cfg, err := config.Load(config.LoadOpts{TestDir: dir})
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
				return &SetupError{Err: fmt.Errorf("failed to load test files: %w\nRun `axiom validate` to check your test YAML for issues.", err)}
			}

			if err := preflightValidate(tests); err != nil {
				return &SetupError{Err: err}
			}

			if len(tests) == 0 {
				fmt.Println("No tests found. Run `axiom add` to create your first test, or `axiom init` to generate a starter suite.")
				return nil
			}

			if dryRun {
				repoRoot, _ := filepath.Abs(".")
				statuses := runner.GetStatuses(tests, cfg.Cache.Dir, repoRoot, cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens, cfg.Provider, cfg.BaseURL))
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
				if len(statuses) == 0 {
					if filter != "" {
						fmt.Fprintf(os.Stderr, "No tests matched filter %q.\n", filter)
					} else if tag != "" {
						fmt.Fprintf(os.Stderr, "No tests matched tag %q.\n", tag)
					}
					return nil
				}
				if all {
					for i := range statuses {
						statuses[i].Status = "pending"
					}
				}
				if !quiet {
					output.PrintDryRun(statuses, cfg.Model, cfg.Agent.MaxTokens, cfg.TestDir)
				}
				return nil
			}

			// Check if any tests match the filter/tag before running
			if filter != "" || tag != "" {
				matched := false
				for _, t := range tests {
					if filter != "" {
						if m, _ := filepath.Match(filter, t.Name); !m {
							continue
						}
					}
					if !runner.MatchesTag(t, tag) {
						continue
					}
					matched = true
					break
				}
				if !matched {
					if filter != "" {
						fmt.Fprintf(os.Stderr, "No tests matched filter %q.\n", filter)
					} else {
						fmt.Fprintf(os.Stderr, "No tests matched tag %q.\n", tag)
					}
					return nil
				}
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
				return fmt.Errorf("running tests: %w", err)
			}

			// --json is shorthand for --format json
			outputFormat := format
			if jsonOut && outputFormat == "text" {
				outputFormat = "json"
			}

			if !quiet {
				switch outputFormat {
				case "json":
					if err := output.PrintJSON(results, cfg.Model); err != nil {
						return fmt.Errorf("writing JSON output: %w", err)
					}
				case "github":
					output.PrintGitHub(results, cfg.Model)
				default:
					output.Print(results, cfg.Model, verbose, cfg.TestDir)
				}
			} else {
				passed, failed, cached, skipped, flaky, errored := 0, 0, 0, 0, 0, 0
				for _, r := range results {
					switch {
					case r.Cached:
						cached++
					case r.Skipped:
						skipped++
					case r.Errored:
						errored++
					case r.Flaky:
						flaky++
					case r.Passed:
						passed++
					default:
						failed++
					}
				}
				fmt.Fprintf(os.Stderr, "axiom: %s\n", output.CISummary(passed, failed, errored, cached, skipped, flaky))
			}

			if output.HasFailures(results) {
				return &RunFailureError{ExitCode: 1, Msg: "some tests failed"}
			}
			if strict && output.HasFlaky(results) {
				return &RunFailureError{ExitCode: 1, Msg: "some tests were flaky (strict mode)"}
			}
			if output.HasErrors(results) {
				return &RunFailureError{ExitCode: 2, Msg: "some tests errored"}
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
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat flaky tests (passed on retry) as failures")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress full output, only emit CI summary line to stderr")

	return cmd
}
