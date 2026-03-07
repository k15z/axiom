package glob

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHashFiles_NoPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", "package main")
	got := HashFiles(nil, dir)
	if len(got) != 0 {
		t.Errorf("expected empty map for nil patterns, got %v", got)
	}
	got = HashFiles([]string{}, dir)
	if len(got) != 0 {
		t.Errorf("expected empty map for empty patterns, got %v", got)
	}
}

func TestHashFiles_NoMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", "package main")
	got := HashFiles([]string{"*.txt"}, dir)
	if len(got) != 0 {
		t.Errorf("expected empty map when no files match, got %v", got)
	}
}

func TestHashFiles_SinglePattern(t *testing.T) {
	dir := t.TempDir()
	content := "package main\n"
	writeFile(t, dir, "a.go", content)
	writeFile(t, dir, "b.go", content)
	writeFile(t, dir, "c.txt", "ignore")

	got := HashFiles([]string{"*.go"}, dir)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(got), got)
	}
	want := sha256hex([]byte(content))
	for _, key := range []string{"a.go", "b.go"} {
		if got[key] != want {
			t.Errorf("hash for %s = %q, want %q", key, got[key], want)
		}
	}
}

func TestHashFiles_MultiplePatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", "go")
	writeFile(t, dir, "b.txt", "txt")
	writeFile(t, dir, "c.md", "md")

	got := HashFiles([]string{"*.go", "*.txt"}, dir)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(got), got)
	}
	if _, ok := got["a.go"]; !ok {
		t.Error("expected a.go in result")
	}
	if _, ok := got["b.txt"]; !ok {
		t.Error("expected b.txt in result")
	}
	if _, ok := got["c.md"]; ok {
		t.Error("expected c.md NOT in result")
	}
}

func TestHashFiles_RecursiveDoubleStarPattern(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sub/a.go", "a")
	writeFile(t, dir, "sub/deep/b.go", "b")
	writeFile(t, dir, "root.go", "r")

	got := HashFiles([]string{"**/*.go"}, dir)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(got), got)
	}
	for _, key := range []string{"sub/a.go", "sub/deep/b.go", "root.go"} {
		if _, ok := got[key]; !ok {
			t.Errorf("expected %s in result", key)
		}
	}
}

func TestHashFiles_HiddenDirectoriesSkipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".hidden/secret.go", "secret")
	writeFile(t, dir, "visible.go", "visible")

	got := HashFiles([]string{"**/*.go"}, dir)
	if _, ok := got[".hidden/secret.go"]; ok {
		t.Error("hidden directory should be skipped")
	}
	if _, ok := got["visible.go"]; !ok {
		t.Error("expected visible.go in result")
	}
}

func TestHashFiles_ContentChangeChangesHash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.go", "version1")
	got1 := HashFiles([]string{"*.go"}, dir)

	writeFile(t, dir, "f.go", "version2")
	got2 := HashFiles([]string{"*.go"}, dir)

	if got1["f.go"] == got2["f.go"] {
		t.Error("expected different hashes for different content")
	}
}

func TestHashFiles_RelativePathsAsKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pkg/util.go", "x")

	got := HashFiles([]string{"**/*.go"}, dir)
	if _, ok := got["pkg/util.go"]; !ok {
		t.Errorf("expected relative key 'pkg/util.go', got keys: %v", got)
	}
}
