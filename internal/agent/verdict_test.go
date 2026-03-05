package agent

import "testing"

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
