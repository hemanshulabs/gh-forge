package agent_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/agent"
)

func TestParseAgentResult_ValidJSON(t *testing.T) {
	raw := `{
		"role": "Reviewer",
		"summary": "The PR introduces a new auth handler with good structure.",
		"findings": [
			{
				"severity": "high",
				"confidence": 0.95,
				"file": "auth/token.go",
				"line": 42,
				"explanation": "Missing expiration check on JWT verification.",
				"suggested_fix": "if claims.ExpiresAt < time.Now().Unix() { return ErrExpired }"
			},
			{
				"severity": "low",
				"confidence": 0.80,
				"file": "auth/token.go",
				"line": 15,
				"explanation": "Consider renaming helper function for clarity."
			}
		]
	}`

	res, err := agent.ParseAgentResult(raw)
	require.NoError(t, err)
	require.Equal(t, "Reviewer", res.Role)
	require.Len(t, res.Findings, 2)
	require.Equal(t, agent.SeverityHigh, res.Findings[0].Severity)
	require.Equal(t, 0.95, res.Findings[0].Confidence)
	require.Equal(t, 42, res.Findings[0].Line)
}

func TestParseAgentResult_MarkdownWrapped(t *testing.T) {
	raw := "```json\n" + `{
		"role": "Triage",
		"summary": "Bug report lacks reproduction environment.",
		"findings": [
			{
				"severity": "medium",
				"confidence": 0.85,
				"file": "",
				"line": 0,
				"explanation": "No OS or Go version specified in the issue body."
			}
		]
	}` + "\n```"

	res, err := agent.ParseAgentResult(raw)
	require.NoError(t, err)
	require.Equal(t, "Triage", res.Role)
	require.Len(t, res.Findings, 1)
}

func TestParseAgentResult_InvalidSeverity(t *testing.T) {
	raw := `{
		"role": "Sentinel",
		"summary": "Check",
		"findings": [
			{
				"severity": "apocalyptic",
				"confidence": 0.9,
				"file": "main.go",
				"line": 1,
				"explanation": "Invalid severity test"
			}
		]
	}`

	_, err := agent.ParseAgentResult(raw)
	require.Error(t, err)
}

func TestParseAgentResult_InvalidConfidence(t *testing.T) {
	raw := `{
		"role": "Sentinel",
		"summary": "Check",
		"findings": [
			{
				"severity": "high",
				"confidence": 1.5,
				"file": "main.go",
				"line": 1,
				"explanation": "Out of range confidence"
			}
		]
	}`

	_, err := agent.ParseAgentResult(raw)
	require.Error(t, err)
}
