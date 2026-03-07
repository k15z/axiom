package runner

import "testing"

func TestAutoConcurrency(t *testing.T) {
	tests := []struct {
		numTests int
		want     int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 3},
		{4, 4},
		{5, 5},
		{6, 5},
		{10, 5},
		{100, 5},
	}

	for _, tt := range tests {
		got := AutoConcurrency(tt.numTests)
		if got != tt.want {
			t.Errorf("AutoConcurrency(%d) = %d, want %d", tt.numTests, got, tt.want)
		}
	}
}
