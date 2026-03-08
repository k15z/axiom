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

func TestLoad_NoAxiomYml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cfg, err := Load(LoadOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return defaults when no axiom.yml exists
	d := Default()
	if cfg.Model != d.Model {
		t.Errorf("Model = %q, want default %q", cfg.Model, d.Model)
	}
	if cfg.TestDir != d.TestDir {
		t.Errorf("TestDir = %q, want default %q", cfg.TestDir, d.TestDir)
	}
	if cfg.APIKey != "" {
		t.Error("APIKey should be empty without ResolveKey")
	}
}

func TestLoad_PartialAxiomYml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// axiom.yml only sets model — other fields should get defaults
	if err := os.WriteFile("axiom.yml", []byte("model: gpt-4o\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4o")
	}
	d := Default()
	if cfg.TestDir != d.TestDir {
		t.Errorf("TestDir should default to %q, got %q", d.TestDir, cfg.TestDir)
	}
	if cfg.Agent.MaxIterations != d.Agent.MaxIterations {
		t.Errorf("MaxIterations should default to %d, got %d", d.Agent.MaxIterations, cfg.Agent.MaxIterations)
	}
	if cfg.Agent.MaxTokens != d.Agent.MaxTokens {
		t.Errorf("MaxTokens should default to %d, got %d", d.Agent.MaxTokens, cfg.Agent.MaxTokens)
	}
}

func TestLoad_InvalidYaml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile("axiom.yml", []byte(":\n  :\n    bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(LoadOpts{})
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestResolveKey_Anthropic(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("ANTHROPIC_API_KEY", "test-key-123")

	cfg := Default()
	if err := cfg.ResolveKey(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "anthropic")
	}
	if cfg.APIKey != "test-key-123" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key-123")
	}
}

func TestResolveKey_OpenAI(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("OPENAI_API_KEY", "sk-test-456")

	cfg := Default()
	cfg.Model = "gpt-4o"
	if err := cfg.ResolveKey(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "openai")
	}
	if cfg.APIKey != "sk-test-456" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "sk-test-456")
	}
}

func TestResolveKey_Gemini(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("GEMINI_API_KEY", "gem-test-789")

	cfg := Default()
	cfg.Model = "gemini-2.0-flash"
	if err := cfg.ResolveKey(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "gemini" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "gemini")
	}
	if cfg.APIKey != "gem-test-789" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "gem-test-789")
	}
}

func TestResolveKey_ExplicitProviderOverridesModel(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("OPENAI_API_KEY", "sk-override")

	cfg := Default()
	cfg.Provider = "openai"
	// Model still looks like Anthropic, but explicit provider wins
	if err := cfg.ResolveKey(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "openai")
	}
}

func TestResolveKey_MissingAPIKey(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Unset all API keys
	t.Setenv("ANTHROPIC_API_KEY", "")
	os.Unsetenv("ANTHROPIC_API_KEY")

	cfg := Default()
	err := cfg.ResolveKey()
	if err == nil {
		t.Error("expected error for missing API key, got nil")
	}
}

func TestLoad_Minimal_NoAPIKeyRequired(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Unset all API keys — Load without ResolveKey should not error
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")

	cfg, err := Load(LoadOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d := Default()
	if cfg.Model != d.Model {
		t.Errorf("Model = %q, want default %q", cfg.Model, d.Model)
	}
}

func TestLoad_Minimal_ReadsAxiomYml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	yml := "model: gemini-2.0-flash\ntest_dir: my-tests/\n"
	if err := os.WriteFile("axiom.yml", []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "gemini-2.0-flash" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gemini-2.0-flash")
	}
	if cfg.TestDir != "my-tests/" {
		t.Errorf("TestDir = %q, want %q", cfg.TestDir, "my-tests/")
	}
}

func TestLoadAPIKeyForProvider_AllProviders(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	tests := []struct {
		provider string
		envVar   string
		value    string
	}{
		{"anthropic", "ANTHROPIC_API_KEY", "ak-123"},
		{"openai", "OPENAI_API_KEY", "sk-456"},
		{"gemini", "GEMINI_API_KEY", "gk-789"},
	}

	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			t.Setenv(tc.envVar, tc.value)
			key, err := LoadAPIKeyForProvider(tc.provider)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != tc.value {
				t.Errorf("key = %q, want %q", key, tc.value)
			}
		})
	}
}

func TestLoadAPIKeyForProvider_UnknownProvider(t *testing.T) {
	_, err := loadAPIKeyForProvider("azure")
	if err == nil {
		t.Error("expected error for unknown provider, got nil")
	}
}

