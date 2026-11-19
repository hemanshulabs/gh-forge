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

	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
)

// OllamaAdapter implements local offline model execution for Private Mode.
type OllamaAdapter struct {
	baseURL string
	client  *http.Client
}

func NewOllamaAdapter() *OllamaAdapter {
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &OllamaAdapter{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OllamaAdapter) ID() string {
	return "ollama"
}

func (o *OllamaAdapter) Name() string {
	return "Local Ollama (Private Mode)"
}

func (o *OllamaAdapter) Description() string {
	return fmt.Sprintf("Local model execution via Ollama at %s. No data leaves the machine.", o.baseURL)
}

func (o *OllamaAdapter) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (o *OllamaAdapter) Complete(ctx context.Context, req CompletionRequest) (*agent.AgentResult, error) {
	model := req.Model
	if model == "" {
		model = os.Getenv("OLLAMA_MODEL")
		if model == "" {
			model = "llama3.2"
		}
	}

	systemInstruction := req.SystemPrompt + "\n\nCRITICAL: Output ONLY a single JSON object conforming strictly to this schema:\n" + req.JSONSchema

	payload := map[string]any{
		"model":  model,
		"prompt": req.UserPrompt,
		"system": systemInstruction,
		"stream": false,
		"format": "json",
		"options": map[string]any{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local Ollama daemon: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama error (%d): %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode ollama response: %w", err)
	}

	result, err := agent.ParseAgentResult(ollamaResp.Response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ollama JSON output: %w (output: %s)", err, ollamaResp.Response)
	}
	result.RawResult = ollamaResp.Response
	if result.Role == "" {
		result.Role = req.Role
	}

	return result, nil
}
