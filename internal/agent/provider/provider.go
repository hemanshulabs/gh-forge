package provider

import (
	"context"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
)

// CompletionRequest defines the input to a model provider.
type CompletionRequest struct {
	Role         string
	SystemPrompt string
	UserPrompt   string
	JSONSchema   string
	MaxTokens    int
	Temperature  float64
	Model        string
}

// Provider is the common interface implemented by all AI backends.
// Agents interact solely through this interface; they never know which backend runs them.
type Provider interface {
	ID() string
	Name() string
	Description() string
	IsAvailable(ctx context.Context) bool
	Complete(ctx context.Context, req CompletionRequest) (*agent.AgentResult, error)
}

// ProviderInfo provides status and discovery details for a provider.
type ProviderInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Reason      string `json:"reason,omitempty"`
}
