package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/k15z/axiom/internal/provider"
)

// mockProvider is a configurable mock for provider.Provider.
// responses is a queue: each Chat call pops the next response.
type mockProvider struct {
	responses []mockResponse
	calls     []provider.ChatParams // records all calls
	callIdx   int
}

type mockResponse struct {
	resp *provider.ChatResponse
	err  error
}

func (m *mockProvider) Chat(_ context.Context, params provider.ChatParams) (*provider.ChatResponse, error) {
	m.calls = append(m.calls, params)
	if m.callIdx >= len(m.responses) {
		return nil, fmt.Errorf("unexpected call #%d: no more mock responses", m.callIdx)
	}
	r := m.responses[m.callIdx]
	m.callIdx++
	return r.resp, r.err
}

// textResponse creates a mock response with a single text block.
func textResponse(text string, inputTokens, outputTokens int) mockResponse {
	return mockResponse{
		resp: &provider.ChatResponse{
			Content: []provider.ContentBlock{{Type: "text", Text: text}},
			Usage:   provider.Usage{InputTokens: inputTokens, OutputTokens: outputTokens},
		},
	}
}

// toolCallResponse creates a mock response with a tool_use block.
func toolCallResponse(toolName, toolID string, input map[string]any, inputTokens, outputTokens int) mockResponse {
	raw, _ := json.Marshal(input)
	return mockResponse{
		resp: &provider.ChatResponse{
			Content: []provider.ContentBlock{{
				Type:     "tool_use",
				ToolName: toolName,
				ToolID:   toolID,
				Input:    raw,
			}},
			Usage: provider.Usage{InputTokens: inputTokens, OutputTokens: outputTokens},
		},
	}
}

// errorResponse creates a mock that returns an error.
func errorResponse(err error) mockResponse {
	return mockResponse{err: err}
}

func TestRun_BasicPass(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			textResponse("VERDICT: PASS\nAll good.", 100, 50),
		},
	}

	result, err := Run(context.Background(), mock, "test-model", "code is correct", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected pass")
	}
	if result.Reasoning != "All good." {
		t.Errorf("reasoning = %q, want %q", result.Reasoning, "All good.")
	}
	if result.Usage.APICalls != 1 {
		t.Errorf("APICalls = %d, want 1", result.Usage.APICalls)
	}
	if result.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", result.Usage.OutputTokens)
	}
}

func TestRun_BasicFail(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			textResponse("VERDICT: FAIL\nMissing validation.", 100, 50),
		},
	}

	result, err := Run(context.Background(), mock, "test-model", "validate inputs", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Error("expected fail")
	}
	if result.Reasoning != "Missing validation." {
		t.Errorf("reasoning = %q, want %q", result.Reasoning, "Missing validation.")
	}
}

func TestRun_ToolDispatch(t *testing.T) {
	tmpDir := t.TempDir()

	mock := &mockProvider{
		responses: []mockResponse{
			// First call: agent requests list_dir
			toolCallResponse("list_dir", "call_1", map[string]any{"path": "."}, 200, 100),
			// Second call: agent gives verdict
			textResponse("VERDICT: PASS\nFound the files.", 300, 150),
		},
	}

	result, err := Run(context.Background(), mock, "test-model", "check structure", nil, tmpDir, nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected pass")
	}
	if result.Usage.APICalls != 2 {
		t.Errorf("APICalls = %d, want 2", result.Usage.APICalls)
	}
	if result.Usage.InputTokens != 500 {
		t.Errorf("InputTokens = %d, want 500", result.Usage.InputTokens)
	}
	// Verify the second call includes tool results
	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(mock.calls))
	}
	secondCall := mock.calls[1]
	if len(secondCall.Messages) < 3 {
		t.Fatalf("expected at least 3 messages in second call, got %d", len(secondCall.Messages))
	}
	// Messages: [user condition, assistant tool_use, user tool_result]
	lastMsg := secondCall.Messages[len(secondCall.Messages)-1]
	if lastMsg.Role != "user" {
		t.Errorf("last message role = %q, want %q", lastMsg.Role, "user")
	}
	if len(lastMsg.Content) == 0 || lastMsg.Content[0].Type != "tool_result" {
		t.Error("expected tool_result in last message")
	}
}

