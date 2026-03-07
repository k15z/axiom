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

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// GeminiProvider calls the Google Gemini API using raw HTTP.
type GeminiProvider struct {
	apiKey string
	client *http.Client
}

// NewGemini creates a provider backed by the Gemini API.
func NewGemini(apiKey string) *GeminiProvider {
	return &GeminiProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

// Gemini request/response types for the generateContent endpoint.

type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	Tools            []geminiToolDecl        `json:"tools,omitempty"`
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall   `json:"functionCall,omitempty"`
	FunctionResponse *geminiFuncResponse   `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type geminiFuncResponse struct {
	Name     string         `json:"name"`
	Response geminiRespData `json:"response"`
}

type geminiRespData struct {
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

type geminiToolDecl struct {
	FunctionDeclarations []geminiFunc `json:"functionDeclarations"`
}

type geminiFunc struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsage  `json:"usageMetadata,omitempty"`
	Error      *geminiError     `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (p *GeminiProvider) Chat(ctx context.Context, params ChatParams) (*ChatResponse, error) {
	// Build contents
	var contents []geminiContent

	for _, m := range params.Messages {
		c := p.convertMessage(m)
		contents = append(contents, c...)
	}

	// Build tools
	var tools []geminiToolDecl
	if len(params.Tools) > 0 {
		var funcs []geminiFunc
		for _, t := range params.Tools {
			funcs = append(funcs, geminiFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		tools = append(tools, geminiToolDecl{FunctionDeclarations: funcs})
	}

	req := geminiRequest{
		Contents: contents,
		Tools:    tools,
		GenerationConfig: &geminiGenerationConfig{
			MaxOutputTokens: params.MaxTokens,
		},
	}

	if params.System != "" {
		req.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: params.System}},
		}
	}

	resp, err := p.doRequest(ctx, params.Model, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("Gemini returned no candidates")
	}

	cand := resp.Candidates[0]
	result := &ChatResponse{
		StopReason: "end_turn",
	}

	if resp.UsageMetadata != nil {
		result.Usage = Usage{
			InputTokens:  resp.UsageMetadata.PromptTokenCount,
			OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		}
	}

	for _, part := range cand.Content.Parts {
		if part.FunctionCall != nil {
			result.StopReason = "tool_use"
			// Gemini doesn't return tool IDs; we generate one
			toolID := fmt.Sprintf("call_%s_%d", part.FunctionCall.Name, len(result.Content))
			result.Content = append(result.Content, ContentBlock{
				Type:     "tool_use",
				ToolName: part.FunctionCall.Name,
				ToolID:   toolID,
				Input:    part.FunctionCall.Args,
			})
		} else if part.Text != "" {
			result.Content = append(result.Content, ContentBlock{
				Type: "text",
				Text: part.Text,
			})
		}
	}

	return result, nil
}

func (p *GeminiProvider) convertMessage(m Message) []geminiContent {
	// Gemini uses "user" and "model" roles
	role := m.Role
	if role == "assistant" {
		role = "model"
	}

	var parts []geminiPart
	var toolResultParts []geminiPart

	for _, b := range m.Content {
		switch b.Type {
		case "text":
			parts = append(parts, geminiPart{Text: b.Text})
		case "tool_use":
			parts = append(parts, geminiPart{
				FunctionCall: &geminiFunctionCall{
					Name: b.ToolName,
					Args: b.Input,
				},
			})
		case "tool_result":
			toolResultParts = append(toolResultParts, geminiPart{
				FunctionResponse: &geminiFuncResponse{
					Name: b.ToolName,
					Response: geminiRespData{
						Content: b.Text,
						IsError: b.IsError,
					},
				},
			})
		}
	}

	var result []geminiContent
	if len(parts) > 0 {
		result = append(result, geminiContent{Role: role, Parts: parts})
	}
	if len(toolResultParts) > 0 {
		// Tool results are sent as "user" role in Gemini
		result = append(result, geminiContent{Role: "user", Parts: toolResultParts})
	}
	return result
}

func (p *GeminiProvider) doRequest(ctx context.Context, model string, req geminiRequest) (*geminiResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", geminiBaseURL, model, p.apiKey)

	delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second}
	for attempt, maxAttempts := 0, len(delays)+1; attempt < maxAttempts; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		if resp.StatusCode == 429 && attempt < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delays[attempt]):
				continue
			}
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		var gemResp geminiResponse
		if err := json.Unmarshal(respBody, &gemResp); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}

		if gemResp.Error != nil {
			if gemResp.Error.Code == 429 && attempt < maxAttempts-1 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delays[attempt]):
					continue
				}
			}
			return nil, fmt.Errorf("Gemini error: %s", gemResp.Error.Message)
		}

		return &gemResp, nil
	}
	return nil, fmt.Errorf("unreachable")
}

// stripGooglePrefix removes "google/" prefix from model names for the Gemini API URL.
func stripGooglePrefix(model string) string {
	return strings.TrimPrefix(model, "google/")
}
