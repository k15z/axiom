package config

import (
	"os"
	"testing"
)

func TestDefault(t *testing.T) {
	d := Default()

	if d.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Model = %q, want %q", d.Model, "claude-haiku-4-5-20251001")
	}
	if d.TestDir != ".axiom/" {
		t.Errorf("TestDir = %q, want %q", d.TestDir, ".axiom/")
	}
	if !d.Cache.Enabled {
		t.Error("Cache.Enabled should be true")
	}
	if d.Cache.Dir != ".axiom/.cache/" {
		t.Errorf("Cache.Dir = %q, want %q", d.Cache.Dir, ".axiom/.cache/")
	}
	if d.Agent.MaxIterations != 30 {
		t.Errorf("Agent.MaxIterations = %d, want 30", d.Agent.MaxIterations)
	}
	if d.Agent.MaxTokens != 10000 {
		t.Errorf("Agent.MaxTokens = %d, want 10000", d.Agent.MaxTokens)
	}
	if d.Agent.Timeout != 0 {
		t.Errorf("Agent.Timeout = %d, want 0", d.Agent.Timeout)
	}
}

func TestLoadDotEnv_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	// Should not panic or error — silently a no-op
	loadDotEnv()
}

func TestLoadDotEnv_BasicKeyValue(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	key := "AXIOM_TEST_DOTENV_BASIC"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	if err := os.WriteFile(".env", []byte(key+"=hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadDotEnv()
	if got := os.Getenv(key); got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestLoadDotEnv_DoubleQuotedValue(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	key := "AXIOM_TEST_DOTENV_DQUOTE"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	if err := os.WriteFile(".env", []byte(key+`="quoted value"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadDotEnv()
	if got := os.Getenv(key); got != "quoted value" {
		t.Errorf("expected %q, got %q", "quoted value", got)
	}
}

func TestLoadDotEnv_SingleQuotedValue(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	key := "AXIOM_TEST_DOTENV_SQUOTE"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	if err := os.WriteFile(".env", []byte(key+"='single quoted'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadDotEnv()
	if got := os.Getenv(key); got != "single quoted" {
		t.Errorf("expected %q, got %q", "single quoted", got)
	}
}

func TestLoadDotEnv_CommentsSkipped(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	key := "AXIOM_TEST_DOTENV_COMMENT"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	content := "# this is a comment\n" + key + "=real\n"
	if err := os.WriteFile(".env", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	loadDotEnv()
	if got := os.Getenv(key); got != "real" {
		t.Errorf("expected %q after comment line, got %q", "real", got)
	}
}

func TestLoadDotEnv_BlankLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	key := "AXIOM_TEST_DOTENV_BLANK"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	content := "\n\n" + key + "=value\n\n"
	if err := os.WriteFile(".env", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	loadDotEnv()
	if got := os.Getenv(key); got != "value" {
		t.Errorf("expected %q, got %q", "value", got)
	}
}

func TestLoadDotEnv_LineWithoutEquals(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	key := "AXIOM_TEST_DOTENV_NOEQ"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	// Line without '=' is skipped; the valid line below it should still be set
	content := "NOEQUALS\n" + key + "=ok\n"
	if err := os.WriteFile(".env", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	loadDotEnv()
	if got := os.Getenv(key); got != "ok" {
		t.Errorf("expected %q, got %q", "ok", got)
	}
}

func TestLoadDotEnv_DoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	key := "AXIOM_TEST_DOTENV_NOOVERWRITE"
	t.Setenv(key, "original")

	if err := os.WriteFile(".env", []byte(key+"=override\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadDotEnv()
	if got := os.Getenv(key); got != "original" {
		t.Errorf("expected existing value %q to be preserved, got %q", "original", got)
	}
}
