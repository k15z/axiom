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
	"github.com/k15z/axiom/internal/notes"
	"github.com/k15z/axiom/internal/provider"
	"github.com/k15z/axiom/internal/types"
)

type Options struct {
	All         bool
	Filter      string
	Tag         string
	Bail        bool
	Verbose     bool
	Concurrency int // ≤1 means sequential
	Retries     int // re-run failed tests up to N times; 0 = no retries
}

func MatchesTag(t discovery.Test, tagFilter string) bool {
	if tagFilter == "" {
		return true
	}
	for _, w := range strings.Split(tagFilter, ",") {
		w = strings.TrimSpace(w)
		for _, tag := range t.Tags {
			if strings.EqualFold(tag, w) {
				return true
			}
		}
	}
	return false
}

// newProvider creates the appropriate LLM provider for the given config.
// Declared as a variable so tests can override it with a mock.
var newProvider = func(cfg config.Config) provider.Provider {
	return provider.FromConfig(provider.ProviderConfig{
		Provider: cfg.Provider,
		APIKey:   cfg.APIKey,
		BaseURL:  cfg.BaseURL,
	})
}

func Run(ctx context.Context, cfg config.Config, tests []discovery.Test, opts Options) ([]types.TestResult, error) {
	repoRoot, _ := filepath.Abs(".")

	var c *cache.Cache
	var cacheMu sync.Mutex
	if cfg.Cache.Enabled && !opts.All {
		var err error
		configHash := cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens, cfg.Provider, cfg.BaseURL)
		c, err = cache.Load(cfg.Cache.Dir, configHash)
		if err != nil {
			c = cache.New(cfg.Cache.Dir, configHash)
		}
	}

	// Load agent notes for context injection
	noteStore := notes.Load(cfg.Cache.Dir)
	var notesMu sync.Mutex

	// Create a single shared provider instance for all tests in this run.
	p := newProvider(cfg)

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = AutoConcurrency(len(tests))
	}

	// Bail support: cancel remaining tests on first failure
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Pre-compute total number of tests that will run (after filter)
	total := 0
	for _, t := range tests {
		if opts.Filter != "" {
			if matched, _ := filepath.Match(opts.Filter, t.Name); !matched {
				continue
			}
		}
		if !MatchesTag(t, opts.Tag) {
			continue
		}
		total++
	}

	// Live display — one spinner line per running test
	live := display.NewLiveDisplay(total)

	// Pre-allocate results slice to preserve original test order
	results := make([]types.TestResult, len(tests))
	included := make([]bool, len(tests))

	sem := make(chan struct{}, concurrency)
	throttle := newThrottle()
	var wg sync.WaitGroup

	for i, test := range tests {
		if opts.Filter != "" {
			if matched, _ := filepath.Match(opts.Filter, test.Name); !matched {
				continue
			}
		}
		if !MatchesTag(test, opts.Tag) {
			continue
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
				live.FinishTest(test.Name, passed, true, false, false, 0)
				continue
			}
		}
		cacheMu.Unlock()

		included[i] = true
		idx := i
		t := test

		exec := &testExecutor{
			cfg:       cfg,
			provider:  p,
			repoRoot:  repoRoot,
			noteStore: noteStore,
			notesMu:   &notesMu,
			throttle:  throttle,
			retries:   opts.Retries,
		}

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
				live.FinishTest(t.Name, false, false, true, false, 0)
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

			tr := exec.execute(ctx, t, progress)
			results[idx] = tr

			live.FinishTest(t.Name, tr.Passed, false, false, tr.Errored, tr.Duration)

			cacheMu.Lock()
			if c != nil && !tr.Errored {
				res := "fail"
				if tr.Passed {
					res = "pass"
				}
				c.Update(t.Name, res, cache.HashGlobs(t.On, repoRoot), tr.Reasoning)
			}
			cacheMu.Unlock()

			if opts.Bail && !tr.Passed {
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

	// Save agent notes
	notesMu.Lock()
	_ = noteStore.Save(cfg.Cache.Dir)
	notesMu.Unlock()

	// Collect results in original order
	var out []types.TestResult
	for i, r := range results {
		if included[i] {
			out = append(out, r)
		}
	}
	return out, nil
}

// testExecutor handles the execution of a single test, including agent
// invocation, retries, notes management, and result construction.
type testExecutor struct {
	cfg       config.Config
	provider  provider.Provider
	repoRoot  string
	noteStore *notes.Store
	notesMu   *sync.Mutex
	throttle  *Throttle
	retries   int
}

