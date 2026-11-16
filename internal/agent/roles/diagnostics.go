package roles

import (
	"github.com/dlvhdr/gh-dash/v4/internal/agent"
)

// DiagnosticsSystemPrompt defines the role instructions for the upcoming Diagnostics agent.
const DiagnosticsSystemPrompt = `You are the Diagnostics Agent for Forge.
Your role is to diagnose what is broken NOW in production or CI by analyzing stack traces, error logs, git blame, and recent commits to identify the root cause.`

// NewDiagnosticsConfig returns the configuration for the Diagnostics agent (scheduled for subsequent release).
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
