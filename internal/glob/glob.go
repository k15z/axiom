// Package glob provides ** -aware glob matching and file hashing utilities
// shared by the agent tools and cache packages.
package glob

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Match reports whether path matches pattern.
// It supports ** to match zero or more path segments (directory recursion).
// Single-segment matching delegates to filepath.Match for standard glob syntax.
func Match(pattern, path string) bool {
	patParts := strings.Split(filepath.ToSlash(pattern), "/")
	pathParts := strings.Split(filepath.ToSlash(path), "/")
	return matchParts(patParts, pathParts)
}

func matchParts(pat, path []string) bool {
	for len(pat) > 0 && len(path) > 0 {
		if pat[0] == "**" {
			pat = pat[1:]
			if len(pat) == 0 {
				return true // ** at end matches everything remaining
			}
			// ** matches zero or more segments
			for i := 0; i <= len(path); i++ {
				if matchParts(pat, path[i:]) {
					return true
				}
			}
			return false
		}

		matched, err := filepath.Match(pat[0], path[0])
		if err != nil || !matched {
			return false
		}
		pat = pat[1:]
		path = path[1:]
	}

	// Consume any trailing **
	for len(pat) > 0 && pat[0] == "**" {
		pat = pat[1:]
	}

	return len(pat) == 0 && len(path) == 0
}

// HashFiles walks root and returns SHA-256 content hashes for all files
// matching any of the given patterns. Patterns support **.
// Hidden directories (name starts with ".") are skipped.
func HashFiles(patterns []string, root string) map[string]string {
	hashes := make(map[string]string)
	if len(patterns) == 0 {
		return hashes
	}

	rootAbs, _ := filepath.Abs(root)

	filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip hidden dirs (e.g. .git) except .axiom (test definitions)
			if strings.HasPrefix(d.Name(), ".") && path != rootAbs {
				if d.Name() == ".axiom" {
					return nil // continue into .axiom
				}
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return nil
		}

		for _, pattern := range patterns {
			if Match(pattern, rel) {
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				sum := sha256.Sum256(data)
				hashes[rel] = hex.EncodeToString(sum[:])
				break // a file only needs one pattern to match
			}
		}
		return nil
	})

	return hashes
}
