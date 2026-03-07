package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// OpenAIProvider calls the OpenAI Chat Completions API (or any compatible endpoint).
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAI creates a provider for OpenAI or any OpenAI-compatible API.
// If baseURL is empty, defaults to the official OpenAI endpoint.
func NewOpenAI(apiKey, baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	// Trim trailing slash for consistent URL building
	baseURL = strings.TrimRight(baseURL, "/")
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// OpenAI request/response types (only what we need for tool calling).

type oaiRequest struct {
	Model      string       `json:"model"`
	Messages   []oaiMessage `json:"messages"`
	Tools      []oaiTool    `json:"tools,omitempty"`
	MaxTokens  int          `json:"max_tokens,omitempty"`
	ToolChoice string       `json:"tool_choice,omitempty"`
}

type oaiMessage struct {
	Role       string          `json:"role"`
	Content    any             `json:"content,omitempty"`    // string or []oaiContentPart or null
	ToolCalls  []oaiToolCall   `json:"tool_calls,omitempty"` // assistant messages
	ToolCallID string          `json:"tool_call_id,omitempty"` // tool messages
	Name       string          `json:"name,omitempty"`         // tool messages
}

type oaiContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type oaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function oaiToolCallFunc `json:"function"`
}

type oaiToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type oaiResponse struct {
	Choices []oaiChoice `json:"choices"`
	Usage   oaiUsage    `json:"usage"`
	Error   *oaiError   `json:"error,omitempty"`
}

type oaiChoice struct {
	Message      oaiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type oaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type oaiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (p *OpenAIProvider) Chat(ctx context.Context, params ChatParams) (*ChatResponse, error) {
	// Build messages
	var msgs []oaiMessage

	// System message
	if params.System != "" {
		msgs = append(msgs, oaiMessage{
			Role:    "system",
			Content: params.System,
		})
	}

	// Conversation messages
	for _, m := range params.Messages {
		msg := p.convertMessage(m)
		msgs = append(msgs, msg...)
	}

	// Build tools
	var tools []oaiTool
	for _, t := range params.Tools {
		tools = append(tools, oaiTool{
			Type: "function",
			Function: oaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	req := oaiRequest{
		Model:     params.Model,
		Messages:  msgs,
		Tools:     tools,
		MaxTokens: params.MaxTokens,
	}
	if len(tools) > 0 {
		req.ToolChoice = "auto"
	}

	resp, err := p.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	choice := resp.Choices[0]
	result := &ChatResponse{
		Usage: Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}

	// Determine stop reason
	if choice.FinishReason == "tool_calls" {
		result.StopReason = "tool_use"
	} else {
		result.StopReason = "end_turn"
	}

	// Extract text content
	if text := extractText(choice.Message.Content); text != "" {
		result.Content = append(result.Content, ContentBlock{
			Type: "text",
			Text: text,
		})
	}

	// Extract tool calls
	for _, tc := range choice.Message.ToolCalls {
		result.Content = append(result.Content, ContentBlock{
			Type:     "tool_use",
			ToolName: tc.Function.Name,
			ToolID:   tc.ID,
			Input:    json.RawMessage(tc.Function.Arguments),
		})
	}

	return result, nil
}

// convertMessage translates a provider.Message into one or more oaiMessages.
// An assistant message with tool_use blocks becomes a single message with tool_calls.
// A user message with tool_result blocks becomes separate "tool" role messages,
// plus any text blocks as a user message.
func (p *OpenAIProvider) convertMessage(m Message) []oaiMessage {
	var result []oaiMessage

	if m.Role == "assistant" {
		msg := oaiMessage{Role: "assistant"}
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				msg.Content = b.Text
			case "tool_use":
				msg.ToolCalls = append(msg.ToolCalls, oaiToolCall{
					ID:   b.ToolID,
					Type: "function",
					Function: oaiToolCallFunc{
						Name:      b.ToolName,
						Arguments: string(b.Input),
					},
				})
			}
		}
		result = append(result, msg)
		return result
	}

	// User messages: separate tool results from text
	var textParts []string
	for _, b := range m.Content {
		switch b.Type {
		case "tool_result":
			result = append(result, oaiMessage{
				Role:       "tool",
				Content:    b.Text,
				ToolCallID: b.ToolID,
			})
		case "text":
			textParts = append(textParts, b.Text)
		}
	}
	if len(textParts) > 0 {
		result = append(result, oaiMessage{
			Role:    "user",
			Content: strings.Join(textParts, "\n"),
		})
	}

	return result
}

func (p *OpenAIProvider) doRequest(ctx context.Context, req oaiRequest) (*oaiResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	return WithRetry(ctx, func() (*oaiResponse, error) {
		resp, err := p.client.Do(httpReq.Clone(ctx))
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		if resp.StatusCode == 429 {
			return nil, &rateLimitError{status: 429}
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		var oaiResp oaiResponse
		if err := json.Unmarshal(respBody, &oaiResp); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}

		if oaiResp.Error != nil {
			return nil, fmt.Errorf("OpenAI error: %s", oaiResp.Error.Message)
		}

		return &oaiResp, nil
	}, isRateLimitError)
}

// extractText gets the text string from an oaiMessage Content field,
// which can be a string or a []oaiContentPart.
func extractText(content any) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	}
	// When unmarshaled from JSON, content might be a string or an array.
	// The json decoder returns string for string values.
	return ""
}
