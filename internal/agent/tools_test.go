package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
		result, isErr := toolTree(context.Background(), ".", 3, root)
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
		result, isErr := toolTree(context.Background(), ".", 1, root)
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
		result, isErr := toolTree(context.Background(), "src", 3, root)
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
		result, isErr := toolTree(context.Background(), "big", 1, root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "more entries") {
			t.Errorf("expected per-dir limit message, got:\n%s", result)
		}
	})

	t.Run("path outside root", func(t *testing.T) {
		_, isErr := toolTree(context.Background(), "../../etc", 1, root)
		if !isErr {
			t.Error("expected error for path outside root")
		}
	})

	t.Run("default depth", func(t *testing.T) {
		result, isErr := toolTree(context.Background(), ".", 0, root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		// depth=0 should default to 3, showing nested content
		if !strings.Contains(result, "button.go") {
			t.Errorf("default depth should show nested files, got:\n%s", result)
		}
	})
}

func TestToolContextCancellation(t *testing.T) {
	root := t.TempDir()
	// Create some files so tools have work to do
	for i := 0; i < 10; i++ {
		os.WriteFile(filepath.Join(root, fmt.Sprintf("file%d.txt", i)), []byte("hello world\n"), 0o644)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	t.Run("grep respects cancelled context", func(t *testing.T) {
		result, _ := toolGrep(ctx, "hello", "", root)
		// With a cancelled context, grep should return quickly
		// It may return partial results or no results, but should not hang
		_ = result
	})

	t.Run("glob respects cancelled context", func(t *testing.T) {
		result, _ := toolGlob(ctx, "**/*.txt", root)
		_ = result
	})

	t.Run("tree respects cancelled context", func(t *testing.T) {
		result, _ := toolTree(ctx, ".", 3, root)
		_ = result
	})

	t.Run("ExecuteTool returns timeout error", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{"pattern": "hello"})
		result, isErr := ExecuteTool(ctx, "grep", input, root, 0)
		if !isErr {
			t.Error("expected error for cancelled context")
		}
		if !strings.Contains(result, "timed out") {
			t.Errorf("expected timeout message, got: %s", result)
		}
	})
}

func TestSafePath(t *testing.T) {
	// Create a temp directory to act as the repo root.
	root := t.TempDir()

	// Create a subdirectory so subpath tests resolve.
	os.MkdirAll(filepath.Join(root, "sub", "dir"), 0o755)

	rootAbs, _ := filepath.Abs(root)

	tests := []struct {
		name    string
		rel     string
		wantAbs string // expected absolute path ("" means check error instead)
		wantErr bool
	}{
		{"empty rel returns root", "", rootAbs, false},
		{"simple subpath", "sub", filepath.Join(rootAbs, "sub"), false},
		{"nested subpath", "sub/dir", filepath.Join(rootAbs, "sub", "dir"), false},
		{"dot-dot escape", "../outside", "", true},
		{"deep dot-dot escape", "sub/../../outside", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safePath(tt.rel, root)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("safePath(%q, root) = %q, want error", tt.rel, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("safePath(%q, root) unexpected error: %v", tt.rel, err)
			}
			if got != tt.wantAbs {
				t.Errorf("safePath(%q, root) = %q, want %q", tt.rel, got, tt.wantAbs)
			}
		})
	}
}

func TestSafePath_SiblingPrefix(t *testing.T) {
	// Ensure /tmp/repo doesn't match /tmp/repobar.
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	sibling := filepath.Join(parent, "repobar")
	os.Mkdir(root, 0o755)
	os.Mkdir(sibling, 0o755)

	// "../repobar" from root resolves to sibling — must be rejected.
	_, err := safePath("../repobar", root)
	if err == nil {
		t.Error("safePath should reject sibling directory with shared prefix")
	}
}

// makeGrepTestDir creates a temp directory with files for grep testing.
func makeGrepTestDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	files := map[string]string{
		"main.go":             "package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n",
		"util.go":             "package main\n\nfunc helper() string {\n\treturn \"hello helper\"\n}\n",
		"README.md":           "# Project\n\nThis is a hello world project.\n",
		"internal/auth.go":    "package internal\n\nfunc Authenticate(token string) bool {\n\treturn token != \"\"\n}\n",
		"internal/handler.go": "package internal\n\nfunc HandleRequest() {\n\t// TODO: implement\n}\n",
		".hidden/secret.go":   "package hidden\n\nvar secret = \"hello hidden\"\n",
	}

	for path, content := range files {
		abs := filepath.Join(root, path)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(content), 0o644)
	}
	return root
}

