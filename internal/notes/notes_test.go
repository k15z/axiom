package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	s := Load(dir)
	if s.Codebase != nil {
		t.Error("expected nil codebase notes")
	}
	if len(s.Tests) != 0 {
		t.Errorf("expected empty tests map, got %d entries", len(s.Tests))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()

	// Create a test file for hashing
	testFile := filepath.Join(root, "main.go")
	os.WriteFile(testFile, []byte("package main"), 0o644)

	s := Load(dir)
	s.UpdateCodebaseNotes("Go project with cobra CLI", []string{"main.go"}, root)
	s.UpdateTestNotes("test_auth", "Auth middleware at internal/auth.go:23", []string{"main.go"}, root)

	if err := s.Save(dir); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Reload
	s2 := Load(dir)
	if s2.Codebase == nil {
		t.Fatal("codebase notes lost after reload")
	}
	if s2.Codebase.Notes != "Go project with cobra CLI" {
		t.Errorf("codebase notes = %q, want %q", s2.Codebase.Notes, "Go project with cobra CLI")
	}

	notes, stale := s2.GetTestNotes("test_auth", root)
	if notes != "Auth middleware at internal/auth.go:23" {
		t.Errorf("test notes = %q, want %q", notes, "Auth middleware at internal/auth.go:23")
	}
	if stale {
		t.Error("notes should not be stale (file unchanged)")
	}
}

func TestStaleness(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()

	testFile := filepath.Join(root, "main.go")
	os.WriteFile(testFile, []byte("package main"), 0o644)

	s := Load(dir)
	s.UpdateTestNotes("test_foo", "Some notes", []string{"main.go"}, root)
	s.Save(dir)

	// Modify the file
	os.WriteFile(testFile, []byte("package main // modified"), 0o644)

	s2 := Load(dir)
	_, stale := s2.GetTestNotes("test_foo", root)
	if !stale {
		t.Error("notes should be stale after file modification")
	}
}

func TestStaleness_DeletedFile(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()

	testFile := filepath.Join(root, "main.go")
	os.WriteFile(testFile, []byte("package main"), 0o644)

	s := Load(dir)
	s.UpdateTestNotes("test_foo", "Some notes", []string{"main.go"}, root)
	s.Save(dir)

	// Delete the file
	os.Remove(testFile)

	s2 := Load(dir)
	_, stale := s2.GetTestNotes("test_foo", root)
	if !stale {
		t.Error("notes should be stale after file deletion")
	}
}

func TestStaleness_NoHashes(t *testing.T) {
	stale := IsStale(nil, "/tmp")
	if stale {
		t.Error("nil hashes should not be stale")
	}
	stale = IsStale(map[string]string{}, "/tmp")
	if stale {
		t.Error("empty hashes should not be stale")
	}
}

func TestGetMissing(t *testing.T) {
	s := &Store{Tests: make(map[string]*Entry)}

	notes, stale := s.GetTestNotes("nonexistent", "/tmp")
	if notes != "" {
		t.Errorf("expected empty notes for missing test, got %q", notes)
	}
	if stale {
		t.Error("missing notes should not be stale")
	}

	notes, stale = s.GetCodebaseNotes("/tmp")
	if notes != "" {
		t.Errorf("expected empty codebase notes, got %q", notes)
	}
	if stale {
		t.Error("missing codebase notes should not be stale")
	}
}

func TestTruncation(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()

	longNotes := strings.Repeat("x", maxNotesLen+100)

	s := Load(dir)
	s.UpdateTestNotes("test_long", longNotes, nil, root)

	entry := s.Tests["test_long"]
	if len(entry.Notes) > maxNotesLen {
		t.Errorf("notes should be truncated to %d chars, got %d", maxNotesLen, len(entry.Notes))
	}
	if !strings.HasSuffix(entry.Notes, "...") {
		t.Error("truncated notes should end with ellipsis")
	}
}

func TestEmptyNotesNotStored(t *testing.T) {
	s := &Store{Tests: make(map[string]*Entry)}
	s.UpdateTestNotes("test_empty", "", nil, "/tmp")
	if _, ok := s.Tests["test_empty"]; ok {
		t.Error("empty notes should not be stored")
	}
	s.UpdateCodebaseNotes("", nil, "/tmp")
	if s.Codebase != nil {
		t.Error("empty codebase notes should not be stored")
	}
}
