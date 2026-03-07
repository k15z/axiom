// Package watch implements a polling-based file watcher that detects changes
// to files matching test on-globs and triggers re-runs of affected tests.
package watch

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/glob"
	"github.com/k15z/axiom/internal/output"
	"github.com/k15z/axiom/internal/runner"
)

const pollInterval = 500 * time.Millisecond

// Run enters watch mode: it runs affected tests, then polls for file changes
// and re-runs tests whose on-globs match changed files. It blocks until ctx
// is cancelled (e.g. Ctrl+C).
func Run(ctx context.Context, cfg config.Config, tests []discovery.Test, opts runner.Options) error {
	repoRoot, _ := filepath.Abs(".")

	// Initial snapshot of file modification times
	mtimes := snapshot(repoRoot)

	gray := color.New(color.FgHiBlack)
	cyan := color.New(color.FgCyan)

	// Initial run: run all matching tests
	fmt.Fprintf(os.Stderr, "\n")
	cyan.Fprintf(os.Stderr, "  ▶ Running all tests…\n")
	fmt.Fprintf(os.Stderr, "\n")

	results, err := runner.Run(ctx, cfg, tests, opts)
	if err != nil {
		return err
	}
	output.Print(results, cfg.Model, opts.Verbose)

	gray.Fprintf(os.Stderr, "\n  Watching for file changes… (Ctrl+C to exit)\n\n")

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "\n")
			gray.Fprintf(os.Stderr, "  Watch mode stopped.\n\n")
			return nil
		case <-ticker.C:
			current := snapshot(repoRoot)
			changed := diff(mtimes, current)
			if len(changed) == 0 {
				continue
			}
			mtimes = current

			// Find tests affected by changed files
			affected := matchTests(tests, changed, opts.Filter)
			if len(affected) == 0 {
				continue
			}

			// Show what triggered the re-run
			fmt.Fprintf(os.Stderr, "\n")
			cyan.Fprintf(os.Stderr, "  ▶ %d file(s) changed", len(changed))
			if len(changed) <= 3 {
				names := make([]string, 0, len(changed))
				for _, f := range changed {
					names = append(names, f)
				}
				cyan.Fprintf(os.Stderr, ": %s", strings.Join(names, ", "))
			}
			fmt.Fprintf(os.Stderr, "\n")
			gray.Fprintf(os.Stderr, "  Re-running %d affected test(s)…\n\n", len(affected))

			// Re-run only affected tests, ignoring cache
			rerunOpts := opts
			rerunOpts.All = true
			results, err = runner.Run(ctx, cfg, affected, rerunOpts)
			if err != nil {
				if ctx.Err() != nil {
					fmt.Fprintf(os.Stderr, "\n")
					gray.Fprintf(os.Stderr, "  Watch mode stopped.\n\n")
					return nil
				}
				return err
			}
			output.Print(results, cfg.Model, opts.Verbose)

			gray.Fprintf(os.Stderr, "\n  Watching for file changes… (Ctrl+C to exit)\n\n")
		}
	}
}

// snapshot walks the repo root and records file modification times.
// Skips hidden directories.
func snapshot(root string) map[string]time.Time {
	mtimes := make(map[string]time.Time)
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		mtimes[rel] = info.ModTime()
		return nil
	})
	return mtimes
}

// diff returns relative paths of files that were added, removed, or modified.
func diff(old, current map[string]time.Time) []string {
	var changed []string
	for path, newTime := range current {
		if oldTime, ok := old[path]; !ok || !newTime.Equal(oldTime) {
			changed = append(changed, path)
		}
	}
	for path := range old {
		if _, ok := current[path]; !ok {
			changed = append(changed, path)
		}
	}
	return changed
}

// matchTests returns the subset of tests whose on-globs match any changed file.
// Tests with no on-globs are always included (they can't be scoped).
func matchTests(tests []discovery.Test, changed []string, filter string) []discovery.Test {
	var affected []discovery.Test
	for _, t := range tests {
		if filter != "" {
			if matched, _ := filepath.Match(filter, t.Name); !matched {
				continue
			}
		}
		if len(t.On) == 0 {
			// Tests without on-globs run on any change
			affected = append(affected, t)
			continue
		}
		if testMatchesAny(t, changed) {
			affected = append(affected, t)
		}
	}
	return affected
}

// testMatchesAny returns true if any changed file matches any of the test's on-globs.
func testMatchesAny(t discovery.Test, changed []string) bool {
	for _, file := range changed {
		for _, pattern := range t.On {
			if glob.Match(pattern, file) {
				return true
			}
		}
	}
	return false
}