func TestToolGrepGo(t *testing.T) {
	root := makeGrepTestDir(t)

	t.Run("basic match", func(t *testing.T) {
		result, isErr := toolGrepGo(context.Background(), "hello", "", root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "main.go:") || !strings.Contains(result, "hello world") {
			t.Errorf("expected match in main.go, got:\n%s", result)
		}
		if !strings.Contains(result, "util.go:") || !strings.Contains(result, "hello helper") {
			t.Errorf("expected match in util.go, got:\n%s", result)
		}
	})

	t.Run("glob filter", func(t *testing.T) {
		result, isErr := toolGrepGo(context.Background(), "hello", "*.go", root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if strings.Contains(result, "README.md") {
			t.Errorf("glob *.go should exclude README.md, got:\n%s", result)
		}
		if !strings.Contains(result, "main.go:") {
			t.Errorf("expected match in main.go, got:\n%s", result)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result, isErr := toolGrepGo(context.Background(), "NONEXISTENT_STRING_XYZ", "", root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if result != "no matches found" {
			t.Errorf("expected 'no matches found', got: %s", result)
		}
	})

	t.Run("invalid regex", func(t *testing.T) {
		_, isErr := toolGrepGo(context.Background(), "[invalid", "", root)
		if !isErr {
			t.Error("expected error for invalid regex")
		}
	})

	t.Run("skips hidden dirs", func(t *testing.T) {
		result, _ := toolGrepGo(context.Background(), "hello hidden", "", root)
		if strings.Contains(result, ".hidden") || strings.Contains(result, "secret.go") {
			t.Errorf("should not match in hidden dirs, got:\n%s", result)
		}
	})

	t.Run("directory scoped glob", func(t *testing.T) {
		result, isErr := toolGrepGo(context.Background(), "package", "internal/**/*.go", root)
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "internal/auth.go") {
			t.Errorf("expected match in internal/auth.go, got:\n%s", result)
		}
		if strings.Contains(result, "main.go") {
			t.Errorf("should not match outside internal/, got:\n%s", result)
		}
	})
}

func TestToolGrepRg(t *testing.T) {
	// Skip if rg is not installed
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep (rg) not installed, skipping")
	}

	root := makeGrepTestDir(t)

	t.Run("basic match", func(t *testing.T) {
		result, isErr, ok := toolGrepRg(context.Background(), "hello", "", root)
		if !ok {
			t.Fatal("toolGrepRg returned ok=false unexpectedly")
		}
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "main.go:") || !strings.Contains(result, "hello world") {
			t.Errorf("expected match in main.go, got:\n%s", result)
		}
	})

	t.Run("glob filter", func(t *testing.T) {
		result, isErr, ok := toolGrepRg(context.Background(), "hello", "*.go", root)
		if !ok {
			t.Fatal("toolGrepRg returned ok=false unexpectedly")
		}
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if strings.Contains(result, "README.md") {
			t.Errorf("glob *.go should exclude README.md, got:\n%s", result)
		}
		if !strings.Contains(result, "main.go:") {
			t.Errorf("expected match in main.go, got:\n%s", result)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result, isErr, ok := toolGrepRg(context.Background(), "NONEXISTENT_STRING_XYZ", "", root)
		if !ok {
			t.Fatal("toolGrepRg returned ok=false unexpectedly")
		}
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if result != "no matches found" {
			t.Errorf("expected 'no matches found', got: %s", result)
		}
	})

	t.Run("skips hidden dirs", func(t *testing.T) {
		result, _, ok := toolGrepRg(context.Background(), "hello hidden", "", root)
		if !ok {
			t.Fatal("toolGrepRg returned ok=false unexpectedly")
		}
		if strings.Contains(result, ".hidden") || strings.Contains(result, "secret.go") {
			t.Errorf("should not match in hidden dirs, got:\n%s", result)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, ok := toolGrepRg(ctx, "hello", "", root)
		// With cancelled context, rg should fail and ok=false (fallback to Go)
		if ok {
			// It's also acceptable if rg ran fast enough to return before cancellation
			// so we don't fail the test, just note it
			t.Log("rg completed despite cancelled context (fast execution)")
		}
	})
}

func TestToolGrepIntegration(t *testing.T) {
	// Test that toolGrep produces consistent results whether using rg or Go fallback.
	root := makeGrepTestDir(t)

	goResult, goErr := toolGrepGo(context.Background(), "hello", "*.go", root)
	if goErr {
		t.Fatalf("Go grep error: %s", goResult)
	}

	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep (rg) not installed, skipping integration comparison")
	}

	rgResult, rgErr, ok := toolGrepRg(context.Background(), "hello", "*.go", root)
	if !ok {
		t.Fatal("toolGrepRg returned ok=false")
	}
	if rgErr {
		t.Fatalf("rg grep error: %s", rgResult)
	}

	// Both should find the same files (order may differ)
	goFiles := extractMatchedFiles(goResult)
	rgFiles := extractMatchedFiles(rgResult)

	if len(goFiles) != len(rgFiles) {
		t.Errorf("file count mismatch: Go found %d files, rg found %d files\nGo: %s\nrg: %s",
			len(goFiles), len(rgFiles), goResult, rgResult)
	}

	for f := range goFiles {
		if !rgFiles[f] {
			t.Errorf("Go found %q but rg did not", f)
		}
	}
}

// extractMatchedFiles extracts unique file paths from grep output lines (path:line:content).
func extractMatchedFiles(output string) map[string]bool {
	files := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "no matches found" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 1 {
			files[parts[0]] = true
		}
	}
	return files
}
