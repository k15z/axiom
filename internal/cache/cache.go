package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/k15z/axiom/internal/glob"
)

// Entry is a single cached test result with file hashes for invalidation.
type Entry struct {
	LastRun    time.Time         `json:"last_run"`
	FileHashes map[string]string `json:"file_hashes"`
	Result     string            `json:"result"`                // "pass" or "fail"
	PrevResult string            `json:"prev_result,omitempty"` // previous run's result for flaky detection
	Reasoning     string            `json:"reasoning,omitempty"`
	PrevReasoning string            `json:"prev_reasoning,omitempty"`
	ConfigHash    string            `json:"config_hash,omitempty"`
}

// Cache stores and retrieves test results keyed by test name and file hashes.
type Cache struct {
	dir        string
	entries    map[string]Entry
	configHash string
}

// HashConfig returns a SHA-256 digest of the agent config fields that affect verdicts.
func HashConfig(model string, maxIterations, maxTokens int, extra ...string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%d:%d", model, maxIterations, maxTokens)
	for _, e := range extra {
		fmt.Fprintf(h, ":%s", e)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// New creates an empty in-memory cache for the given directory and config hash.
func New(dir string, configHash string) *Cache {
	return &Cache{
		dir:        dir,
		entries:    make(map[string]Entry),
		configHash: configHash,
	}
}

// Load reads an existing cache from disk, or returns an empty cache if none exists.
func Load(dir string, configHash string) (*Cache, error) {
	c := New(dir, configHash)
	path := c.filePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	if err := json.Unmarshal(data, &c.entries); err != nil {
		// Corrupted cache — start fresh but warn the user
		fmt.Fprintf(os.Stderr, "warning: cache file corrupted, starting fresh (%s)\n", path)
		c.entries = make(map[string]Entry)
	}

	return c, nil
}

// ShouldSkip returns true if the test can be skipped (cached pass + unchanged files).
// Also returns the current file hashes for use when updating the cache.
func (c *Cache) ShouldSkip(testName string, onGlobs []string, repoRoot string) (bool, map[string]string) {
	current := glob.HashFiles(onGlobs, repoRoot)

	entry, ok := c.entries[testName]
	if !ok || entry.Result != "pass" {
		return false, current
	}

	if entry.ConfigHash != c.configHash {
		return false, current
	}

	if len(current) != len(entry.FileHashes) {
		return false, current
	}
	for path, hash := range current {
		if entry.FileHashes[path] != hash {
			return false, current
		}
	}

	return true, current
}

// Update records a test result and its associated file hashes in the cache.
func (c *Cache) Update(testName string, result string, fileHashes map[string]string, reasoning string) {
	prevResult := ""
	prevReasoning := ""
	if old, ok := c.entries[testName]; ok {
		prevResult = old.Result
		prevReasoning = old.Reasoning
	}
	c.entries[testName] = Entry{
		LastRun:       time.Now(),
		FileHashes:    fileHashes,
		Result:        result,
		PrevResult:    prevResult,
		Reasoning:     reasoning,
		PrevReasoning: prevReasoning,
		ConfigHash:    c.configHash,
	}
}

// IsFlaky returns true if the test's current and previous results differ.
func (c *Cache) IsFlaky(testName string) bool {
	entry, ok := c.entries[testName]
	if !ok || entry.PrevResult == "" {
		return false
	}
	return entry.Result != entry.PrevResult
}

// Save writes the in-memory cache to disk, creating the directory if needed.
func (c *Cache) Save() error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	if err := os.WriteFile(c.filePath(), data, 0o644); err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}
	return nil
}

// Clear removes the cache file from disk. No-ops if the file does not exist.
func (c *Cache) Clear() error {
	err := os.Remove(c.filePath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing cache file: %w", err)
	}
	return nil
}

// GetEntry returns the cached entry for testName, if present.
func (c *Cache) GetEntry(testName string) (Entry, bool) {
	e, ok := c.entries[testName]
	return e, ok
}

// Entries returns all cache entries. Used by cache info command.
func (c *Cache) Entries() map[string]Entry {
	return c.entries
}

// FilePath returns the path to the cache file on disk.
func (c *Cache) FilePath() string {
	return c.filePath()
}

func (c *Cache) filePath() string {
	return filepath.Join(c.dir, "results.json")
}

// HashGlobs hashes files matching the given globs. Exported for use by runner.
func HashGlobs(globs []string, root string) map[string]string {
	return glob.HashFiles(globs, root)
}