func TestLoad_DotEnvIntegration(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Write .env with a custom key and axiom.yml referencing it
	envContent := "ANTHROPIC_API_KEY=from-dotenv\n"
	if err := os.WriteFile(".env", []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ensure the env var isn't already set
	os.Unsetenv("ANTHROPIC_API_KEY")

	cfg, err := Load(LoadOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Load without ResolveKey loads .env but doesn't resolve the key
	if cfg.APIKey != "" {
		t.Errorf("APIKey should be empty before ResolveKey, got %q", cfg.APIKey)
	}

	// Now resolve — should pick up from .env
	if err := cfg.ResolveKey(); err != nil {
		t.Fatalf("ResolveKey error: %v", err)
	}
	if cfg.APIKey != "from-dotenv" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "from-dotenv")
	}
}

func TestLoad_CacheEnabledExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Explicitly setting cache.enabled: false should not be overridden by defaults
	yml := "cache:\n  enabled: false\n"
	if err := os.WriteFile("axiom.yml", []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cache.Enabled {
		t.Error("Cache.Enabled should be false when explicitly set")
	}
}

func TestLoad_EmptyAxiomYml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile("axiom.yml", []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All fields should get defaults
	d := Default()
	if cfg.Model != d.Model {
		t.Errorf("Model = %q, want default %q", cfg.Model, d.Model)
	}
	if cfg.Agent.MaxIterations != d.Agent.MaxIterations {
		t.Errorf("MaxIterations = %d, want default %d", cfg.Agent.MaxIterations, d.Agent.MaxIterations)
	}
	if cfg.Agent.ToolTimeout != d.Agent.ToolTimeout {
		t.Errorf("ToolTimeout = %d, want default %d", cfg.Agent.ToolTimeout, d.Agent.ToolTimeout)
	}
}

func TestLoadDotEnv_ValueWithEquals(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	key := "AXIOM_TEST_DOTENV_EQUALS"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	// Value contains '=' which should be preserved
	if err := os.WriteFile(".env", []byte(key+"=abc=def=ghi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadDotEnv()
	if got := os.Getenv(key); got != "abc=def=ghi" {
		t.Errorf("expected %q, got %q", "abc=def=ghi", got)
	}
}

func TestLoad_FullAxiomYml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	yml := `model: gpt-4o
provider: openai
base_url: https://custom.api.com/v1
test_dir: tests/
cache:
  enabled: false
  dir: .cache/
agent:
  max_iterations: 50
  max_tokens: 20000
  timeout: 120
  tool_timeout: 60
`
	if err := os.WriteFile("axiom.yml", []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4o")
	}
	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "openai")
	}
	if cfg.BaseURL != "https://custom.api.com/v1" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://custom.api.com/v1")
	}
	if cfg.TestDir != "tests/" {
		t.Errorf("TestDir = %q, want %q", cfg.TestDir, "tests/")
	}
	if cfg.Cache.Enabled {
		t.Error("Cache.Enabled should be false")
	}
	if cfg.Cache.Dir != ".cache/" {
		t.Errorf("Cache.Dir = %q, want %q", cfg.Cache.Dir, ".cache/")
	}
	if cfg.Agent.MaxIterations != 50 {
		t.Errorf("MaxIterations = %d, want 50", cfg.Agent.MaxIterations)
	}
	if cfg.Agent.MaxTokens != 20000 {
		t.Errorf("MaxTokens = %d, want 20000", cfg.Agent.MaxTokens)
	}
	if cfg.Agent.Timeout != 120 {
		t.Errorf("Timeout = %d, want 120", cfg.Agent.Timeout)
	}
	if cfg.Agent.ToolTimeout != 60 {
		t.Errorf("ToolTimeout = %d, want 60", cfg.Agent.ToolTimeout)
	}
}

func TestLoad_Minimal_InvalidYaml(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile("axiom.yml", []byte(":\n  :\n    bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load previously swallowed YAML errors silently via LoadMinimal. After consolidation,
	// it should surface them just like Load does.
	_, err := Load(LoadOpts{})
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoad_WithResolveKey(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("ANTHROPIC_API_KEY", "test-key-load")

	cfg, err := Load(LoadOpts{ResolveKey: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "anthropic")
	}
	if cfg.APIKey != "test-key-load" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key-load")
	}
}

func TestLoad_WithoutResolveKey(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cfg, err := Load(LoadOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey should be empty without ResolveKey, got %q", cfg.APIKey)
	}
	d := Default()
	if cfg.Model != d.Model {
		t.Errorf("Model = %q, want default %q", cfg.Model, d.Model)
	}
}

func TestLoad_TestDirOverride(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cfg, err := Load(LoadOpts{TestDir: "custom/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TestDir != "custom/" {
		t.Errorf("TestDir = %q, want %q", cfg.TestDir, "custom/")
	}
}
