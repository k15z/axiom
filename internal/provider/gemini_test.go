package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiConvertMessage_UserText(t *testing.T) {
	p := &GeminiProvider{}
	m := Message{
		Role: "user",
		Content: []ContentBlock{
			{Type: "text", Text: "hello"},
		},
	}

	contents := p.convertMessage(m)
	if len(contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(contents))
	}
	if contents[0].Role != "user" {
		t.Errorf("role = %q, want %q", contents[0].Role, "user")
	}
	if len(contents[0].Parts) != 1 || contents[0].Parts[0].Text != "hello" {
		t.Errorf("parts = %+v, want single text part 'hello'", contents[0].Parts)
	}
}

func TestGeminiConvertMessage_AssistantRole(t *testing.T) {
	p := &GeminiProvider{}
	m := Message{
		Role: "assistant",
		Content: []ContentBlock{
			{Type: "text", Text: "thinking"},
		},
	}

	contents := p.convertMessage(m)
	if len(contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(contents))
	}
	if contents[0].Role != "model" {
		t.Errorf("role = %q, want %q (Gemini maps assistant to model)", contents[0].Role, "model")
	}
}

func TestGeminiConvertMessage_ToolUse(t *testing.T) {
	p := &GeminiProvider{}
	m := Message{
		Role: "assistant",
		Content: []ContentBlock{
			{Type: "tool_use", ToolName: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)},
		},
	}

	contents := p.convertMessage(m)
	if len(contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(contents))
	}
	if contents[0].Parts[0].FunctionCall == nil {
		t.Fatal("expected FunctionCall, got nil")
	}
	if contents[0].Parts[0].FunctionCall.Name != "read_file" {
		t.Errorf("function name = %q, want %q", contents[0].Parts[0].FunctionCall.Name, "read_file")
	}
}

func TestGeminiConvertMessage_ToolResult(t *testing.T) {
	p := &GeminiProvider{}
	m := Message{
		Role: "user",
		Content: []ContentBlock{
			{Type: "tool_result", ToolName: "read_file", Text: "file contents", IsError: false},
		},
	}

	contents := p.convertMessage(m)
	if len(contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(contents))
	}
	// Tool results should be sent as "user" role
	if contents[0].Role != "user" {
		t.Errorf("role = %q, want %q", contents[0].Role, "user")
	}
	if contents[0].Parts[0].FunctionResponse == nil {
		t.Fatal("expected FunctionResponse, got nil")
	}
	if contents[0].Parts[0].FunctionResponse.Name != "read_file" {
		t.Errorf("function name = %q, want %q", contents[0].Parts[0].FunctionResponse.Name, "read_file")
	}
	if contents[0].Parts[0].FunctionResponse.Response.Content != "file contents" {
		t.Errorf("response content = %q, want %q", contents[0].Parts[0].FunctionResponse.Response.Content, "file contents")
	}
}

func TestGeminiConvertMessage_ToolResultError(t *testing.T) {
	p := &GeminiProvider{}
	m := Message{
		Role: "user",
		Content: []ContentBlock{
			{Type: "tool_result", ToolName: "read_file", Text: "not found", IsError: true},
		},
	}

	contents := p.convertMessage(m)
	if len(contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(contents))
	}
	if !contents[0].Parts[0].FunctionResponse.Response.IsError {
		t.Error("expected IsError=true for error tool result")
	}
}

func TestGeminiConvertMessage_MixedContent(t *testing.T) {
	p := &GeminiProvider{}
	m := Message{
		Role: "assistant",
		Content: []ContentBlock{
			{Type: "text", Text: "Let me check"},
			{Type: "tool_use", ToolName: "grep", Input: json.RawMessage(`{"pattern":"TODO"}`)},
		},
	}

	contents := p.convertMessage(m)
	if len(contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(contents))
	}
	if len(contents[0].Parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(contents[0].Parts))
	}
	if contents[0].Parts[0].Text != "Let me check" {
		t.Errorf("parts[0].text = %q, want %q", contents[0].Parts[0].Text, "Let me check")
	}
	if contents[0].Parts[1].FunctionCall == nil {
		t.Error("parts[1] should have FunctionCall")
	}
}

func TestGeminiConvertMessage_TextAndToolResult(t *testing.T) {
	p := &GeminiProvider{}
	m := Message{
		Role: "user",
		Content: []ContentBlock{
			{Type: "text", Text: "budget hint"},
			{Type: "tool_result", ToolName: "read_file", Text: "contents"},
		},
	}

	contents := p.convertMessage(m)
	// Text goes in one content block, tool results in another
	if len(contents) != 2 {
		t.Fatalf("got %d contents, want 2 (text + tool result as separate contents)", len(contents))
	}
	if contents[0].Parts[0].Text != "budget hint" {
		t.Errorf("first content text = %q, want %q", contents[0].Parts[0].Text, "budget hint")
	}
	if contents[1].Parts[0].FunctionResponse == nil {
		t.Error("second content should have FunctionResponse")
	}
}

func TestGeminiDoRequest_StripsGooglePrefix(t *testing.T) {
	var requestedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"VERDICT: PASS"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`))
	}))
	defer srv.Close()

	// Swap out the transport so requests to geminiBaseURL go to our test server
	p := &GeminiProvider{
		apiKey: "test-key",
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				// Capture the path, then forward to our test server
				requestedPath = req.URL.Path
				req.URL.Scheme = "http"
				req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	_, err := p.Chat(t.Context(), ChatParams{
		Model:     "google/gemini-2.0-flash",
		System:    "test",
		Messages:  []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hello"}}}},
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// The URL path should NOT contain "google/" -- stripGooglePrefix should remove it
	if strings.Contains(requestedPath, "google/") {
		t.Errorf("request path %q still contains 'google/' prefix; stripGooglePrefix not applied", requestedPath)
	}
	wantSuffix := "/models/gemini-2.0-flash:generateContent"
	if !strings.HasSuffix(requestedPath, wantSuffix) {
		t.Errorf("request path = %q, want suffix %q", requestedPath, wantSuffix)
	}
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestStripGooglePrefix(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"google/gemini-2.0-flash", "gemini-2.0-flash"},
		{"gemini-2.0-flash", "gemini-2.0-flash"},
		{"google/", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := stripGooglePrefix(tc.input)
		if got != tc.want {
			t.Errorf("stripGooglePrefix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
