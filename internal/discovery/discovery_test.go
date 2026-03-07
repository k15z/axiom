package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func writeYAML(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscover_Ordering(t *testing.T) {
	dir := t.TempDir()

	// Files named so lexicographic order is: a.yml, b.yml, c.yaml
	writeYAML(t, dir, "c.yaml", `
second_in_c:
  condition: "c second"
first_in_c:
  condition: "c first"
`)
	writeYAML(t, dir, "a.yml", `
only_in_a:
  condition: "a only"
`)
	writeYAML(t, dir, "b.yml", `
beta:
  condition: "b beta"
alpha:
  condition: "b alpha"
`)

	tests, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	// Expected order: a.yml tests, then b.yml tests (YAML key order), then c.yaml tests
	wantNames := []string{"only_in_a", "beta", "alpha", "second_in_c", "first_in_c"}
	if len(tests) != len(wantNames) {
		t.Fatalf("got %d tests, want %d", len(tests), len(wantNames))
	}
	for i, want := range wantNames {
		if tests[i].Name != want {
			t.Errorf("tests[%d].Name = %q, want %q", i, tests[i].Name, want)
		}
	}
}

func TestDiscover_HiddenDirSkipped(t *testing.T) {
	dir := t.TempDir()

	writeYAML(t, dir, "visible.yml", `
vis:
  condition: "visible"
`)
	writeYAML(t, dir, ".hidden/secret.yml", `
secret:
  condition: "hidden"
`)

	tests, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(tests) != 1 || tests[0].Name != "vis" {
		t.Errorf("expected only 'vis' test, got %v", tests)
	}
}

func TestDiscover_MissingCondition(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad.yml", `
no_condition:
  on: ["*.go"]
`)

	_, err := Discover(dir)
	if err == nil {
		t.Fatal("expected error for missing condition")
	}
}

func TestDiscover_NonexistentDir(t *testing.T) {
	_, err := Discover("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestDiscover_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "a.yml", `
dup:
  condition: "first definition"
`)
	writeYAML(t, dir, "b.yml", `
dup:
  condition: "second definition"
`)

	_, err := Discover(dir)
	if err == nil {
		t.Fatal("expected error for duplicate test name")
	}
}

func TestDiscover_OnField(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "with_on.yml", `
tracked:
  on: ["src/*.go", "**/*.md"]
  condition: "check tracked files"
`)

	tests, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(tests) != 1 {
		t.Fatalf("got %d tests, want 1", len(tests))
	}
	if len(tests[0].On) != 2 {
		t.Errorf("tests[0].On = %v, want 2 globs", tests[0].On)
	}
}