func TestRun_IterationLimit(t *testing.T) {
	// Agent keeps making tool calls, never gives a verdict
	mock := &mockProvider{
		responses: []mockResponse{
			toolCallResponse("list_dir", "c1", map[string]any{"path": "."}, 100, 50),
			toolCallResponse("list_dir", "c2", map[string]any{"path": "."}, 100, 50),
			toolCallResponse("list_dir", "c3", map[string]any{"path": "."}, 100, 50),
		},
	}

	result, err := Run(context.Background(), mock, "test-model", "check", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 3,
		MaxTokens:     1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Error("expected fail when iteration limit reached")
	}
	if !strings.Contains(result.Reasoning, "exceeded maximum iterations") {
		t.Errorf("reasoning = %q, want to contain 'exceeded maximum iterations'", result.Reasoning)
	}
	if result.Usage.APICalls != 3 {
		t.Errorf("APICalls = %d, want 3", result.Usage.APICalls)
	}
}

func TestRun_ProviderError(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			errorResponse(fmt.Errorf("503 Service Unavailable")),
		},
	}

	result, err := Run(context.Background(), mock, "test-model", "check", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err == nil {
		t.Fatal("expected error from provider")
	}
	if !strings.Contains(err.Error(), "API call failed") {
		t.Errorf("error = %q, want to contain 'API call failed'", err.Error())
	}
	if !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Errorf("error = %q, want to contain original error", err.Error())
	}
	// Usage should reflect zero calls completed
	if result.Usage.APICalls != 0 {
		t.Errorf("APICalls = %d, want 0", result.Usage.APICalls)
	}
}

func TestRun_ProviderErrorAfterToolCall(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			toolCallResponse("list_dir", "c1", map[string]any{"path": "."}, 200, 100),
			errorResponse(fmt.Errorf("rate limited")),
		},
	}

	result, err := Run(context.Background(), mock, "test-model", "check", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err == nil {
		t.Fatal("expected error from provider")
	}
	// First call succeeded, so usage should reflect it
	if result.Usage.APICalls != 1 {
		t.Errorf("APICalls = %d, want 1 (one successful call before error)", result.Usage.APICalls)
	}
}

func TestRun_BudgetHintInjection(t *testing.T) {
	// Set MaxTokens=100 so budget = 300, threshold = 225.
	// After first tool call with 200+100=300 tokens, we're over 75%.
	mock := &mockProvider{
		responses: []mockResponse{
			toolCallResponse("list_dir", "c1", map[string]any{"path": "."}, 200, 100),
			textResponse("VERDICT: PASS\nDone.", 50, 30),
		},
	}

	result, err := Run(context.Background(), mock, "test-model", "check", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected pass")
	}

	// Check that the budget hint was injected in the second call's messages
	if len(mock.calls) < 2 {
		t.Fatal("expected at least 2 calls")
	}
	secondCall := mock.calls[1]
	lastMsg := secondCall.Messages[len(secondCall.Messages)-1]
	foundHint := false
	for _, block := range lastMsg.Content {
		if block.Type == "text" && strings.Contains(block.Text, "token budget") {
			foundHint = true
			break
		}
	}
	if !foundHint {
		t.Error("expected budget hint to be injected after exceeding 75% of token budget")
	}
}

func TestRun_NotesExtraction(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			textResponse("VERDICT: PASS\nLooks good.\nNOTES:\nKey file: internal/auth.go:42\nUses JWT tokens", 100, 50),
		},
	}

	result, err := Run(context.Background(), mock, "test-model", "check auth", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Notes == "" {
		t.Error("expected non-empty notes")
	}
	if !strings.Contains(result.Notes, "JWT tokens") {
		t.Errorf("notes = %q, want to contain 'JWT tokens'", result.Notes)
	}
	if len(result.NoteFiles) == 0 {
		t.Error("expected file paths in notes")
	}
	found := false
	for _, f := range result.NoteFiles {
		if f == "internal/auth.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("NoteFiles = %v, want to contain 'internal/auth.go'", result.NoteFiles)
	}
	// Reasoning should not contain NOTES section
	if strings.Contains(result.Reasoning, "NOTES:") {
		t.Error("reasoning should not contain NOTES section")
	}
}

