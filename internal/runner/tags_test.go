package runner

import (
	"testing"

	"github.com/k15z/axiom/internal/discovery"
)

func TestMatchesTag(t *testing.T) {
	tests := []struct {
		name      string
		tags      []string
		tagFilter string
		want      bool
	}{
		{"empty filter matches all", []string{"ci"}, "", true},
		{"empty filter matches untagged", nil, "", true},
		{"exact match", []string{"ci", "fast"}, "ci", true},
		{"no match", []string{"ci"}, "slow", false},
		{"untagged excluded", nil, "ci", false},
		{"comma-separated OR", []string{"slow"}, "ci,slow", true},
		{"comma-separated no match", []string{"fast"}, "ci,slow", false},
		{"case insensitive", []string{"CI"}, "ci", true},
		{"case insensitive reverse", []string{"ci"}, "CI", true},
		{"whitespace in filter", []string{"ci"}, " ci , fast ", true},
		{"multiple tags first match", []string{"ci", "fast", "unit"}, "fast", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test := discovery.Test{Name: "test", Tags: tt.tags}
			got := MatchesTag(test, tt.tagFilter)
			if got != tt.want {
				t.Errorf("MatchesTag(tags=%v, filter=%q) = %v, want %v", tt.tags, tt.tagFilter, got, tt.want)
			}
		})
	}
}
