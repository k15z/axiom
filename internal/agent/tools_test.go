package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolTree(t *testing.T) {
	// Create a temp directory structure
	root := t.TempDir()
	dirs := []string{
		"src",
		"src/components",
		"src/utils",
		"docs",
	}
	files := []string{
		"README.md",
		"src/main.go",
		"src/components/button.go",
		"src/components/input.go",
		"src/utils/helpers.go",
		"docs/guide.md",
		".hidden/secret.txt",
	}
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	os.MkdirAll(filepath.Join(root, ".hidden"), 0o755)
	for _, f := range files {
		os.WriteFile(filepath.Join(root, f), []byte("test"), 0o644)
	}

	t.Run("basic tree", func(t *testing.T) {
		result, isErr := toolTree(".", 3, root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		// Should contain files and dirs
		for _, want := range []string{"src/", "docs/", "README.md", "main.go", "button.go", "helpers.go", "guide.md"} {
			if !strings.Contains(result, want) {
				t.Errorf("expected %q in output, got:\n%s", want, result)
			}
		}
		// Should not contain hidden dirs
		if strings.Contains(result, ".hidden") || strings.Contains(result, "secret.txt") {
			t.Errorf("should not contain hidden entries, got:\n%s", result)
		}
	})

	t.Run("depth limit", func(t *testing.T) {
		result, isErr := toolTree(".", 1, root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		// Should show top-level dirs but not their contents
		if !strings.Contains(result, "src/") {
			t.Errorf("expected src/ in output, got:\n%s", result)
		}
		if strings.Contains(result, "main.go") {
			t.Errorf("depth=1 should not show nested files, got:\n%s", result)
		}
	})

	t.Run("subdirectory", func(t *testing.T) {
		result, isErr := toolTree("src", 3, root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "components/") {
			t.Errorf("expected components/ in output, got:\n%s", result)
		}
	})

	t.Run("per-dir limit", func(t *testing.T) {
		// Create a directory with many files
		bigDir := filepath.Join(root, "big")
		os.MkdirAll(bigDir, 0o755)
		for i := 0; i < 100; i++ {
			os.WriteFile(filepath.Join(bigDir, strings.Repeat("a", 3)+string(rune('a'+i/26))+string(rune('a'+i%26))+".txt"), []byte("test"), 0o644)
		}
		result, isErr := toolTree("big", 1, root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "more entries") {
			t.Errorf("expected per-dir limit message, got:\n%s", result)
		}
	})

	t.Run("path outside root", func(t *testing.T) {
		_, isErr := toolTree("../../etc", 1, root)
		if !isErr {
			t.Error("expected error for path outside root")
		}
	})

	t.Run("default depth", func(t *testing.T) {
		result, isErr := toolTree(".", 0, root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		// depth=0 should default to 3, showing nested content
		if !strings.Contains(result, "button.go") {
			t.Errorf("default depth should show nested files, got:\n%s", result)
		}
	})
}
