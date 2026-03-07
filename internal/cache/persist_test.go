package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew_EmptyCache(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	if _, ok := c.GetEntry("missing"); ok {
		t.Error("expected GetEntry on empty cache to return ok=false")
	}
}

func TestUpdate_GetEntry(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	hashes := map[string]string{"main.go": "abc123"}
	c.Update("my-test", "pass", hashes, "all good")

	entry, ok := c.GetEntry("my-test")
	if !ok {
		t.Fatal("expected GetEntry to return ok=true after Update")
	}
	if entry.Result != "pass" {
		t.Errorf("Result = %q, want %q", entry.Result, "pass")
	}
	if entry.Reasoning != "all good" {
		t.Errorf("Reasoning = %q, want %q", entry.Reasoning, "all good")
	}
	if entry.FileHashes["main.go"] != "abc123" {
		t.Errorf("FileHashes[main.go] = %q, want %q", entry.FileHashes["main.go"], "abc123")
	}
	if entry.LastRun.IsZero() {
		t.Error("expected LastRun to be set")
	}
}

func TestUpdate_LastWriteWins(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	c.Update("t", "pass", nil, "first")
	c.Update("t", "fail", nil, "second")

	entry, _ := c.GetEntry("t")
	if entry.Result != "fail" || entry.Reasoning != "second" {
		t.Errorf("expected last write to win; got Result=%q Reasoning=%q", entry.Result, entry.Reasoning)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	c.Update("alpha", "pass", map[string]string{"a.go": "h1"}, "ok")
	c.Update("beta", "fail", nil, "nope")

	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	c2, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	e, ok := c2.GetEntry("alpha")
	if !ok {
		t.Fatal("expected 'alpha' entry after load")
	}
	if e.Result != "pass" || e.FileHashes["a.go"] != "h1" {
		t.Errorf("alpha entry mismatch: %+v", e)
	}

	e2, ok := c2.GetEntry("beta")
	if !ok {
		t.Fatal("expected 'beta' entry after load")
	}
	if e2.Result != "fail" || e2.Reasoning != "nope" {
		t.Errorf("beta entry mismatch: %+v", e2)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load on missing file should succeed, got: %v", err)
	}
	if _, ok := c.GetEntry("anything"); ok {
		t.Error("expected empty cache from missing file")
	}
}

func TestLoad_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")
	if err := os.WriteFile(path, []byte("not-json{{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load with corrupted JSON should not error, got: %v", err)
	}
	if _, ok := c.GetEntry("anything"); ok {
		t.Error("expected empty cache from corrupted JSON")
	}
}

func TestClear(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	c.Update("x", "pass", nil, "")
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "results.json")); err != nil {
		t.Fatal("expected results.json to exist before Clear")
	}

	if err := c.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	// After clear, Load should return an empty cache
	c2, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after Clear: %v", err)
	}
	if _, ok := c2.GetEntry("x"); ok {
		t.Error("expected empty cache after Clear")
	}
}

func TestClear_NoCacheFile(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	// Clear on a non-existent file should return an error (os.Remove fails)
	err := c.Clear()
	if err == nil {
		t.Error("expected error when clearing non-existent cache file")
	}
}
