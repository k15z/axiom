package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/k15z/axiom/internal/agent"
	"github.com/k15z/axiom/internal/cache"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/display"
	"github.com/k15z/axiom/internal/types"
)

type Options struct {
	All         bool
	Filter      string
	Bail        bool
	Verbose     bool
	Concurrency int // ≤1 means sequential
}

func Run(ctx context.Context, cfg config.Config, tests []discovery.Test, opts Options) ([]types.TestResult, error) {
	repoRoot, _ := filepath.Abs(".")

	var c *cache.Cache
	var cacheMu sync.Mutex
	if cfg.Cache.Enabled && !opts.All {
		var err error
		configHash := cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens)
		c, err = cache.Load(cfg.Cache.Dir, configHash)
		if err != nil {
			c = cache.New(cfg.Cache.Dir, configHash)
		}
	}

	concurrency := opts.Concurrency
	if concurrency <= 1 {
		concurrency = 1
	}

	// Bail support: cancel remaining tests on first failure
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Pre-compute total number of tests that will run (after filter)
	total := 0
	for _, t := range tests {
		if opts.Filter == "" {
			total++
		} else if matched, _ := filepath.Match(opts.Filter, t.Name); matched {
			total++
		}
	}

	// Live display — one spinner line per running test
	live := display.NewLiveDisplay(total)

	// Pre-allocate results slice to preserve original test order
	results := make([]types.TestResult, len(tests))
	included := make([]bool, len(tests))

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, test := range tests {
		if opts.Filter != "" {
			if matched, _ := filepath.Match(opts.Filter, test.Name); !matched {
				continue
			}
		}

		// Cache check
		cacheMu.Lock()
		if c != nil && len(test.On) > 0 {
			if skip, _ := c.ShouldSkip(test.Name, test.On, repoRoot); skip {
				entry, _ := c.GetEntry(test.Name)
				passed := entry.Result == "pass"
				results[i] = types.TestResult{Test: test, Passed: passed, Cached: true}
				included[i] = true
				cacheMu.Unlock()
				live.StartTest(test.Name)
				live.FinishTest(test.Name, passed, true, false, 0)
				continue
			}
		}
		cacheMu.Unlock()

		included[i] = true
		idx := i
		t := test

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			// Check if bailed before starting
			select {
			case <-ctx.Done():
				results[idx] = types.TestResult{Test: t, Skipped: true}
				live.StartTest(t.Name)
				live.FinishTest(t.Name, false, false, true, 0)
				return
			default:
			}

			live.StartTest(t.Name)

			var textBuf strings.Builder
			progress := func(e agent.Event) {
				var status string
				switch e.Kind {
				case "thinking":
					textBuf.Reset()
					status = e.Message
				case "tool_call":
					textBuf.Reset()
					status = "→ " + e.Message
				case "text":
					textBuf.WriteString(e.Message)
					// Show last line of accumulated text as status
					s := textBuf.String()
					s = strings.ReplaceAll(s, "\n", " ")
					s = strings.TrimSpace(s)
					if s == "" {
						return
					}
					status = "✎ " + s
				default:
					return
				}
				live.Update(t.Name, status)
			}

			start := time.Now()

			// Per-test timeout
			runCtx := ctx
			if cfg.Agent.Timeout > 0 {
				var timeoutCancel context.CancelFunc
				runCtx, timeoutCancel = context.WithTimeout(ctx, time.Duration(cfg.Agent.Timeout)*time.Second)
				defer timeoutCancel()
			}

			result, err := agent.Run(runCtx, cfg.APIKey, cfg.Model, t.Condition, t.On, repoRoot, progress, agent.RunOptions{
				MaxIterations: cfg.Agent.MaxIterations,
				MaxTokens:     cfg.Agent.MaxTokens,
				ToolTimeout:   time.Duration(cfg.Agent.ToolTimeout) * time.Second,
			})
			duration := time.Since(start)

			tr := types.TestResult{Test: t, Duration: duration}
			tr.Usage = types.Usage{
				InputTokens:  result.Usage.InputTokens,
				OutputTokens: result.Usage.OutputTokens,
				APICalls:     result.Usage.APICalls,
			}
			if err != nil {
				tr.Passed = false
				tr.Reasoning = "Agent error: " + err.Error()
			} else {
				tr.Passed = result.Passed
				tr.Reasoning = result.Reasoning
			}
			results[idx] = tr

			live.FinishTest(t.Name, tr.Passed, false, false, duration)

			cacheMu.Lock()
			if c != nil {
				res := "fail"
				if err == nil && result.Passed {
					res = "pass"
				}
				c.Update(t.Name, res, cache.HashGlobs(t.On, repoRoot), tr.Reasoning)
			}
			cacheMu.Unlock()

			if opts.Bail && err == nil && !result.Passed {
				cancel()
			}
		}()
	}

	wg.Wait()
	live.Close()

	// Blank line between live display and final summary
	fmt.Fprintln(os.Stderr)

	if c != nil {
		cacheMu.Lock()
		_ = c.Save()
		cacheMu.Unlock()
	}

	// Collect results in original order
	var out []types.TestResult
	for i, r := range results {
		if included[i] {
			out = append(out, r)
		}
	}
	return out, nil
}

// ClearCache deletes the test cache. Used by the cache clear command.
func ClearCache(cacheDir string) error {
	return cache.New(cacheDir, "").Clear()
}

// GetStatuses returns the cached status for each test without running any agents.
func GetStatuses(tests []discovery.Test, cacheDir string, repoRoot string, configHash string) []types.TestStatus {
	c, _ := cache.Load(cacheDir, configHash)
	statuses := make([]types.TestStatus, len(tests))
	for i, t := range tests {
		s := types.TestStatus{Test: t, Status: "pending"}
		if c != nil {
			if entry, ok := c.GetEntry(t.Name); ok {
				skip, _ := c.ShouldSkip(t.Name, t.On, repoRoot)
				if skip {
					s.Status = "cached-" + entry.Result
				} else {
					s.Status = "stale-" + entry.Result
				}
			}
		}
		statuses[i] = s
	}
	return statuses
}
