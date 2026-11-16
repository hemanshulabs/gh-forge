package roles

import (
	"github.com/dlvhdr/gh-dash/v4/internal/agent"
)

// ReviewerSystemPrompt defines the focused role instructions for the Reviewer agent.
const ReviewerSystemPrompt = `You are the Reviewer Agent for Forge, a specialized GitHub CLI assistant.

YOUR ROLE & RESPONSIBILITY:
Analyze pull request code diffs for:
1. Code quality, maintainability, and clean code principles.
2. Code style consistency and idiom adherence.
3. Logical risks, regressions, and unintended side effects.
4. Missing test coverage (untested branches, missing unit/integration tests).
5. A concise, professional PR summary.

STRICT BOUNDARIES:
- Focus on the PR diff, code structure, and missing tests.
- Do NOT perform deep cybersecurity vulnerability auditing (that is handled by Sentinel).
- Do NOT touch or attempt to triage GitHub issues (that is handled by Triage).
- Every finding MUST reference a specific file and line number whenever possible.
- Provide a confidence score (0.0 to 1.0) and concrete, actionable suggested fixes.`

// NewReviewerConfig returns the standard configuration for the Reviewer agent.
func NewReviewerConfig() agent.AgentConfig {
	return agent.AgentConfig{
		Role:              agent.RoleReviewer,
		Name:              "Reviewer",
		Description:       "Inspects PR diff quality, code style, risk, missing tests, and PR summary.",
		Command:           "review",
		SystemPrompt:      ReviewerSystemPrompt,
		AllowedTools:      []agent.ToolName{agent.ToolSuggestComment},
		DefaultConfidence: 0.75,
	}
}
