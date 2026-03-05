package glob

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		// Exact matches
		{"exact file", "foo.go", "foo.go", true},
		{"exact path", "src/main.go", "src/main.go", true},
		{"exact mismatch", "foo.go", "bar.go", false},

		// Single-segment wildcards
		{"star matches file", "*.go", "main.go", true},
		{"star no match ext", "*.go", "main.rs", false},
		{"star in dir", "src/*.go", "src/main.go", true},
		{"star wrong dir", "src/*.go", "pkg/main.go", false},
		{"question mark", "?.go", "a.go", true},
		{"question mark too long", "?.go", "ab.go", false},

		// Character class
		{"char class match", "[abc].go", "a.go", true},
		{"char class no match", "[abc].go", "d.go", false},

		// ** at various positions
		{"doublestar prefix", "**/*.go", "main.go", true},
		{"doublestar prefix nested", "**/*.go", "src/pkg/main.go", true},
		{"doublestar prefix wrong ext", "**/*.go", "src/main.rs", false},
		{"doublestar suffix", "src/**", "src/main.go", true},
		{"doublestar suffix nested", "src/**", "src/a/b/c.go", true},
		{"doublestar middle", "src/**/main.go", "src/main.go", true},
		{"doublestar middle deep", "src/**/main.go", "src/a/b/main.go", true},
		{"doublestar middle mismatch", "src/**/main.go", "src/a/b/other.go", false},
		{"doublestar alone", "**", "any/path/at/all.txt", true},

		// ** matching zero segments
		{"doublestar zero segments", "src/**/main.go", "src/main.go", true},

		// Empty pattern and path
		{"both empty", "", "", true},
		{"empty pattern nonempty path", "", "foo", false},
		{"nonempty pattern empty path", "foo", "", false},

		// Pattern longer than path
		{"pattern longer", "a/b/c", "a/b", false},
		{"path longer", "a/b", "a/b/c", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
