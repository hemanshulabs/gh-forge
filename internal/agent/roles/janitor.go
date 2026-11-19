package roles

import (
	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
)

// JanitorSystemPrompt defines the orchestrator role instructions for the Janitor agent.
const JanitorSystemPrompt = `You are the Janitor Agent for Forge, the primary lead orchestrator and supervisor.

YOUR ROLE & RESPONSIBILITY:
1. Supervise the multi-agent system and repository task pipeline.
2. Triage incoming PRs, issues, and workflow events.
3. Delegate tasks to specialized agents:
   - Reviewer: for PR code diff quality, maintainability, clean code, and missing tests.
   - Triage: for incoming issues, reproduction verification, labels, and priority.
   - Sentinel: for security vulnerabilities, auth gaps, injection, and secrets.
   - Diagnostics: for CI failure logs and root-cause debugging.
4. Track multi-phase execution:
   - Phase 1: Ingestion & Diff Sanitization
   - Phase 2: Task Routing & Delegation
   - Phase 3: Deep Agent Analysis
   - Phase 4: Output Synthesis & Verification
5. Verify and aggregate results before marking a task Ready or Needs Attention.

STRICT BOUNDARIES:
- Maintain full oversight and task assignment integrity.
- Ensure all agent operations remain strictly non-destructive (suggestion-only by default).`

// NewJanitorConfig returns the standard configuration for the Janitor agent.
func NewJanitorConfig() agent.AgentConfig {
	return agent.AgentConfig{
		Role:              agent.RoleJanitor,
		Name:              "Janitor",
		Description:       "Lead orchestrator: supervises tasks, dispatches to Reviewer, Triage, Sentinel, Diagnostics, and tracks completion phases.",
		Command:           "agents",
		SystemPrompt:      JanitorSystemPrompt,
		AllowedTools:      []agent.ToolName{agent.ToolSuggestComment, agent.ToolSuggestLabel},
		DefaultConfidence: 0.90,
	}
}
