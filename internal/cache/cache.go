package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/k15z/axiom/internal/glob"
)

type Entry struct {
	LastRun    time.Time         `json:"last_run"`
	FileHashes map[string]string `json:"file_hashes"`
	Result     string            `json:"result"` // "pass" or "fail"
	Reasoning  string            `json:"reasoning,omitempty"`
}

type Cache struct {
	dir     string
	entries map[string]Entry
}

func New(dir string) *Cache {
	return &Cache{
		dir:     dir,
		entries: make(map[string]Entry),
	}
}

func Load(dir string) (*Cache, error) {
	c := New(dir)
	path := c.filePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	if err := json.Unmarshal(data, &c.entries); err != nil {
		// Corrupted cache — start fresh
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

func (c *Cache) Update(testName string, result string, fileHashes map[string]string, reasoning string) {
	c.entries[testName] = Entry{
		LastRun:    time.Now(),
		FileHashes: fileHashes,
		Result:     result,
		Reasoning:  reasoning,
	}
}

func (c *Cache) Save() error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	return os.WriteFile(c.filePath(), data, 0o644)
}

func (c *Cache) Clear() error {
	return os.Remove(c.filePath())
}

func (c *Cache) GetEntry(testName string) (Entry, bool) {
	e, ok := c.entries[testName]
	return e, ok
}

func (c *Cache) filePath() string {
	return filepath.Join(c.dir, "results.json")
}

// HashGlobs hashes files matching the given globs. Exported for use by runner.
func HashGlobs(globs []string, root string) map[string]string {
	return glob.HashFiles(globs, root)
}
