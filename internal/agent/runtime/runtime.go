package runtime

import (
	"context"
	"fmt"
	"strings"

	"charm.land/log/v2"
	"github.com/dlvhdr/gh-dash/v4/internal/agent"
	"github.com/dlvhdr/gh-dash/v4/internal/agent/provider"
)

// DefaultTokenBudget is the standard budget for prompt + completion tokens.
const DefaultTokenBudget = 32000

// Config configures runtime execution options.
type Config struct {
	MinConfidence float64
	TokenBudget   int
	MaxRetries    int
	DryRun        bool
}

// Runtime is the shared execution engine powering all specialized agents.
type Runtime struct {
	provider provider.Provider
	cfg      Config
	executor *SafeExecutor
}

// New creates a new agent runtime using a chosen provider.
func New(p provider.Provider, cfg Config) *Runtime {
	if cfg.MinConfidence <= 0 {
		cfg.MinConfidence = 0.70
	}
	if cfg.TokenBudget <= 0 {
		cfg.TokenBudget = DefaultTokenBudget
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 2
	}
	return &Runtime{
		provider: p,
		cfg:      cfg,
		executor: NewSafeExecutor(cfg.DryRun),
	}
}

// Provider returns the active provider.
func (r *Runtime) Provider() provider.Provider {
	return r.provider
}

// SafeExecutor returns the write safety executor.
func (r *Runtime) Executor() *SafeExecutor {
	return r.executor
}

// Execute runs the core agent loop:
// 1. Assembles prompt and rigid JSON schema
// 2. Invokes provider.Complete
// 3. Validates output against schema
// 4. Retries on invalid output (up to MaxRetries)
// 5. Filters by confidence and deduplicates findings
func (r *Runtime) Execute(ctx context.Context, agentConfig agent.AgentConfig, userPrompt string) (*agent.AgentResult, error) {
	req := provider.CompletionRequest{
		Role:         string(agentConfig.Role),
		SystemPrompt: agentConfig.SystemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   agent.JSONSchemaSpecification,
		MaxTokens:    4096,
		Temperature:  0.2, // low temperature for deterministic, structured findings
	}

	var lastErr error
	var result *agent.AgentResult

	currentPrompt := userPrompt
	for attempt := 0; attempt <= r.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Warn("Retrying agent completion due to validation error", "attempt", attempt, "error", lastErr)
			req.UserPrompt = currentPrompt + fmt.Sprintf("\n\nPREVIOUS RESPONSE FAILED VALIDATION:\n%v\nPLEASE OUTPUT STRICT, VALID JSON ONLY.", lastErr)
		}

		log.Debug("Invoking agent provider", "provider", r.provider.ID(), "role", agentConfig.Role, "attempt", attempt)
		res, err := r.provider.Complete(ctx, req)
		if err != nil {
			lastErr = err
			continue
		}

		if err := res.Validate(); err != nil {
			lastErr = err
			continue
		}

		result = res
		break
	}

	if result == nil {
		return nil, fmt.Errorf("agent execution failed after %d attempts: %w", r.cfg.MaxRetries+1, lastErr)
	}

	// Post-processing: deduplication, filtering, sorting
	result.Findings = DeduplicateFindings(result.Findings)
	result.Findings = FilterFindings(result.Findings, r.cfg.MinConfidence)
	result.Findings = SortFindings(result.Findings)
	result.Role = string(agentConfig.Role)

	return result, nil
}

// ChunkedExecute executes across multiple chunks of large context (e.g. diffs) and merges the findings.
func (r *Runtime) ChunkedExecute(ctx context.Context, agentConfig agent.AgentConfig, basePrompt string, chunks []string) (*agent.AgentResult, error) {
	if len(chunks) <= 1 {
		fullPrompt := basePrompt
		if len(chunks) == 1 {
			fullPrompt += "\n\n" + chunks[0]
		}
		return r.Execute(ctx, agentConfig, fullPrompt)
	}

	log.Info("Chunking large context across multiple completions", "chunks", len(chunks))
	merged := &agent.AgentResult{
		Role:     string(agentConfig.Role),
		Summary:  "",
		Findings: make([]agent.Finding, 0),
	}

	var summaries []string
	for i, chunk := range chunks {
		prompt := fmt.Sprintf("%s\n\n[Diff Chunk %d of %d]:\n%s", basePrompt, i+1, len(chunks), chunk)
		res, err := r.Execute(ctx, agentConfig, prompt)
		if err != nil {
			log.Warn("Error evaluating chunk", "chunk", i+1, "error", err)
			continue
		}
		if res.Summary != "" {
			summaries = append(summaries, res.Summary)
		}
		merged.Findings = append(merged.Findings, res.Findings...)
	}

	merged.Summary = strings.Join(summaries, " | ")
	merged.Findings = DeduplicateFindings(merged.Findings)
	merged.Findings = FilterFindings(merged.Findings, r.cfg.MinConfidence)
	merged.Findings = SortFindings(merged.Findings)

	return merged, nil
}
