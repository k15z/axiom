package agent

import "testing"

func TestShouldInjectBudgetHint(t *testing.T) {
	tests := []struct {
		name         string
		inputTokens  int
		outputTokens int
		maxTokens    int
		want         bool
	}{
		{
			name:         "well under budget",
			inputTokens:  1000,
			outputTokens: 500,
			maxTokens:    10000,
			want:         false,
		},
		{
			name:         "exactly at 75% threshold",
			inputTokens:  15000,
			outputTokens: 7500,
			maxTokens:    10000,
			want:         true, // 22500 >= 30000*3/4 = 22500
		},
		{
			name:         "over 75% threshold",
			inputTokens:  20000,
			outputTokens: 5000,
			maxTokens:    10000,
			want:         true,
		},
		{
			name:         "just under 75% threshold",
			inputTokens:  15000,
			outputTokens: 7499,
			maxTokens:    10000,
			want:         false, // 22499 < 22500
		},
		{
			name:         "zero maxTokens disables hint",
			inputTokens:  5000,
			outputTokens: 5000,
			maxTokens:    0,
			want:         false,
		},
		{
			name:         "zero usage",
			inputTokens:  0,
			outputTokens: 0,
			maxTokens:    10000,
			want:         false,
		},
		{
			name:         "negative maxTokens disables hint",
			inputTokens:  5000,
			outputTokens: 5000,
			maxTokens:    -1,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldInjectBudgetHint(tt.inputTokens, tt.outputTokens, tt.maxTokens)
			if got != tt.want {
				t.Errorf("shouldInjectBudgetHint(%d, %d, %d) = %v, want %v",
					tt.inputTokens, tt.outputTokens, tt.maxTokens, got, tt.want)
			}
		})
	}
}

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		wantPassed    bool
		wantReasoning string // substring to check in reasoning ("" = skip check)
		noVerdict     bool   // expect "Could not parse verdict" reasoning
	}{
		{
			name:       "uppercase PASS",
			text:       "VERDICT: PASS",
			wantPassed: true,
		},
		{
			name:       "lowercase pass",
			text:       "verdict: pass",
			wantPassed: true,
		},
		{
			name:       "mixed case Pass",
			text:       "Verdict: Pass with flying colors",
			wantPassed: true,
		},
		{
			name:          "pass with reasoning",
			text:          "VERDICT: PASS The tests all passed successfully.",
			wantPassed:    true,
			wantReasoning: "The tests all passed successfully.",
		},
		{
			name:       "uppercase FAIL",
			text:       "VERDICT: FAIL",
			wantPassed: false,
		},
		{
			name:          "fail with reasoning",
			text:          "VERDICT: FAIL Missing required field",
			wantPassed:    false,
			wantReasoning: "Missing required field",
		},
		{
			name:       "pass takes priority over fail",
			text:       "VERDICT: PASS but also VERDICT: FAIL",
			wantPassed: true,
		},
		{
			name:      "no verdict",
			text:      "I completed the analysis but forgot the verdict.",
			noVerdict: true,
		},
		{
			name:      "empty string",
			text:      "",
			noVerdict: true,
		},
		{
			name:       "verdict embedded in text",
			text:       "After analysis, the VERDICT: FAIL because the output was incorrect.",
			wantPassed: false,
		},
		{
			name:       "verdict buried in longer text",
			text:       "I looked at the code carefully.\nHere is my analysis of the files.\nAfter reviewing everything:\n\nVERDICT: PASS\nThe implementation looks correct.",
			wantPassed: true,
		},
		{
			name:          "multi-line reasoning after verdict",
			text:          "VERDICT: FAIL\nLine 1 of reasoning\nLine 2 of reasoning",
			wantPassed:    false,
			wantReasoning: "Line 1 of reasoning\nLine 2 of reasoning",
		},
		{
			name:      "whitespace only",
			text:      "   \n\n\t  ",
			noVerdict: true,
		},
		{
			name:      "verdict-like but not exact",
			text:      "My VERDICT is PASS",
			noVerdict: true,
		},
		{
			name:      "verdict with extra whitespace does not match",
			text:      "VERDICT:   PASS   extra spaces",
			noVerdict: true,
		},
		{
			name:       "fail verdict at end of text",
			text:       "After thorough analysis of the codebase\nVERDICT: FAIL",
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVerdict(tt.text)
			if got.Passed != tt.wantPassed {
				t.Errorf("parseVerdict(%q).Passed = %v, want %v", tt.text, got.Passed, tt.wantPassed)
			}
			if tt.noVerdict {
				if got.Reasoning == "" || got.Passed {
					t.Errorf("expected no-verdict result, got Passed=%v Reasoning=%q", got.Passed, got.Reasoning)
				}
				return
			}
			if tt.wantReasoning != "" && got.Reasoning != tt.wantReasoning {
				t.Errorf("parseVerdict(%q).Reasoning = %q, want %q", tt.text, got.Reasoning, tt.wantReasoning)
			}
		})
	}
}
