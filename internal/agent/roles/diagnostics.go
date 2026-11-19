package roles

import (
	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
)

// DiagnosticsSystemPrompt defines the role instructions for the upcoming Diagnostics agent.
const DiagnosticsSystemPrompt = `You are the Diagnostics Agent for Forge.
Analyze only the supplied CI failure logs, stack traces, status, and commit context to identify a likely root cause.

STRICT BOUNDARIES:
- Do not execute shell commands, access files, call external services, or mutate project or GitHub state.
- Do not claim to have inspected logs, commits, or source files that were not included in the input.
- Separate observed evidence from hypotheses. If the supplied evidence is insufficient, say so and request the specific missing evidence.
- Return only findings that meet the shared structured result schema and confidence threshold.`

// NewDiagnosticsConfig returns the standard configuration for the Diagnostics agent.
func NewDiagnosticsConfig() agent.AgentConfig {
	return agent.AgentConfig{
		Role:              agent.RoleDiagnostics,
		Name:              "Diagnostics",
		Description:       "Determines the root cause of failures from stack traces, logs, git blame, and recent commits.",
		Command:           "diagnose",
		SystemPrompt:      DiagnosticsSystemPrompt,
		AllowedTools:      []agent.ToolName{agent.ToolSuggestComment},
		DefaultConfidence: 0.80,
	}
}