func TestRun_PriorNotesInjected(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			textResponse("VERDICT: PASS", 100, 50),
		},
	}

	_, err := Run(context.Background(), mock, "test-model", "check", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
		PriorNotes:    "Previous: auth uses JWT",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First message should contain the prior notes
	if len(mock.calls) == 0 {
		t.Fatal("expected at least 1 call")
	}
	firstMsg := mock.calls[0].Messages[0]
	if !strings.Contains(firstMsg.Content[0].Text, "Previous: auth uses JWT") {
		t.Errorf("first message = %q, want to contain prior notes", firstMsg.Content[0].Text)
	}
}

func TestRun_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	mock := &mockProvider{
		responses: []mockResponse{
			errorResponse(context.Canceled),
		},
	}

	_, err := Run(ctx, mock, "test-model", "check", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRun_ProgressEvents(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			toolCallResponse("list_dir", "c1", map[string]any{"path": "."}, 100, 50),
			textResponse("VERDICT: PASS", 100, 50),
		},
	}

	var events []Event
	progress := func(e Event) {
		events = append(events, e)
	}

	_, err := Run(context.Background(), mock, "test-model", "check", nil, t.TempDir(), progress, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: thinking, tool_call, thinking, text
	kinds := make(map[string]int)
	for _, e := range events {
		kinds[e.Kind]++
	}
	if kinds["thinking"] < 2 {
		t.Errorf("expected at least 2 thinking events, got %d", kinds["thinking"])
	}
	if kinds["tool_call"] < 1 {
		t.Errorf("expected at least 1 tool_call event, got %d", kinds["tool_call"])
	}
	if kinds["text"] < 1 {
		t.Errorf("expected at least 1 text event, got %d", kinds["text"])
	}
}

func TestRun_SystemPromptPassed(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			textResponse("VERDICT: PASS", 100, 50),
		},
	}

	_, err := Run(context.Background(), mock, "test-model", "check", nil, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.calls[0].System != systemPrompt {
		t.Errorf("system prompt not passed to provider")
	}
	if mock.calls[0].Model != "test-model" {
		t.Errorf("model = %q, want %q", mock.calls[0].Model, "test-model")
	}
	if mock.calls[0].MaxTokens != 1000 {
		t.Errorf("MaxTokens = %d, want 1000", mock.calls[0].MaxTokens)
	}
}

func TestRun_OnGlobsInCondition(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			textResponse("VERDICT: PASS", 100, 50),
		},
	}

	_, err := Run(context.Background(), mock, "test-model", "check auth", []string{"src/*.go", "pkg/*.go"}, t.TempDir(), nil, RunOptions{
		MaxIterations: 5,
		MaxTokens:     1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	firstMsg := mock.calls[0].Messages[0].Content[0].Text
	if !strings.Contains(firstMsg, "Condition: check auth") {
		t.Errorf("first message = %q, want to contain condition", firstMsg)
	}
	if !strings.Contains(firstMsg, "Start with: src/*.go, pkg/*.go") {
		t.Errorf("first message = %q, want to contain on globs", firstMsg)
	}
}

func TestRun_DefaultIterationsAndTokens(t *testing.T) {
	mock := &mockProvider{
		responses: []mockResponse{
			textResponse("VERDICT: PASS", 100, 50),
		},
	}

	_, err := Run(context.Background(), mock, "test-model", "check", nil, t.TempDir(), nil, RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Default maxTokens should be 10000
	if mock.calls[0].MaxTokens != 10000 {
		t.Errorf("default MaxTokens = %d, want 10000", mock.calls[0].MaxTokens)
	}
}
