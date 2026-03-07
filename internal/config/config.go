package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/k15z/axiom/internal/provider"
	"gopkg.in/yaml.v3"
)

type CacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"`
}

type AgentConfig struct {
	MaxIterations int `yaml:"max_iterations"`
	MaxTokens     int `yaml:"max_tokens"`
	Timeout       int `yaml:"timeout"`      // per-test timeout in seconds; 0 means no timeout
	ToolTimeout   int `yaml:"tool_timeout"` // per-tool timeout in seconds; 0 means no timeout
}

type Config struct {
	Model    string      `yaml:"model"`
	Provider string      `yaml:"provider"` // "anthropic", "openai", "gemini" (inferred from model if omitted)
	BaseURL  string      `yaml:"base_url"` // custom API endpoint for OpenAI-compatible providers
	TestDir  string      `yaml:"test_dir"`
	Cache    CacheConfig `yaml:"cache"`
	Agent    AgentConfig `yaml:"agent"`
	APIKey   string      `yaml:"-"`
}

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

func Load(testDir string) (Config, error) {
	cfg, err := LoadWithoutKey(testDir)
	if err != nil {
		return cfg, err
	}
	return cfg, cfg.ResolveKey()
}

// LoadForDryRun loads config without requiring an API key.
// Used by --dry-run mode which performs discovery and cache checks only.
func LoadForDryRun(testDir string) (Config, error) {
	return LoadWithoutKey(testDir)
}

// LoadWithoutKey loads config from axiom.yml and .env but does not resolve the
// provider or load the API key. Call ResolveKey() after applying CLI flag
// overrides (e.g. --provider, --model).
func LoadWithoutKey(testDir string) (Config, error) {
	// Load .env before anything else so vars are available via os.Getenv
	loadDotEnv()

	cfg := Default()

	// Load axiom.yml from the project root if it exists
	if data, err := os.ReadFile("axiom.yml"); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parsing axiom.yml: %w", err)
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
	}

	if testDir != "" {
		cfg.TestDir = testDir
	}

	return cfg, nil
}

// ResolveKey resolves the provider from the model name and loads the
// appropriate API key from the environment. Call this after applying any
// CLI flag overrides to Provider and Model.
func (cfg *Config) ResolveKey() error {
	resolved, err := provider.ResolveProvider(cfg.Provider, cfg.Model)
	if err != nil {
		return err
	}
	cfg.Provider = resolved

	cfg.APIKey, err = loadAPIKeyForProvider(resolved)
	if err != nil {
		return err
	}

	return nil
}

// LoadMinimal loads config from .env and axiom.yml without requiring
// ANTHROPIC_API_KEY. Use this for commands that don't call the API (e.g.
// validate, list).
func LoadMinimal(testDir string) Config {
	loadDotEnv()

	cfg := Default()

	if data, err := os.ReadFile("axiom.yml"); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err == nil {
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
		}
	}

	if testDir != "" {
		cfg.TestDir = testDir
	}

	return cfg
}

// LoadAPIKey loads the API key from the environment or .env file.
// Unlike Load(), it does not require axiom.yml to exist.
func LoadAPIKey() (string, error) {
	loadDotEnv()
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY is not set (set it in the environment or a .env file)")
	}
	return key, nil
}

// loadAPIKeyForProvider returns the API key for the given provider.
func loadAPIKeyForProvider(prov string) (string, error) {
	var envVar string
	switch prov {
	case "anthropic":
		envVar = "ANTHROPIC_API_KEY"
	case "openai":
		envVar = "OPENAI_API_KEY"
	case "gemini":
		envVar = "GEMINI_API_KEY"
	default:
		return "", fmt.Errorf("unknown provider %q", prov)
	}
	key := os.Getenv(envVar)
	if key == "" {
		return "", fmt.Errorf("%s is not set (set it in the environment or a .env file)", envVar)
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
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
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
