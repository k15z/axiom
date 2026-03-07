package watch

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/k15z/axiom/internal/discovery"
)

func TestMatchTests(t *testing.T) {
	tests := []discovery.Test{
		{Name: "go-test", On: []string{"**/*.go"}},
		{Name: "md-test", On: []string{"**/*.md"}},
		{Name: "no-glob", On: nil},
		{Name: "multi-glob", On: []string{"src/*.go", "pkg/**/*.go"}},
		{Name: "tagged", On: []string{"**/*.go"}, Tags: []string{"security"}},
	}

	cases := []struct {
		name    string
		changed []string
		filter  string
		tag     string
		want    []string
	}{
		{
			"go file matches go-test, no-glob, and tagged",
			[]string{"main.go"},
			"", "",
			[]string{"go-test", "no-glob", "tagged"},
		},
		{
			"md file matches md-test and no-glob",
			[]string{"README.md"},
			"", "",
			[]string{"md-test", "no-glob"},
		},
		{
			"nested go file",
			[]string{"internal/foo/bar.go"},
			"", "",
			[]string{"go-test", "no-glob", "tagged"},
		},
		{
			"src go file matches multi-glob too",
			[]string{"src/main.go"},
			"", "",
			[]string{"go-test", "no-glob", "multi-glob", "tagged"},
		},
		{
			"pkg nested go file matches multi-glob",
			[]string{"pkg/sub/util.go"},
			"", "",
			[]string{"go-test", "no-glob", "multi-glob", "tagged"},
		},
		{
			"unrelated file only matches no-glob",
			[]string{"data.csv"},
			"", "",
			[]string{"no-glob"},
		},
		{
			"filter limits results",
			[]string{"main.go"},
			"go-test", "",
			[]string{"go-test"},
		},
		{
			"filter excludes all",
			[]string{"main.go"},
			"nonexistent", "",
			nil,
		},
		{
			"tag filter",
			[]string{"main.go"},
			"", "security",
			[]string{"tagged"},
		},
		{
			"filter and tag combined",
			[]string{"main.go"},
			"tagged", "security",
			[]string{"tagged"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchTests(tests, tt.changed, tt.filter, tt.tag)
			gotNames := make([]string, len(got))
			for i, g := range got {
				gotNames[i] = g.Name
			}
			if len(got) != len(tt.want) {
				t.Fatalf("MatchTests() returned %v, want %v", gotNames, tt.want)
			}
			for i, w := range tt.want {
				if gotNames[i] != w {
					t.Errorf("MatchTests()[%d] = %q, want %q", i, gotNames[i], w)
				}
			}
		})
	}
}

func TestTestMatchesAny(t *testing.T) {
	cases := []struct {
		name    string
		test    discovery.Test
		changed []string
		want    bool
	}{
		{
			"single glob match",
			discovery.Test{On: []string{"**/*.go"}},
			[]string{"main.go"},
			true,
		},
		{
			"no match",
			discovery.Test{On: []string{"**/*.go"}},
			[]string{"README.md"},
			false,
		},
		{
			"multiple globs, second matches",
			discovery.Test{On: []string{"**/*.py", "**/*.go"}},
			[]string{"main.go"},
			true,
		},
		{
			"multiple changed files, one matches",
			discovery.Test{On: []string{"src/**/*.go"}},
			[]string{"README.md", "src/main.go"},
			true,
		},
		{
			"empty on globs",
			discovery.Test{On: nil},
			[]string{"main.go"},
			false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := testMatchesAny(tt.test, tt.changed)
			if got != tt.want {
				t.Errorf("testMatchesAny() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUnderCacheDir(t *testing.T) {
	root := t.TempDir()

	cases := []struct {
		name     string
		rel      string
		cacheDir string
		want     bool
	}{
		{
			"file in cache dir",
			filepath.Join(".axiom", ".cache", "results.json"),
			filepath.Join(root, ".axiom", ".cache"),
			true,
		},
		{
			"cache dir itself",
			filepath.Join(".axiom", ".cache"),
			filepath.Join(root, ".axiom", ".cache"),
			true,
		},
		{
			"file outside cache dir",
			filepath.Join("src", "main.go"),
			filepath.Join(root, ".axiom", ".cache"),
			false,
		},
		{
			"file in axiom dir but not cache",
			filepath.Join(".axiom", "tests.yml"),
			filepath.Join(root, ".axiom", ".cache"),
			false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnderCacheDir(tt.rel, tt.cacheDir, root)
			if got != tt.want {
				t.Errorf("isUnderCacheDir(%q) = %v, want %v", tt.rel, got, tt.want)
			}
		})
	}
}

func TestDebounceDelay(t *testing.T) {
	// Verify the debounce constant is reasonable (between 100ms and 2s)
	if debounceDelay < 100*time.Millisecond || debounceDelay > 2*time.Second {
		t.Errorf("debounceDelay = %v, want between 100ms and 2s", debounceDelay)
	}
}
