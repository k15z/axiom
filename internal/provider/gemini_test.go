package provider

import (
	"encoding/json"
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
