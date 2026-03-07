package cache

import (
	"os"
	"path/filepath"
	"testing"
)

// setupRepo creates a temp dir with the given files and returns the path.
func setupRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestShouldSkip(t *testing.T) {
	repoFiles := map[string]string{
		"src/main.go":  "package main",
		"src/utils.go": "package main",
	}
	globs := []string{"src/*.go"}
	configHash := HashConfig("claude-haiku-4-5-20251001", 30, 10000)

	tests := []struct {
		name     string
		seed     func(c *Cache, root string) // seed cache before test
		mutate   func(root string)           // mutate repo files before ShouldSkip
		wantSkip bool
	}{
		{
			name:     "no cache entry",
			seed:     func(c *Cache, root string) {},
			wantSkip: false,
		},
		{
			name: "cached fail",
			seed: func(c *Cache, root string) {
				hashes := HashGlobs(globs, root)
				c.Update("test1", "fail", hashes, "")
			},
			wantSkip: false,
		},
		{
			name: "cached pass unchanged files",
			seed: func(c *Cache, root string) {
				hashes := HashGlobs(globs, root)
				c.Update("test1", "pass", hashes, "")
			},
			wantSkip: true,
		},
		{
			name: "cached pass file content changed",
			seed: func(c *Cache, root string) {
				hashes := HashGlobs(globs, root)
				c.Update("test1", "pass", hashes, "")
			},
			mutate: func(root string) {
				os.WriteFile(filepath.Join(root, "src/main.go"), []byte("package changed"), 0o644)
			},
			wantSkip: false,
		},
		{
			name: "cached pass file added",
			seed: func(c *Cache, root string) {
				hashes := HashGlobs(globs, root)
				c.Update("test1", "pass", hashes, "")
			},
			mutate: func(root string) {
				os.WriteFile(filepath.Join(root, "src/new.go"), []byte("package main"), 0o644)
			},
			wantSkip: false,
		},
		{
			name: "cached pass file deleted",
			seed: func(c *Cache, root string) {
				hashes := HashGlobs(globs, root)
				c.Update("test1", "pass", hashes, "")
			},
			mutate: func(root string) {
				os.Remove(filepath.Join(root, "src/utils.go"))
			},
			wantSkip: false,
		},
		{
			name: "cached pass config hash changed",
			seed: func(c *Cache, root string) {
				// Seed with a different config hash (simulates old run with different model)
				old := New(c.dir, HashConfig("claude-opus-4-6", 30, 10000))
				hashes := HashGlobs(globs, root)
				old.Update("test1", "pass", hashes, "")
				c.entries = old.entries
			},
			wantSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := setupRepo(t, repoFiles)
			c := New(t.TempDir(), configHash)

			tt.seed(c, root)
			if tt.mutate != nil {
				tt.mutate(root)
			}

			skip, current := c.ShouldSkip("test1", globs, root)
			if skip != tt.wantSkip {
				t.Errorf("ShouldSkip() = %v, want %v", skip, tt.wantSkip)
			}
			if current == nil {
				t.Error("ShouldSkip() returned nil current hashes")
			}
		})
	}
}

func TestPrevResultTracking(t *testing.T) {
	c := New(t.TempDir(), "hash1")
	hashes := map[string]string{"a.go": "abc123"}

	// First update — no previous result
	c.Update("test1", "pass", hashes, "all good")
	entry, ok := c.GetEntry("test1")
	if !ok {
		t.Fatal("expected entry")
	}
	if entry.PrevResult != "" {
		t.Errorf("first update PrevResult = %q, want empty", entry.PrevResult)
	}

	// Second update — previous result should be "pass"
	c.Update("test1", "fail", hashes, "broke")
	entry, _ = c.GetEntry("test1")
	if entry.PrevResult != "pass" {
		t.Errorf("PrevResult = %q, want %q", entry.PrevResult, "pass")
	}
	if entry.Result != "fail" {
		t.Errorf("Result = %q, want %q", entry.Result, "fail")
	}

	// Third update — previous result should be "fail"
	c.Update("test1", "pass", hashes, "fixed")
	entry, _ = c.GetEntry("test1")
	if entry.PrevResult != "fail" {
		t.Errorf("PrevResult = %q, want %q", entry.PrevResult, "fail")
	}
}

func TestIsFlaky(t *testing.T) {
	c := New(t.TempDir(), "hash1")
	hashes := map[string]string{"a.go": "abc123"}

	// No entry — not flaky
	if c.IsFlaky("test1") {
		t.Error("expected not flaky for missing entry")
	}

	// First update — no prev result — not flaky
	c.Update("test1", "pass", hashes, "")
	if c.IsFlaky("test1") {
		t.Error("expected not flaky after first run")
	}

	// Same result — not flaky
	c.Update("test1", "pass", hashes, "")
	if c.IsFlaky("test1") {
		t.Error("expected not flaky when result unchanged")
	}

	// Different result — flaky
	c.Update("test1", "fail", hashes, "")
	if !c.IsFlaky("test1") {
		t.Error("expected flaky when result flipped")
	}
}

func TestHashConfig(t *testing.T) {
	h1 := HashConfig("claude-haiku-4-5-20251001", 30, 10000)
	h2 := HashConfig("claude-haiku-4-5-20251001", 30, 10000)
	h3 := HashConfig("claude-opus-4-6", 30, 10000)
	h4 := HashConfig("claude-haiku-4-5-20251001", 20, 10000)

	if h1 != h2 {
		t.Error("same inputs should produce same hash")
	}
	if h1 == h3 {
		t.Error("different model should produce different hash")
	}
	if h1 == h4 {
		t.Error("different max_iterations should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got %d chars", len(h1))
	}

	// Extra params (provider, base_url) change the hash
	h5 := HashConfig("claude-haiku-4-5-20251001", 30, 10000, "anthropic", "")
	h6 := HashConfig("claude-haiku-4-5-20251001", 30, 10000, "openai", "")
	h7 := HashConfig("claude-haiku-4-5-20251001", 30, 10000, "openai", "http://localhost:11434/v1")

	if h1 == h5 {
		t.Error("adding provider extra should produce different hash")
	}
	if h5 == h6 {
		t.Error("different provider should produce different hash")
	}
	if h6 == h7 {
		t.Error("different base_url should produce different hash")
	}

	// Same extra params produce same hash
	h8 := HashConfig("claude-haiku-4-5-20251001", 30, 10000, "anthropic", "")
	if h5 != h8 {
		t.Error("same extra params should produce same hash")
	}
}
