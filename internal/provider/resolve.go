package provider

import (
	"fmt"
	"strings"
)

// ResolveProvider determines the provider name from an explicit provider string
// or by inferring from the model name.
func ResolveProvider(explicit, model string) (string, error) {
	if explicit != "" {
		switch explicit {
		case "anthropic", "openai", "gemini":
			return explicit, nil
		default:
			return "", fmt.Errorf("unknown provider %q (supported: anthropic, openai, gemini)", explicit)
		}
	}
	return InferProvider(model)
}

// InferProvider guesses the provider from the model name.
func InferProvider(model string) (string, error) {
	lower := strings.ToLower(model)
	switch {
	case strings.HasPrefix(lower, "claude-") || strings.HasPrefix(lower, "anthropic/"):
		return "anthropic", nil
	case strings.HasPrefix(lower, "gpt-") ||
		strings.HasPrefix(lower, "o1-") ||
		strings.HasPrefix(lower, "o3-") ||
		strings.HasPrefix(lower, "o4-") ||
		strings.HasPrefix(lower, "openai/"):
		return "openai", nil
	case strings.HasPrefix(lower, "gemini-") || strings.HasPrefix(lower, "google/"):
		return "gemini", nil
	}
	return "", fmt.Errorf("cannot infer provider from model %q; set 'provider' in axiom.yml (anthropic, openai, gemini)", model)
}
