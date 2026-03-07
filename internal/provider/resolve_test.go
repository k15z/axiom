package provider

import "testing"

func TestInferProvider(t *testing.T) {
	cases := []struct {
		model    string
		want     string
		wantErr  bool
	}{
		{"claude-haiku-4-5-20251001", "anthropic", false},
		{"claude-sonnet-4-5", "anthropic", false},
		{"anthropic/claude-3-opus", "anthropic", false},
		{"gpt-4o", "openai", false},
		{"gpt-4o-mini", "openai", false},
		{"o1-preview", "openai", false},
		{"o3-mini", "openai", false},
		{"o4-mini", "openai", false},
		{"openai/gpt-4", "openai", false},
		{"gemini-2.0-flash", "gemini", false},
		{"gemini-1.5-pro", "gemini", false},
		{"google/gemini-pro", "gemini", false},
		{"my-local-model", "", true},
		{"llama-3", "", true},
	}

	for _, tt := range cases {
		t.Run(tt.model, func(t *testing.T) {
			got, err := InferProvider(tt.model)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("InferProvider(%q) = %q, want error", tt.model, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("InferProvider(%q) error: %v", tt.model, err)
			}
			if got != tt.want {
				t.Errorf("InferProvider(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

func TestResolveProvider(t *testing.T) {
	cases := []struct {
		name     string
		explicit string
		model    string
		want     string
		wantErr  bool
	}{
		{"explicit anthropic", "anthropic", "anything", "anthropic", false},
		{"explicit openai", "openai", "anything", "openai", false},
		{"explicit gemini", "gemini", "anything", "gemini", false},
		{"explicit unknown", "azure", "anything", "", true},
		{"infer from model", "", "gpt-4o", "openai", false},
		{"infer fails", "", "unknown-model", "", true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveProvider(tt.explicit, tt.model)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveProvider(%q, %q) = %q, want error", tt.explicit, tt.model, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveProvider(%q, %q) error: %v", tt.explicit, tt.model, err)
			}
			if got != tt.want {
				t.Errorf("ResolveProvider(%q, %q) = %q, want %q", tt.explicit, tt.model, got, tt.want)
			}
		})
	}
}
