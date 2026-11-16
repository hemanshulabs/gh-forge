package eval_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/agent"
	"github.com/dlvhdr/gh-dash/v4/internal/agent/provider"
	"github.com/dlvhdr/gh-dash/v4/internal/agent/roles"
	"github.com/dlvhdr/gh-dash/v4/internal/agent/runtime"
)

// MockProvider simulates a model returning a pre-configured response for evaluation.
type MockProvider struct {
	Response string
}

func (m *MockProvider) ID() string          { return "mock" }
func (m *MockProvider) Name() string        { return "Mock Provider" }
func (m *MockProvider) Description() string { return "Mock provider for eval tests" }
func (m *MockProvider) IsAvailable(ctx context.Context) bool {
	return true
}

func (m *MockProvider) Complete(ctx context.Context, req provider.CompletionRequest) (*agent.AgentResult, error) {
	return agent.ParseAgentResult(m.Response)
}

func TestEval_ReviewerKnownGoodDiff(t *testing.T) {
	mockResponse := `{
		"role": "Reviewer",
		"summary": "PR adds new token verification endpoint with high test coverage.",
		"findings": [
			{
				"severity": "medium",
				"confidence": 0.85,
				"file": "auth/service.go",
				"line": 45,
				"explanation": "Consider handling context cancellation during signature verification.",
				"suggested_fix": "select { case <-ctx.Done(): return ctx.Err() default: }"
			}
		]
	}`

	mock := &MockProvider{Response: mockResponse}
	rt := runtime.New(mock, runtime.Config{
		MinConfidence: 0.70,
	})

	cfg := roles.NewReviewerConfig()
	result, err := rt.Execute(context.Background(), cfg, "PR diff test")
	require.NoError(t, err)
	require.Equal(t, "Reviewer", result.Role)
	require.Len(t, result.Findings, 1)
	require.Equal(t, 45, result.Findings[0].Line)
	require.Equal(t, agent.SeverityMedium, result.Findings[0].Severity)
}

func TestEval_SentinelKnownExploit(t *testing.T) {
	mockResponse := `{
		"role": "Sentinel",
		"summary": "Detected critical SQL injection risk in query construction.",
		"findings": [
			{
				"severity": "critical",
				"confidence": 0.98,
				"file": "db/query.go",
				"line": 102,
				"explanation": "Direct string interpolation into raw SQL query without parameterized arguments.",
				"suggested_fix": "db.QueryRowContext(ctx, \"SELECT * FROM users WHERE id = $1\", userID)"
			}
		]
	}`

	mock := &MockProvider{Response: mockResponse}
	rt := runtime.New(mock, runtime.Config{
		MinConfidence: 0.80,
	})

	cfg := roles.NewSentinelConfig()
	result, err := rt.Execute(context.Background(), cfg, "Security diff test")
	require.NoError(t, err)
	require.Equal(t, "Sentinel", result.Role)
	require.Len(t, result.Findings, 1)
	require.Equal(t, agent.SeverityCritical, result.Findings[0].Severity)
	require.Equal(t, 0.98, result.Findings[0].Confidence)
}
