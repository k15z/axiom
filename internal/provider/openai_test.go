package provider

import (
	"encoding/json"
	"testing"
)

func TestOpenAIConvertMessage_UserText(t *testing.T) {
	p := &OpenAIProvider{}
	m := Message{
		Role: "user",
		Content: []ContentBlock{
			{Type: "text", Text: "hello"},
		},
	}

	msgs := p.convertMessage(m)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("role = %q, want %q", msgs[0].Role, "user")
	}
	if msgs[0].Content != "hello" {
		t.Errorf("content = %v, want %q", msgs[0].Content, "hello")
	}
}

func TestOpenAIConvertMessage_AssistantWithToolCalls(t *testing.T) {
	p := &OpenAIProvider{}
	m := Message{
		Role: "assistant",
		Content: []ContentBlock{
			{Type: "text", Text: "Let me check"},
			{Type: "tool_use", ToolName: "read_file", ToolID: "call_123", Input: json.RawMessage(`{"path":"main.go"}`)},
		},
	}

	msgs := p.convertMessage(m)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Role != "assistant" {
		t.Errorf("role = %q, want %q", msgs[0].Role, "assistant")
	}
	if len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(msgs[0].ToolCalls))
	}
	tc := msgs[0].ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("tool call ID = %q, want %q", tc.ID, "call_123")
	}
	if tc.Function.Name != "read_file" {
		t.Errorf("tool call name = %q, want %q", tc.Function.Name, "read_file")
	}
}

func TestOpenAIConvertMessage_ToolResults(t *testing.T) {
	p := &OpenAIProvider{}
	m := Message{
		Role: "user",
		Content: []ContentBlock{
			{Type: "tool_result", ToolID: "call_123", Text: "file contents here"},
			{Type: "text", Text: "budget hint"},
		},
	}

	msgs := p.convertMessage(m)
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	// First should be tool result
	if msgs[0].Role != "tool" {
		t.Errorf("msgs[0].role = %q, want %q", msgs[0].Role, "tool")
	}
	if msgs[0].ToolCallID != "call_123" {
		t.Errorf("msgs[0].ToolCallID = %q, want %q", msgs[0].ToolCallID, "call_123")
	}
	// Second should be user text
	if msgs[1].Role != "user" {
		t.Errorf("msgs[1].role = %q, want %q", msgs[1].Role, "user")
	}
}

func TestExtractText(t *testing.T) {
	cases := []struct {
		name    string
		content any
		want    string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"empty string", "", ""},
		{"number", 42, ""},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := extractText(tt.content)
			if got != tt.want {
				t.Errorf("extractText(%v) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}
