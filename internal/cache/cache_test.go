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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := setupRepo(t, repoFiles)
			c := New(t.TempDir())

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
