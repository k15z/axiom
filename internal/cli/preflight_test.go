package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k15z/axiom/internal/discovery"
)

func TestPreflightValidate_ValidGlobs(t *testing.T) {
	tests := []discovery.Test{
		{
			Name:       "valid_test",
			Condition:  "check that all go files have tests",
			SourceFile: "tests.yml",
			On:         []string{"**/*.go", "src/*.ts", "*.md"},
		},
	}
	if err := preflightValidate(tests); err != nil {
		t.Errorf("expected no error for valid globs, got: %v", err)
	}
}

func TestPreflightValidate_InvalidGlobSyntax(t *testing.T) {
	tests := []discovery.Test{
		{
			Name:       "bad_glob_test",
			Condition:  "check that all go files have tests",
			SourceFile: "tests.yml",
			On:         []string{"src/[invalid"},
		},
	}
	err := preflightValidate(tests)
	if err == nil {
		t.Fatal("expected error for invalid glob syntax, got nil")
	}
	if !strings.Contains(err.Error(), "pre-flight validation failed") {
		t.Errorf("error message should mention pre-flight validation, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bad_glob_test") {
		t.Errorf("error message should name the failing test, got: %v", err)
	}
	if !strings.Contains(err.Error(), "src/[invalid") {
		t.Errorf("error message should include the bad pattern, got: %v", err)
	}
}

func TestPreflightValidate_MultipleInvalidGlobs(t *testing.T) {
	tests := []discovery.Test{
		{
			Name:       "test_a",
			Condition:  "check that all go files have tests",
			SourceFile: "a.yml",
			On:         []string{"src/[bad"},
		},
		{
			Name:       "test_b",
			Condition:  "check that all go files have tests",
			SourceFile: "b.yml",
			On:         []string{"*.go", "lib/[also_bad"},
		},
	}
	err := preflightValidate(tests)
	if err == nil {
		t.Fatal("expected error for multiple invalid globs, got nil")
	}
	// Both tests should be reported
	if !strings.Contains(err.Error(), "test_a") {
		t.Errorf("error should mention test_a, got: %v", err)
	}
	if !strings.Contains(err.Error(), "test_b") {
		t.Errorf("error should mention test_b, got: %v", err)
	}
}

func TestPreflightValidate_NoOnPatterns(t *testing.T) {
	// Tests without 'on' patterns are valid for preflight (they just can't cache).
	tests := []discovery.Test{
		{
			Name:       "no_on_test",
			Condition:  "check that all go files have tests",
			SourceFile: "tests.yml",
			On:         nil,
		},
	}
	if err := preflightValidate(tests); err != nil {
		t.Errorf("expected no error for missing 'on' patterns, got: %v", err)
	}
}

func TestPreflightValidate_EmptyList(t *testing.T) {
	if err := preflightValidate(nil); err != nil {
		t.Errorf("expected no error for empty test list, got: %v", err)
	}
}

func TestPreflightValidate_DoubleStarValid(t *testing.T) {
	// ** glob segments are explicitly allowed (they skip filepath.Match validation).
	tests := []discovery.Test{
		{
			Name:       "star_star_test",
			Condition:  "check that all go files have tests",
			SourceFile: "tests.yml",
			On:         []string{"**/*.go", "**/internal/**/*.go"},
		},
	}
	if err := preflightValidate(tests); err != nil {
		t.Errorf("expected no error for ** globs, got: %v", err)
	}
}

// TestRunCmd_PreflightBlocksInvalidGlob verifies that the run command returns a
// SetupError (exit code 2) when a test YAML contains an invalid glob in its
// 'on:' field, and does so before making any API calls.
func TestRunCmd_PreflightBlocksInvalidGlob(t *testing.T) {
	// Create a temp test directory with a YAML file that has a bad glob.
	testDir := t.TempDir()
	yamlContent := `
bad_glob_test:
  on: ["src/[unclosed"]
  condition: "this condition is long enough to pass the length check"
`
	if err := os.WriteFile(filepath.Join(testDir, "tests.yml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set a fake API key so ResolveKey() doesn't fail first.
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-used")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"run", "--dir", testDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from run command with invalid glob, got nil")
	}

	var se *SetupError
	if !errors.As(err, &se) {
		t.Errorf("expected SetupError (exit code 2), got %T: %v", err, err)
	}

	if !strings.Contains(err.Error(), "pre-flight validation failed") {
		t.Errorf("error should mention pre-flight validation, got: %v", err)
	}
}
