package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// AnthropicProvider calls the Anthropic Messages API with streaming.
type AnthropicProvider struct {
	client   anthropic.Client
	progress ProgressFunc
}

// NewAnthropic creates a provider backed by the Anthropic API.
func NewAnthropic(apiKey string, progress ProgressFunc) *AnthropicProvider {
	return &AnthropicProvider{
		client:   anthropic.NewClient(option.WithAPIKey(apiKey)),
		progress: progress,
	}
}

func (p *AnthropicProvider) Chat(ctx context.Context, params ChatParams) (*ChatResponse, error) {
	// Convert tools
	tools := make([]anthropic.ToolUnionParam, len(params.Tools))
	for i, t := range params.Tools {
		var schema anthropic.ToolInputSchemaParam
		if err := json.Unmarshal(t.InputSchema, &schema); err != nil {
			return nil, fmt.Errorf("invalid tool schema for %s: %w", t.Name, err)
		}
		tools[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: schema,
			},
		}
	}

	// Convert messages
	msgs := make([]anthropic.MessageParam, len(params.Messages))
	for i, m := range params.Messages {
		blocks := make([]anthropic.ContentBlockParamUnion, len(m.Content))
		for j, b := range m.Content {
			switch b.Type {
			case "text":
				blocks[j] = anthropic.NewTextBlock(b.Text)
			case "tool_use":
				// Parse Input back to any for the Anthropic SDK
				var inputAny any
				if len(b.Input) > 0 {
					_ = json.Unmarshal(b.Input, &inputAny)
				}
				blocks[j] = anthropic.NewToolUseBlock(b.ToolID, inputAny, b.ToolName)
			case "tool_result":
				blocks[j] = anthropic.NewToolResultBlock(b.ToolID, b.Text, b.IsError)
			}
		}
		switch m.Role {
		case "user":
			msgs[i] = anthropic.NewUserMessage(blocks...)
		case "assistant":
			msgs[i] = anthropic.NewAssistantMessage(blocks...)
		}
	}

	apiParams := anthropic.MessageNewParams{
		Model:     anthropic.Model(params.Model),
		MaxTokens: int64(params.MaxTokens),
		System:    []anthropic.TextBlockParam{{Text: params.System}},
		Tools:     tools,
		Messages:  msgs,
	}

	resp, err := p.streamWithRetry(ctx, apiParams)
	if err != nil {
		return nil, err
	}

	// Convert response
	result := &ChatResponse{
		Usage: Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		},
	}

	switch resp.StopReason {
	case "tool_use":
		result.StopReason = "tool_use"
	default:
		result.StopReason = "end_turn"
	}

	for _, block := range resp.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			result.Content = append(result.Content, ContentBlock{
				Type: "text",
				Text: v.Text,
			})
		case anthropic.ToolUseBlock:
			raw, _ := json.Marshal(v.Input)
			result.Content = append(result.Content, ContentBlock{
				Type:     "tool_use",
				ToolName: v.Name,
				ToolID:   v.ID,
				Input:    raw,
			})
		}
	}

	return result, nil
}

// consumeStream reads all events from a streaming response.
func (p *AnthropicProvider) consumeStream(stream *ssestream.Stream[anthropic.MessageStreamEventUnion]) (*anthropic.Message, error) {
	defer stream.Close()

	var msg anthropic.Message
	for stream.Next() {
		event := stream.Current()
		if err := msg.Accumulate(event); err != nil {
			return nil, fmt.Errorf("stream accumulate: %w", err)
		}
		if p.progress != nil {
			if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
				if td, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok && td.Text != "" {
					p.progress(td.Text)
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	return &msg, nil
}

// streamWithRetry opens a streaming API call with retry on 429 rate limit errors.
func (p *AnthropicProvider) streamWithRetry(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	return WithRetry(ctx, func() (*anthropic.Message, error) {
		stream := p.client.Messages.NewStreaming(ctx, params)
		return p.consumeStream(stream)
	}, func(err error) bool {
		s := err.Error()
		return strings.Contains(s, "429") || strings.Contains(s, "rate_limit_error")
	})
}
