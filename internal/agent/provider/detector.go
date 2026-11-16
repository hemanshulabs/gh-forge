package provider

import (
	"context"
	"fmt"
	"os"
)

// Registry manages available model providers and auto-detection.
type Registry struct {
	providers []Provider
}

// NewRegistry initializes the default registry with adapters ordered by requirement:
// 1. Existing CLI adapter (Claude Code headless mode)
// 2. Direct API adapter (Anthropic/OpenAI/Gemini keys)
// 3. Local model adapter (Ollama), opt-in private mode
func NewRegistry() *Registry {
	return &Registry{
		providers: []Provider{
			NewClaudeCLIAdapter(),
			NewDirectAPIAdapter(),
			NewOllamaAdapter(),
		},
	}
}

// Detect returns the requested provider if specified, or auto-detects the first available provider.
func (r *Registry) Detect(ctx context.Context, preferredID string, privateMode bool) (Provider, error) {
	if privateMode {
		ollama := NewOllamaAdapter()
		if ollama.IsAvailable(ctx) {
			return ollama, nil
		}
		return nil, fmt.Errorf("private mode requested, but Ollama is not running or accessible at %s", os.Getenv("OLLAMA_HOST"))
	}

	if preferredID != "" {
		for _, p := range r.providers {
			if p.ID() == preferredID {
				if !p.IsAvailable(ctx) {
					return nil, fmt.Errorf("requested provider %q is not currently available or configured", preferredID)
				}
				return p, nil
			}
		}
		return nil, fmt.Errorf("unknown provider %q; available providers: claude-cli, direct-api, ollama", preferredID)
	}

	// Auto-detect based on priority
	for _, p := range r.providers {
		if p.IsAvailable(ctx) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no AI backend detected. Please install 'claude' CLI, set ANTHROPIC_API_KEY/OPENAI_API_KEY/GEMINI_API_KEY, or run Ollama locally")
}

// ListProviders returns information about all registered providers and their availability.
func (r *Registry) ListProviders(ctx context.Context) []ProviderInfo {
	infos := make([]ProviderInfo, len(r.providers))
	for i, p := range r.providers {
		available := p.IsAvailable(ctx)
		reason := "Ready"
		if !available {
			switch p.ID() {
			case "claude-cli":
				reason = "'claude' binary not found in PATH"
			case "direct-api":
				reason = "Missing API keys (ANTHROPIC_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY)"
			case "ollama":
				reason = "Ollama daemon not responding at localhost:11434"
			}
		}
		infos[i] = ProviderInfo{
			ID:          p.ID(),
			Name:        p.Name(),
			Description: p.Description(),
			Available:   available,
			Reason:      reason,
		}
	}
	return infos
}
