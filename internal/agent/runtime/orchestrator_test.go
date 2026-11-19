package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
	agentcontext "github.com/hemanshulabs/gh-forge/v4/internal/agent/context"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/provider"
)

type orchestrationTestProvider struct {
	results  []*agent.AgentResult
	errors   []error
	requests []provider.CompletionRequest
}

func (p *orchestrationTestProvider) ID() string                       { return "test" }
func (p *orchestrationTestProvider) Name() string                     { return "test" }
func (p *orchestrationTestProvider) Description() string              { return "test provider" }
func (p *orchestrationTestProvider) IsAvailable(context.Context) bool { return true }
func (p *orchestrationTestProvider) Complete(_ context.Context, req provider.CompletionRequest) (*agent.AgentResult, error) {
	p.requests = append(p.requests, req)
	i := len(p.requests) - 1
	if i < len(p.errors) && p.errors[i] != nil {
		return nil, p.errors[i]
	}
	if i >= len(p.results) {
		return nil, errors.New("unexpected provider call")
	}
	return p.results[i], nil
}

func finding(confidence float64) agent.Finding {
	return agent.Finding{Severity: agent.SeverityLow, Confidence: confidence, Explanation: "verified finding"}
}

func TestRuntimeRetriesOnceAndEnforcesConfidenceFloor(t *testing.T) {
	provider := &orchestrationTestProvider{results: []*agent.AgentResult{
		{},
		{Role: "Reviewer", Summary: "analysis complete", Findings: []agent.Finding{finding(0.74), finding(0.75)}},
	}}
	rt := New(provider, Config{Model: "chosen-model"})
	result, err := rt.Execute(context.Background(), agent.AgentConfig{Role: agent.RoleReviewer, SystemPrompt: "review"}, "input")
	require.NoError(t, err)
	require.Len(t, provider.requests, 2)
	require.Equal(t, "chosen-model", provider.requests[0].Model)
	require.Equal(t, 1, rt.cfg.MaxRetries)
	require.Equal(t, 0.75, rt.cfg.MinConfidence)
	require.Len(t, result.Findings, 1)
	require.Equal(t, 0.75, result.Findings[0].Confidence)
}

func TestRouteWorkersForIssueAndFailedSecurityPR(t *testing.T) {
	issue := routeWorkers(&agentcontext.AggregatedContext{TargetType: "issue"})
	require.Equal(t, []agent.AgentRole{agent.RoleTriage}, []agent.AgentRole{issue[0].Role})

	pr := routeWorkers(&agentcontext.AggregatedContext{
		TargetType: "pr",
		Diff:       "diff --git a/internal/auth.go b/internal/auth.go\n+token query",
		CIStatus:   "FAILED (unit-tests)",
	})
	require.Equal(t, []agent.AgentRole{agent.RoleReviewer, agent.RoleSentinel, agent.RoleDiagnostics}, []agent.AgentRole{pr[0].Role, pr[1].Role, pr[2].Role})
	require.Contains(t, pr[2].Reason, "failed")
}

func TestOrchestratorPreservesSuccessfulWorkersWhenDiagnosticsNeedsLogs(t *testing.T) {
	provider := &orchestrationTestProvider{results: []*agent.AgentResult{
		{Role: "Reviewer", Summary: "review completed", Findings: []agent.Finding{finding(0.90)}},
		{Role: "Sentinel", Summary: "security scan completed"},
	}}
	rt := New(provider, Config{})
	result, err := rt.Orchestrate(context.Background(), &agentcontext.AggregatedContext{
		TargetType: "pr",
		Diff:       "diff --git a/internal/auth.go b/internal/auth.go\n+token value",
		CIStatus:   "FAILURE",
	})
	require.NoError(t, err)
	require.Equal(t, "needs_input", result.Status)
	require.Len(t, provider.requests, 2)
	require.Len(t, result.Result.Findings, 1)
	require.Equal(t, "needs_input", result.Workers[2].Status)
	require.Contains(t, result.Workers[2].Error, "logs are not available")
}
