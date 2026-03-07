package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/provider"
)

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

// --- Integration tests for runner.Run ---

// mockRunnerProvider returns canned responses for testing without real API calls.
type mockRunnerProvider struct {
	response func(condition string) (*provider.ChatResponse, error)
}

func (m *mockRunnerProvider) Chat(_ context.Context, params provider.ChatParams) (*provider.ChatResponse, error) {
	condition := ""
	if len(params.Messages) > 0 {
		for _, b := range params.Messages[0].Content {
			if b.Type == "text" {
				condition = b.Text
				break
			}
		}
	}
	return m.response(condition)
}

func passResponse() (*provider.ChatResponse, error) {
	return &provider.ChatResponse{
		Content: []provider.ContentBlock{{Type: "text", Text: "VERDICT: PASS\nAll good."}},
		Usage:   provider.Usage{InputTokens: 100, OutputTokens: 50},
	}, nil
}

func failResponse() (*provider.ChatResponse, error) {
	return &provider.ChatResponse{
		Content: []provider.ContentBlock{{Type: "text", Text: "VERDICT: FAIL\nNot good."}},
		Usage:   provider.Usage{InputTokens: 100, OutputTokens: 50},
	}, nil
}

func errorResponse() (*provider.ChatResponse, error) {
	return nil, fmt.Errorf("503 Service Unavailable")
}

func setupMock(mock *mockRunnerProvider) func() {
	orig := newProvider
	newProvider = func(_ config.Config, _ provider.ProgressFunc) provider.Provider {
		return mock
	}
	return func() { newProvider = orig }
}

func minimalConfig(t *testing.T) config.Config {
	t.Helper()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	os.MkdirAll(cacheDir, 0o755)
	return config.Config{
		Model:    "test-model",
		Provider: "anthropic",
		APIKey:   "test-key",
		Agent: config.AgentConfig{
			MaxIterations: 5,
			MaxTokens:     1000,
		},
		Cache: config.CacheConfig{
			Dir: cacheDir,
		},
	}
}

func makeTest(name string) discovery.Test {
	return discovery.Test{
		Name:       name,
		Condition:  "check " + name,
		SourceFile: "test.yml",
	}
}

