package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/agent"
	"github.com/dlvhdr/gh-dash/v4/internal/agent/runtime"
)

func TestFilterFindings(t *testing.T) {
	findings := []agent.Finding{
		{Severity: agent.SeverityHigh, Confidence: 0.90, File: "a.go", Line: 10, Explanation: "Issue 1"},
		{Severity: agent.SeverityLow, Confidence: 0.50, File: "b.go", Line: 20, Explanation: "Issue 2"},
		{Severity: agent.SeverityMedium, Confidence: 0.75, File: "c.go", Line: 30, Explanation: "Issue 3"},
	}

	filtered := runtime.FilterFindings(findings, 0.70)
	require.Len(t, filtered, 2)
	require.Equal(t, "Issue 1", filtered[0].Explanation)
	require.Equal(t, "Issue 3", filtered[1].Explanation)
}

func TestDeduplicateFindings(t *testing.T) {
	findings := []agent.Finding{
		{Severity: agent.SeverityHigh, Confidence: 0.90, File: "a.go", Line: 10, Explanation: "Duplicate issue"},
		{Severity: agent.SeverityHigh, Confidence: 0.85, File: "a.go", Line: 10, Explanation: "duplicate issue"},
		{Severity: agent.SeverityMedium, Confidence: 0.75, File: "b.go", Line: 20, Explanation: "Different issue"},
	}

	unique := runtime.DeduplicateFindings(findings)
	require.Len(t, unique, 2)
}

func TestSortFindings(t *testing.T) {
	findings := []agent.Finding{
		{Severity: agent.SeverityLow, File: "a.go", Line: 10},
		{Severity: agent.SeverityCritical, File: "b.go", Line: 5},
		{Severity: agent.SeverityHigh, File: "a.go", Line: 2},
	}

	sorted := runtime.SortFindings(findings)
	require.Equal(t, agent.SeverityCritical, sorted[0].Severity)
	require.Equal(t, agent.SeverityHigh, sorted[1].Severity)
	require.Equal(t, agent.SeverityLow, sorted[2].Severity)
}
