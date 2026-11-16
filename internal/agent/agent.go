package agent

import (
	"context"
)

// AgentRole defines the role identifier for a specialized agent.
type AgentRole string

const (
	RoleReviewer    AgentRole = "Reviewer"
	RoleTriage      AgentRole = "Triage"
	RoleSentinel    AgentRole = "Sentinel"
	RoleDiagnostics AgentRole = "Diagnostics"
)

// ToolName represents a scoped capability granted to an agent.
type ToolName string

const (
	ToolSuggestComment  ToolName = "suggest_comment"
	ToolSuggestLabel    ToolName = "suggest_label"
	ToolSuggestPriority ToolName = "suggest_priority"
	ToolSuggestClose    ToolName = "suggest_close"
)

// AgentConfig represents the declarative configuration of an agent role.
type AgentConfig struct {
	Role              AgentRole  `json:"role"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	Command           string     `json:"command"`
	SystemPrompt      string     `json:"system_prompt"`
	AllowedTools      []ToolName `json:"allowed_tools"`
	DefaultConfidence float64    `json:"default_confidence"`
}

// HasTool checks if a tool is permitted for this agent.
func (c AgentConfig) HasTool(tool ToolName) bool {
	for _, t := range c.AllowedTools {
		if t == tool {
			return true
		}
	}
	return false
}

// Agent represents an executable specialized agent.
type Agent interface {
	Config() AgentConfig
	Run(ctx context.Context, target string, opts RunOptions) (*AgentResult, error)
}

// RunOptions specifies user flags and runtime controls for an agent execution.
type RunOptions struct {
	ConfidenceThreshold float64
	TokenBudget         int
	ProviderOverride    string
	ModelOverride       string
	Format              string // "pretty", "json", "markdown"
	DryRun              bool
	ApplyAction         bool // explicit opt-in for write action (e.g. comment or label)
}
