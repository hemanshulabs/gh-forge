package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dlvhdr/gh-dash/v4/internal/agent"
)

// DirectAPIAdapter supports direct HTTP completions to Anthropic, OpenAI, or Gemini.
type DirectAPIAdapter struct {
	client *http.Client
}

func NewDirectAPIAdapter() *DirectAPIAdapter {
	return &DirectAPIAdapter{
		client: &http.Client{Timeout: 90 * time.Second},
	}
}

func (a *DirectAPIAdapter) ID() string {
	return "direct-api"
}

func (a *DirectAPIAdapter) Name() string {
	return "Direct API (Anthropic / OpenAI / Gemini)"
}

func (a *DirectAPIAdapter) Description() string {
	return "Direct HTTP calls using ANTHROPIC_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY."
}

func (a *DirectAPIAdapter) IsAvailable(ctx context.Context) bool {
	return os.Getenv("ANTHROPIC_API_KEY") != "" ||
		os.Getenv("OPENAI_API_KEY") != "" ||
		os.Getenv("GEMINI_API_KEY") != ""
}

func (a *DirectAPIAdapter) Complete(ctx context.Context, req CompletionRequest) (*agent.AgentResult, error) {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return a.completeAnthropic(ctx, req)
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return a.completeOpenAI(ctx, req)
	}
	if os.Getenv("GEMINI_API_KEY") != "" {
		return a.completeGemini(ctx, req)
	}
	return nil, fmt.Errorf("no API key found: set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY")
}

func (a *DirectAPIAdapter) completeAnthropic(ctx context.Context, req CompletionRequest) (*agent.AgentResult, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	model := req.Model
	if model == "" {
		model = "claude-3-7-sonnet-20250219"
	}

	systemInstruction := req.SystemPrompt + "\n\nYou must return ONLY valid JSON matching this schema:\n" + req.JSONSchema

	payload := map[string]any{
		"model":      model,
		"max_tokens": 4096,
		"system":     systemInstruction,
		"messages": []map[string]string{
			{"role": "user", "content": req.UserPrompt},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, err
	}
	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("anthropic returned empty content")
	}

	rawText := anthropicResp.Content[0].Text
	result, err := agent.ParseAgentResult(rawText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse anthropic response as JSON: %w (raw: %s)", err, rawText)
	}
	result.RawResult = rawText
	if result.Role == "" {
		result.Role = req.Role
	}
	return result, nil
}

func (a *DirectAPIAdapter) completeOpenAI(ctx context.Context, req CompletionRequest) (*agent.AgentResult, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	systemInstruction := req.SystemPrompt + "\n\nYou must return ONLY valid JSON matching this schema:\n" + req.JSONSchema

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemInstruction},
			{"role": "user", "content": req.UserPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, err
	}
	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("openAI returned empty choices")
	}

	rawText := openaiResp.Choices[0].Message.Content
	result, err := agent.ParseAgentResult(rawText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse openAI response as JSON: %w", err)
	}
	result.RawResult = rawText
	if result.Role == "" {
		result.Role = req.Role
	}
	return result, nil
}

func (a *DirectAPIAdapter) completeGemini(ctx context.Context, req CompletionRequest) (*agent.AgentResult, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	model := req.Model
	if model == "" {
		model = "gemini-1.5-pro-latest"
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	systemInstruction := req.SystemPrompt + "\n\nYou must return ONLY valid JSON matching this schema:\n" + req.JSONSchema

	payload := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": systemInstruction}},
		},
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": req.UserPrompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, err
	}
	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned empty candidates")
	}

	rawText := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	result, err := agent.ParseAgentResult(rawText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gemini response as JSON: %w (raw: %s)", err, rawText)
	}
	result.RawResult = rawText
	if result.Role == "" {
		result.Role = req.Role
	}
	return result, nil
}
