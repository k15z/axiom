package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/k15z/axiom/internal/provider"
	"gopkg.in/yaml.v3"
)

// CacheConfig controls result caching behaviour.
type CacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"`
}

// AgentConfig tunes the LLM agent loop parameters.
type AgentConfig struct {
	MaxIterations int `yaml:"max_iterations"`
	MaxTokens     int `yaml:"max_tokens"`
	Timeout       int `yaml:"timeout"`      // per-test timeout in seconds; 0 means no timeout
	ToolTimeout   int `yaml:"tool_timeout"` // per-tool timeout in seconds; 0 means no timeout
}

// Config is the fully resolved axiom configuration for a single run.
type Config struct {
	Model    string      `yaml:"model"`
	Provider string      `yaml:"provider"` // "anthropic", "openai", "gemini" (inferred from model if omitted)
	BaseURL  string      `yaml:"base_url"` // custom API endpoint for OpenAI-compatible providers
	TestDir  string      `yaml:"test_dir"`
	Cache    CacheConfig `yaml:"cache"`
	Agent    AgentConfig `yaml:"agent"`
	APIKey   string      `yaml:"-"`
}

// LoadOpts controls which steps Load performs.
type LoadOpts struct {
	TestDir    string // override test_dir from config; empty = use config value
	ResolveKey bool   // if true, resolve provider and load API key from environment
}

// Default returns the baseline configuration used when axiom.yml is absent or
// when fields are omitted.
func Default() Config {
	return Config{
		Model:   "claude-haiku-4-5-20251001",
		TestDir: ".axiom/",
		Cache: CacheConfig{
			Enabled: true,
			Dir:     ".axiom/.cache/",
		},
		Agent: AgentConfig{
			MaxIterations: 30,
			MaxTokens:     10000,
			Timeout:       0,
			ToolTimeout:   30,
		},
	}
}

// Load reads .env and axiom.yml, applies defaults, and optionally resolves the
// API key. This is the single entry point for all config loading.
func Load(opts LoadOpts) (Config, error) {
	loadDotEnv()

	cfg, err := loadYAML()
	if err != nil {
		return cfg, err
	}

	if opts.TestDir != "" {
		cfg.TestDir = opts.TestDir
	}

	if opts.ResolveKey {
		if err := cfg.ResolveKey(); err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}


// ResolveKey resolves the provider from the model name and loads the
// appropriate API key from the environment. Call this after applying any
// CLI flag overrides to Provider and Model.
func (cfg *Config) ResolveKey() error {
	resolved, err := provider.ResolveProvider(cfg.Provider, cfg.Model)
	if err != nil {
		return fmt.Errorf("resolving provider: %w", err)
	}
	cfg.Provider = resolved

	cfg.APIKey, err = loadAPIKeyForProvider(resolved)
	if err != nil {
		return fmt.Errorf("loading API key: %w", err)
	}

	return nil
}

// loadYAML reads axiom.yml and applies defaults for zero values.
func loadYAML() (Config, error) {
	cfg := Default()

	data, err := os.ReadFile("axiom.yml")
	if err != nil {
		return cfg, nil // no config file is fine
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing axiom.yml: %w — run `axiom doctor` to diagnose config issues", err)
	}

	// Re-apply defaults for zero values that weren't explicitly set
	d := Default()
	if cfg.Model == "" {
		cfg.Model = d.Model
	}
	if cfg.TestDir == "" {
		cfg.TestDir = d.TestDir
	}
	if cfg.Cache.Dir == "" {
		cfg.Cache.Dir = d.Cache.Dir
	}
	if cfg.Agent.MaxIterations == 0 {
		cfg.Agent.MaxIterations = d.Agent.MaxIterations
	}
	if cfg.Agent.MaxTokens == 0 {
		cfg.Agent.MaxTokens = d.Agent.MaxTokens
	}
	if cfg.Agent.ToolTimeout == 0 {
		cfg.Agent.ToolTimeout = d.Agent.ToolTimeout
	}

	return cfg, nil
}

// LoadAPIKeyForProvider returns the API key for the given provider.
// It loads .env first, then checks the environment for the provider-specific key.
func LoadAPIKeyForProvider(prov string) (string, error) {
	loadDotEnv()
	return loadAPIKeyForProvider(prov)
}

// loadAPIKeyForProvider returns the API key for the given provider.
func loadAPIKeyForProvider(prov string) (string, error) {
	var envVar, signupHint string
	switch prov {
	case "anthropic":
		envVar = "ANTHROPIC_API_KEY"
		signupHint = "get one at console.anthropic.com/settings/keys"
	case "openai":
		envVar = "OPENAI_API_KEY"
		signupHint = "get one at platform.openai.com/api-keys"
	case "gemini":
		envVar = "GEMINI_API_KEY"
		signupHint = "get one at aistudio.google.com/app/apikey"
	default:
		return "", fmt.Errorf("unknown provider %q — supported providers: anthropic, openai, gemini", prov)
	}
	key := os.Getenv(envVar)
	if key == "" {
		return "", fmt.Errorf("%s is not set — %s and add it to .env as %s=your-key", envVar, signupHint, envVar)
	}
	return key, nil
}

// loadDotEnv reads a .env file from the current directory and sets any
// environment variables that are not already set. Silently ignores missing files.
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		key = strings.TrimPrefix(key, "export ")
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
