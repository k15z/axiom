// Package provider defines a common interface for LLM providers and
// implements Anthropic, OpenAI, and OpenAI-compatible backends.
package provider

import (
	"context"
	"encoding/json"
)

// Provider abstracts LLM chat completion with tool use.
type Provider interface {
	Chat(ctx context.Context, params ChatParams) (*ChatResponse, error)
}

// ChatParams holds the inputs for a chat completion request.
type ChatParams struct {
	Model     string
	System    string
	Messages  []Message
	Tools     []Tool
	MaxTokens int
}

// Message represents a single conversation turn.
type Message struct {
	Role    string         // "user" or "assistant"
	Content []ContentBlock // one or more blocks
}

// ContentBlock is a polymorphic block within a message.
type ContentBlock struct {
	Type     string          // "text", "tool_use", "tool_result"
	Text     string          // for "text"
	ToolName string          // for "tool_use"
	ToolID   string          // for "tool_use" and "tool_result"
	Input    json.RawMessage // for "tool_use"
	IsError  bool            // for "tool_result"
}

// Tool describes a tool the model can call.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage // JSON Schema
}

// ChatResponse holds the outputs from a chat completion.
type ChatResponse struct {
	Content    []ContentBlock
	Usage      Usage
	StopReason string // "end_turn" or "tool_use"
}

// Usage tracks token consumption for a single API call.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// ProgressFunc receives streamed text deltas for live display.
type ProgressFunc func(text string)