// execute runs a single test and returns its result.
func (e *testExecutor) execute(ctx context.Context, t discovery.Test, progress agent.ProgressFunc) types.TestResult {
	e.throttle.Wait()

	start := time.Now()

	// Per-test config overrides
	testModel := e.cfg.Model
	if t.Model != "" {
		testModel = t.Model
	}
	testTimeout := e.cfg.Agent.Timeout
	if t.Timeout > 0 {
		testTimeout = t.Timeout
	}
	testMaxIter := e.cfg.Agent.MaxIterations
	if t.MaxIterations > 0 {
		testMaxIter = t.MaxIterations
	}

	// Per-test timeout
	runCtx := ctx
	if testTimeout > 0 {
		var timeoutCancel context.CancelFunc
		runCtx, timeoutCancel = context.WithTimeout(ctx, time.Duration(testTimeout)*time.Second)
		defer timeoutCancel()
	}

	// Build prior notes context for the agent
	e.notesMu.Lock()
	priorNotes := buildPriorNotes(e.noteStore, t.Name, e.repoRoot)
	e.notesMu.Unlock()

	agentOpts := agent.RunOptions{
		MaxIterations: testMaxIter,
		MaxTokens:     e.cfg.Agent.MaxTokens,
		ToolTimeout:   time.Duration(e.cfg.Agent.ToolTimeout) * time.Second,
		PriorNotes:    priorNotes,
	}

	result, err := agent.Run(runCtx, e.provider, testModel, t.Condition, t.On, e.repoRoot, progress, agentOpts)
	duration := time.Since(start)

	tr := types.TestResult{Test: t, Duration: duration}
	tr.Usage = types.Usage{
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		APICalls:     result.Usage.APICalls,
	}
	if err != nil {
		tr.Errored = true
		tr.Reasoning = "Agent error: " + err.Error()
		if provider.IsRateLimitError(err) {
			e.throttle.Signal()
		}
	} else {
		tr.Passed = result.Passed
		tr.Reasoning = result.Reasoning
	}

	// Save agent notes for future runs
	if result.Notes != "" {
		e.notesMu.Lock()
		e.noteStore.UpdateTestNotes(t.Name, result.Notes, result.NoteFiles, e.repoRoot)
		e.notesMu.Unlock()
	}

	// Retry failed tests. If a retry passes, mark as flaky.
	// Skip retries for infrastructure errors (API failures, timeouts).
	if !tr.Passed && !tr.Errored && e.retries > 0 {
		for retry := 1; retry <= e.retries; retry++ {
			if ctx.Err() != nil {
				break
			}
			progress(agent.Event{Kind: "thinking", Message: fmt.Sprintf("retrying (%d/%d)…", retry, e.retries)})
			retryResult, retryErr := agent.Run(runCtx, e.provider, testModel, t.Condition, t.On, e.repoRoot, progress, agentOpts)
			tr.Duration = time.Since(start)
			tr.Usage.InputTokens += retryResult.Usage.InputTokens
			tr.Usage.OutputTokens += retryResult.Usage.OutputTokens
			tr.Usage.APICalls += retryResult.Usage.APICalls
			tr.Retries = retry
			if retryErr == nil && retryResult.Passed {
				tr.Passed = true
				tr.Flaky = true
				tr.Reasoning = retryResult.Reasoning
				break
			}
			if retryErr != nil {
				tr.Reasoning = "Agent error: " + retryErr.Error()
			} else {
				tr.Reasoning = retryResult.Reasoning
			}
		}
	}

	return tr
}

// newThrottle creates the rate-limit throttle for a run.
// Declared as a variable so tests can override it with a fast throttle.
var newThrottle = NewThrottle

// ClearCache deletes the test cache. Used by the cache clear command.
func ClearCache(cacheDir string) error {
	return cache.New(cacheDir, "").Clear()
}

// buildPriorNotes assembles context from cached notes for a specific test.
// Combines codebase-level and test-specific notes with staleness caveats.
func buildPriorNotes(store *notes.Store, testName string, repoRoot string) string {
	var parts []string

	cbNotes, cbStale := store.GetCodebaseNotes(repoRoot)
	if cbNotes != "" {
		header := "Codebase overview"
		if cbStale {
			header += " (some referenced files changed — verify)"
		}
		parts = append(parts, header+":\n"+cbNotes)
	}

	testNotes, testStale := store.GetTestNotes(testName, repoRoot)
	if testNotes != "" {
		header := "Previous investigation of this test"
		if testStale {
			header += " (some referenced files changed — verify)"
		}
		parts = append(parts, header+":\n"+testNotes)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
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
