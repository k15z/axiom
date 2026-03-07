package watch

import (
	"testing"
	"time"

	"github.com/k15z/axiom/internal/discovery"
)

func TestDiff(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(time.Second)

	tests := []struct {
		name    string
		old     map[string]time.Time
		current map[string]time.Time
		want    int
	}{
		{"no changes", map[string]time.Time{"a.go": t1}, map[string]time.Time{"a.go": t1}, 0},
		{"modified file", map[string]time.Time{"a.go": t1}, map[string]time.Time{"a.go": t2}, 1},
		{"new file", map[string]time.Time{"a.go": t1}, map[string]time.Time{"a.go": t1, "b.go": t2}, 1},
		{"deleted file", map[string]time.Time{"a.go": t1, "b.go": t1}, map[string]time.Time{"a.go": t1}, 1},
		{"multiple changes", map[string]time.Time{"a.go": t1, "b.go": t1}, map[string]time.Time{"a.go": t2, "c.go": t2}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diff(tt.old, tt.current)
			if len(got) != tt.want {
				t.Errorf("diff() returned %d changed files, want %d: %v", len(got), tt.want, got)
			}
		})
	}
}

func TestMatchTests(t *testing.T) {
	tests := []discovery.Test{
		{Name: "go-test", On: []string{"**/*.go"}},
		{Name: "md-test", On: []string{"**/*.md"}},
		{Name: "no-glob", On: nil},
		{Name: "multi-glob", On: []string{"src/*.go", "pkg/**/*.go"}},
	}

	cases := []struct {
		name    string
		changed []string
		filter  string
		want    []string
	}{
		{
			"go file change matches go-test and no-glob",
			[]string{"main.go"},
			"",
			[]string{"go-test", "no-glob"},
		},
		{
			"md file change matches md-test and no-glob",
			[]string{"README.md"},
			"",
			[]string{"md-test", "no-glob"},
		},
		{
			"nested go file matches go-test",
			[]string{"internal/foo/bar.go"},
			"",
			[]string{"go-test", "no-glob"},
		},
		{
			"src go file matches multi-glob",
			[]string{"src/main.go"},
			"",
			[]string{"go-test", "no-glob", "multi-glob"},
		},
		{
			"pkg nested go file matches multi-glob",
			[]string{"pkg/sub/util.go"},
			"",
			[]string{"go-test", "no-glob", "multi-glob"},
		},
		{
			"unrelated file only matches no-glob",
			[]string{"data.csv"},
			"",
			[]string{"no-glob"},
		},
		{
			"filter limits results",
			[]string{"main.go"},
			"go-test",
			[]string{"go-test"},
		},
		{
			"filter excludes all",
			[]string{"main.go"},
			"nonexistent",
			nil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := matchTests(tests, tt.changed, tt.filter)
			gotNames := make([]string, len(got))
			for i, g := range got {
				gotNames[i] = g.Name
			}
			if len(got) != len(tt.want) {
				t.Fatalf("matchTests() returned %v, want %v", gotNames, tt.want)
			}
			for i, w := range tt.want {
				if gotNames[i] != w {
					t.Errorf("matchTests()[%d] = %q, want %q", i, gotNames[i], w)
				}
			}
		})
	}
}