func TestRun_SinglePassingTest(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return passResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	results, err := Run(context.Background(), minimalConfig(t), []discovery.Test{makeTest("test_a")}, Options{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Error("expected pass")
	}
	if results[0].Cached {
		t.Error("should not be cached on first run")
	}
}

func TestRun_SingleFailingTest(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return failResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	results, err := Run(context.Background(), minimalConfig(t), []discovery.Test{makeTest("test_a")}, Options{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Passed {
		t.Error("expected fail")
	}
}

func TestRun_InfrastructureError(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return errorResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	results, err := Run(context.Background(), minimalConfig(t), []discovery.Test{makeTest("test_a")}, Options{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !results[0].Errored {
		t.Error("expected Errored=true for infrastructure error")
	}
	if results[0].Passed {
		t.Error("errored test should not be marked as passed")
	}
}

func TestRun_InfrastructureErrorNotRetried(t *testing.T) {
	var callCount int64
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) {
		atomic.AddInt64(&callCount, 1)
		return errorResponse()
	}}
	cleanup := setupMock(mock)
	defer cleanup()

	results, err := Run(context.Background(), minimalConfig(t), []discovery.Test{makeTest("test_a")}, Options{
		Concurrency: 1,
		Retries:     3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !results[0].Errored {
		t.Error("expected Errored=true")
	}
	if atomic.LoadInt64(&callCount) != 1 {
		t.Errorf("expected 1 call (no retries for infra errors), got %d", atomic.LoadInt64(&callCount))
	}
}

func TestRun_RetryMakesFlakyOnPass(t *testing.T) {
	var callCount int64
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) {
		n := atomic.AddInt64(&callCount, 1)
		if n == 1 {
			return failResponse()
		}
		return passResponse()
	}}
	cleanup := setupMock(mock)
	defer cleanup()

	results, err := Run(context.Background(), minimalConfig(t), []discovery.Test{makeTest("test_a")}, Options{
		Concurrency: 1,
		Retries:     2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !results[0].Passed {
		t.Error("expected pass on retry")
	}
	if !results[0].Flaky {
		t.Error("expected Flaky=true when test passes on retry")
	}
	if results[0].Retries != 1 {
		t.Errorf("Retries = %d, want 1", results[0].Retries)
	}
}

func TestRun_Filter(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return passResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	tests := []discovery.Test{makeTest("auth_test"), makeTest("db_test"), makeTest("auth_check")}
	results, err := Run(context.Background(), minimalConfig(t), tests, Options{
		Concurrency: 1,
		Filter:      "auth_*",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results matching 'auth_*', got %d", len(results))
	}
}

func TestRun_TagFilter(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return passResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	tests := []discovery.Test{
		{Name: "test_a", Condition: "check a", SourceFile: "test.yml", Tags: []string{"security"}},
		{Name: "test_b", Condition: "check b", SourceFile: "test.yml", Tags: []string{"performance"}},
		{Name: "test_c", Condition: "check c", SourceFile: "test.yml", Tags: []string{"security", "auth"}},
	}
	results, err := Run(context.Background(), minimalConfig(t), tests, Options{
		Concurrency: 1,
		Tag:         "security",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results with 'security' tag, got %d", len(results))
	}
}

func TestRun_BailStopsOnFirstFailure(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return failResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	tests := []discovery.Test{makeTest("test_a"), makeTest("test_b"), makeTest("test_c")}
	results, err := Run(context.Background(), minimalConfig(t), tests, Options{
		Concurrency: 1,
		Bail:        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	failed := 0
	skipped := 0
	for _, r := range results {
		if r.Skipped {
			skipped++
		} else if !r.Passed {
			failed++
		}
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
	if skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", skipped)
	}
}

func TestRun_ConcurrentExecution(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return passResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	tests := make([]discovery.Test, 10)
	for i := range tests {
		tests[i] = makeTest(fmt.Sprintf("test_%d", i))
	}

	results, err := Run(context.Background(), minimalConfig(t), tests, Options{Concurrency: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	for i, r := range results {
		if !r.Passed {
			t.Errorf("test %d failed unexpectedly", i)
		}
	}
}

func TestRun_ResultsPreserveOrder(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return passResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	tests := []discovery.Test{makeTest("alpha"), makeTest("beta"), makeTest("gamma")}
	results, err := Run(context.Background(), minimalConfig(t), tests, Options{Concurrency: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Test.Name != "alpha" || results[1].Test.Name != "beta" || results[2].Test.Name != "gamma" {
		t.Errorf("results out of order: %s, %s, %s",
			results[0].Test.Name, results[1].Test.Name, results[2].Test.Name)
	}
}

func TestRun_EmptyTestList(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return passResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	results, err := Run(context.Background(), minimalConfig(t), nil, Options{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRun_ToolCallResponse(t *testing.T) {
	var callCount int64
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) {
		n := atomic.AddInt64(&callCount, 1)
		if n == 1 {
			input, _ := json.Marshal(map[string]any{"path": "."})
			return &provider.ChatResponse{
				Content: []provider.ContentBlock{{
					Type:     "tool_use",
					ToolName: "list_dir",
					ToolID:   "call_1",
					Input:    input,
				}},
				Usage: provider.Usage{InputTokens: 100, OutputTokens: 50},
			}, nil
		}
		return passResponse()
	}}
	cleanup := setupMock(mock)
	defer cleanup()

	results, err := Run(context.Background(), minimalConfig(t), []discovery.Test{makeTest("test_a")}, Options{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !results[0].Passed {
		t.Error("expected pass after tool call")
	}
	if results[0].Usage.APICalls != 2 {
		t.Errorf("APICalls = %d, want 2", results[0].Usage.APICalls)
	}
}

func TestRun_ConcurrentMixedResults(t *testing.T) {
	// Even-indexed tests pass, odd-indexed fail.
	// The agent wraps the condition in a prompt, so we check for substring.
	mock := &mockRunnerProvider{response: func(condition string) (*provider.ChatResponse, error) {
		if strings.Contains(condition, "check test_1") || strings.Contains(condition, "check test_3") {
			return failResponse()
		}
		return passResponse()
	}}
	cleanup := setupMock(mock)
	defer cleanup()

	tests := make([]discovery.Test, 4)
	for i := range tests {
		tests[i] = makeTest(fmt.Sprintf("test_%d", i))
	}

	results, err := Run(context.Background(), minimalConfig(t), tests, Options{Concurrency: 4})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	// Verify each result matches expected outcome
	for i, r := range results {
		wantPass := i%2 == 0
		if r.Passed != wantPass {
			t.Errorf("test_%d: Passed=%v, want %v", i, r.Passed, wantPass)
		}
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return passResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	tests := []discovery.Test{makeTest("test_a"), makeTest("test_b")}
	results, err := Run(ctx, minimalConfig(t), tests, Options{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All tests should be skipped since context was already cancelled
	for _, r := range results {
		if !r.Skipped {
			t.Errorf("test %s: expected Skipped=true with cancelled context", r.Test.Name)
		}
	}
}

func TestRun_RetryExhausted(t *testing.T) {
	var callCount int64
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) {
		atomic.AddInt64(&callCount, 1)
		return failResponse()
	}}
	cleanup := setupMock(mock)
	defer cleanup()

	results, err := Run(context.Background(), minimalConfig(t), []discovery.Test{makeTest("test_a")}, Options{
		Concurrency: 1,
		Retries:     2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Passed {
		t.Error("expected fail after all retries exhausted")
	}
	if results[0].Flaky {
		t.Error("should not be flaky when all retries fail")
	}
	if results[0].Retries != 2 {
		t.Errorf("Retries = %d, want 2", results[0].Retries)
	}
	// 1 initial + 2 retries = 3 calls
	if got := atomic.LoadInt64(&callCount); got != 3 {
		t.Errorf("expected 3 API calls (1 + 2 retries), got %d", got)
	}
}

func TestRun_UsageAccumulation(t *testing.T) {
	var callCount int64
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) {
		n := atomic.AddInt64(&callCount, 1)
		if n == 1 {
			return failResponse()
		}
		return passResponse()
	}}
	cleanup := setupMock(mock)
	defer cleanup()

	results, err := Run(context.Background(), minimalConfig(t), []discovery.Test{makeTest("test_a")}, Options{
		Concurrency: 1,
		Retries:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both initial and retry contribute 100 input + 50 output tokens
	if results[0].Usage.InputTokens < 200 {
		t.Errorf("InputTokens = %d, want >= 200 (accumulated across retry)", results[0].Usage.InputTokens)
	}
	if results[0].Usage.OutputTokens < 100 {
		t.Errorf("OutputTokens = %d, want >= 100 (accumulated across retry)", results[0].Usage.OutputTokens)
	}
	if results[0].Usage.APICalls < 2 {
		t.Errorf("APICalls = %d, want >= 2", results[0].Usage.APICalls)
	}
}

func TestMatchesTag_CommaSeparated(t *testing.T) {
	test := discovery.Test{Tags: []string{"security", "auth"}}
	if !MatchesTag(test, "security,auth") {
		t.Error("should match when test has one of the comma-separated tags")
	}
	if !MatchesTag(test, "perf,auth") {
		t.Error("should match when test has at least one tag from comma list")
	}
	if MatchesTag(test, "perf,db") {
		t.Error("should not match when test has none of the tags")
	}
}

func TestMatchesTag_CaseInsensitive(t *testing.T) {
	test := discovery.Test{Tags: []string{"Security"}}
	if !MatchesTag(test, "security") {
		t.Error("tag matching should be case-insensitive")
	}
	if !MatchesTag(test, "SECURITY") {
		t.Error("tag matching should be case-insensitive (uppercase)")
	}
}

func TestMatchesTag_EmptyFilter(t *testing.T) {
	test := discovery.Test{Tags: []string{"security"}}
	if !MatchesTag(test, "") {
		t.Error("empty filter should match all tests")
	}
}

func TestMatchesTag_NoTags(t *testing.T) {
	test := discovery.Test{}
	if MatchesTag(test, "security") {
		t.Error("test with no tags should not match a tag filter")
	}
}

func TestMatchesTag_WhitespaceInFilter(t *testing.T) {
	test := discovery.Test{Tags: []string{"security"}}
	if !MatchesTag(test, " security , auth ") {
		t.Error("should trim whitespace around tag filter values")
	}
}

func TestRun_FilterAndTagCombined(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) { return passResponse() }}
	cleanup := setupMock(mock)
	defer cleanup()

	tests := []discovery.Test{
		{Name: "auth_login", Condition: "check auth", SourceFile: "test.yml", Tags: []string{"security"}},
		{Name: "auth_logout", Condition: "check auth", SourceFile: "test.yml", Tags: []string{"ux"}},
		{Name: "db_query", Condition: "check db", SourceFile: "test.yml", Tags: []string{"security"}},
	}
	results, err := Run(context.Background(), minimalConfig(t), tests, Options{
		Concurrency: 1,
		Filter:      "auth_*",
		Tag:         "security",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only auth_login matches both filter (auth_*) and tag (security)
	if len(results) != 1 {
		t.Errorf("expected 1 result matching both filter and tag, got %d", len(results))
	}
	if len(results) > 0 && results[0].Test.Name != "auth_login" {
		t.Errorf("expected auth_login, got %s", results[0].Test.Name)
	}
}
