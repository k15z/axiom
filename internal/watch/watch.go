// Package watch implements file watching for axiom's watch mode.
// It uses fsnotify for filesystem events and debounces rapid changes.
package watch

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/glob"
	"github.com/k15z/axiom/internal/output"
	"github.com/k15z/axiom/internal/runner"
)

const debounceDelay = 500 * time.Millisecond

// Run enters watch mode: runs matching tests once, then watches for file
// changes and re-runs affected tests. Blocks until ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, opts runner.Options) error {
	repoRoot, _ := filepath.Abs(".")

	gray := color.New(color.FgHiBlack)
	cyan := color.New(color.FgCyan)

	// Initial discovery and run
	tests, err := discovery.Discover(cfg.TestDir)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n")
	cyan.Fprintf(os.Stderr, "  ▶ Running all tests…\n")
	fmt.Fprintf(os.Stderr, "\n")

	results, err := runner.Run(ctx, cfg, tests, opts)
	if err != nil {
		return err
	}
	output.Print(results, cfg.Model, opts.Verbose, cfg.TestDir)

	// Set up fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating file watcher: %w", err)
	}
	defer watcher.Close()

	// Watch directories that contain files matching test on-globs,
	// plus the test directory itself for definition changes.
	if err := addWatchDirs(watcher, repoRoot, tests, cfg.TestDir); err != nil {
		return fmt.Errorf("setting up watches: %w", err)
	}

	gray.Fprintf(os.Stderr, "\n  Watching for file changes… (Ctrl+C to exit)\n\n")

	// Debounce loop: collect changed files, then trigger after quiet period
	var mu sync.Mutex
	changedFiles := make(map[string]struct{})
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "\n")
			gray.Fprintf(os.Stderr, "  Watch mode stopped.\n\n")
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			rel, err := filepath.Rel(repoRoot, event.Name)
			if err != nil {
				continue
			}

			// Skip cache directory to avoid feedback loops
			if isUnderCacheDir(rel, cfg.Cache.Dir, repoRoot) {
				continue
			}

			mu.Lock()
			changedFiles[rel] = struct{}{}
			timer.Reset(debounceDelay)
			mu.Unlock()

			// If a new directory was created, watch it too
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = watcher.Add(event.Name)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "  watch error: %v\n", err)

		case <-timer.C:
			mu.Lock()
			files := make([]string, 0, len(changedFiles))
			for f := range changedFiles {
				files = append(files, f)
			}
			changedFiles = make(map[string]struct{})
			mu.Unlock()

			if len(files) == 0 {
				continue
			}

			// Check if test definitions changed — re-discover if so
			testDirRel, _ := filepath.Rel(repoRoot, cfg.TestDir)
			testsChanged := false
			for _, f := range files {
				if strings.HasPrefix(f, testDirRel+string(filepath.Separator)) || f == testDirRel {
					testsChanged = true
					break
				}
			}

			if testsChanged {
				newTests, err := discovery.Discover(cfg.TestDir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  discovery error: %v\n", err)
				} else {
					tests = newTests
					// Re-add watches in case new globs appeared
					_ = addWatchDirs(watcher, repoRoot, tests, cfg.TestDir)
				}
			}

			// Find affected tests
			affected := MatchTests(tests, files, opts.Filter, opts.Tag)
			if len(affected) == 0 {
				continue
			}

			// Show trigger info
			fmt.Fprintf(os.Stderr, "\n")
			cyan.Fprintf(os.Stderr, "  ▶ %d file(s) changed", len(files))
			if len(files) <= 3 {
				names := make([]string, 0, len(files))
				for _, f := range files {
					names = append(names, f)
				}
				cyan.Fprintf(os.Stderr, ": %s", strings.Join(names, ", "))
			}
			fmt.Fprintf(os.Stderr, "\n")
			gray.Fprintf(os.Stderr, "  Re-running %d affected test(s)…\n\n", len(affected))

			// Re-run affected tests, ignoring cache (user just changed the file)
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
			output.Print(results, cfg.Model, opts.Verbose, cfg.TestDir)

			gray.Fprintf(os.Stderr, "\n  Watching for file changes… (Ctrl+C to exit)\n\n")
		}
	}
}

// addWatchDirs adds directories to the fsnotify watcher based on test on-globs.
// It watches all non-hidden directories under root (since globs can use **),
// plus the test definition directory.
func addWatchDirs(watcher *fsnotify.Watcher, root string, tests []discovery.Test, testDir string) error {
	// Collect all directories that might contain watched files.
	// Since globs can use **, we need to watch recursively.
	// fsnotify watches directories, not individual files.
	dirs := make(map[string]struct{})

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		// Skip hidden dirs except .axiom (test definitions)
		if strings.HasPrefix(name, ".") && path != root {
			if name == ".axiom" {
				dirs[path] = struct{}{}
				return nil // continue into .axiom but skip .axiom/.cache below
			}
			return filepath.SkipDir
		}
		dirs[path] = struct{}{}
		return nil
	})

	// Also make sure test dir itself is watched
	absTestDir, _ := filepath.Abs(testDir)
	dirs[absTestDir] = struct{}{}

	for dir := range dirs {
		// Skip the cache directory
		rel, _ := filepath.Rel(root, dir)
		cacheRel, _ := filepath.Rel(root, filepath.Join(root, ".axiom", ".cache"))
		if strings.HasPrefix(rel, cacheRel) {
			continue
		}
		_ = watcher.Add(dir)
	}

	return nil
}

// isUnderCacheDir checks if a relative path falls under the cache directory.
func isUnderCacheDir(rel string, cacheDir string, repoRoot string) bool {
	absCacheDir, _ := filepath.Abs(cacheDir)
	relCache, err := filepath.Rel(repoRoot, absCacheDir)
	if err != nil {
		return false
	}
	return strings.HasPrefix(rel, relCache+string(filepath.Separator)) || rel == relCache
}

// MatchTests returns tests whose on-globs match any of the changed files.
// Tests with no on-globs are always included.
func MatchTests(tests []discovery.Test, changed []string, filter string, tag string) []discovery.Test {
	var affected []discovery.Test
	for _, t := range tests {
		if filter != "" {
			if matched, _ := filepath.Match(filter, t.Name); !matched {
				continue
			}
		}
		if !runner.MatchesTag(t, tag) {
			continue
		}
		if len(t.On) == 0 {
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
