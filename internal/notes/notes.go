// Package notes provides persistent agent memory for codebase exploration.
// Notes are cached observations from previous agent runs that get injected
// as context in subsequent runs, reducing redundant codebase exploration.
package notes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const maxNotesLen = 2000 // max characters per notes entry (~500 tokens)

// Store holds codebase-level and per-test notes.
type Store struct {
	Codebase *Entry            `json:"codebase,omitempty"`
	Tests    map[string]*Entry `json:"tests,omitempty"`
}

// Entry is a single notes record with staleness tracking.
type Entry struct {
	UpdatedAt  time.Time         `json:"updated_at"`
	Notes      string            `json:"notes"`
	FileHashes map[string]string `json:"file_hashes"` // path -> sha256
}

// Load reads notes from disk. Returns an empty store if the file doesn't exist.
func Load(cacheDir string) *Store {
	s := &Store{Tests: make(map[string]*Entry)}
	data, err := os.ReadFile(filePath(cacheDir))
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, s); err != nil {
		return &Store{Tests: make(map[string]*Entry)}
	}
	if s.Tests == nil {
		s.Tests = make(map[string]*Entry)
	}
	return s
}

// Save writes notes to disk.
func (s *Store) Save(cacheDir string) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath(cacheDir), data, 0o644)
}

// GetTestNotes returns the notes for a test with staleness info.
// Returns empty string if no notes exist.
func (s *Store) GetTestNotes(testName string, repoRoot string) (notes string, stale bool) {
	entry, ok := s.Tests[testName]
	if !ok || entry.Notes == "" {
		return "", false
	}
	return entry.Notes, IsStale(entry.FileHashes, repoRoot)
}

// GetCodebaseNotes returns codebase-level notes with staleness info.
func (s *Store) GetCodebaseNotes(repoRoot string) (notes string, stale bool) {
	if s.Codebase == nil || s.Codebase.Notes == "" {
		return "", false
	}
	return s.Codebase.Notes, IsStale(s.Codebase.FileHashes, repoRoot)
}

// UpdateTestNotes stores notes for a specific test.
func (s *Store) UpdateTestNotes(testName string, notesText string, files []string, repoRoot string) {
	if notesText == "" {
		return
	}
	notesText = truncate(notesText, maxNotesLen)
	s.Tests[testName] = &Entry{
		UpdatedAt:  time.Now(),
		Notes:      notesText,
		FileHashes: hashFiles(files, repoRoot),
	}
}

// UpdateCodebaseNotes stores codebase-level notes.
func (s *Store) UpdateCodebaseNotes(notesText string, files []string, repoRoot string) {
	if notesText == "" {
		return
	}
	notesText = truncate(notesText, maxNotesLen)
	s.Codebase = &Entry{
		UpdatedAt:  time.Now(),
		Notes:      notesText,
		FileHashes: hashFiles(files, repoRoot),
	}
}

// IsStale returns true if any file hash has changed from the stored value.
func IsStale(stored map[string]string, repoRoot string) bool {
	if len(stored) == 0 {
		return false
	}
	for path, oldHash := range stored {
		abs := filepath.Join(repoRoot, path)
		data, err := os.ReadFile(abs)
		if err != nil {
			return true // file deleted or unreadable
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != oldHash {
			return true
		}
	}
	return false
}

func hashFiles(paths []string, repoRoot string) map[string]string {
	hashes := make(map[string]string, len(paths))
	for _, path := range paths {
		abs := filepath.Join(repoRoot, path)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(data)
		hashes[path] = hex.EncodeToString(sum[:])
	}
	return hashes
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// Truncate to max bytes minus space for ellipsis, ensuring valid UTF-8
	truncated := s[:max-3] + "..."
	return truncated
}

func filePath(cacheDir string) string {
	return filepath.Join(cacheDir, "notes.json")
}
