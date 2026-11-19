package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
	agentcontext "github.com/hemanshulabs/gh-forge/v4/internal/agent/context"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/roles"
)

// WorkerAssignment records why a worker was selected and what happened when it ran.
type WorkerAssignment struct {
	Role   agent.AgentRole    `json:"role"`
	Reason string             `json:"reason"`
	Status string             `json:"status"`
	Result *agent.AgentResult `json:"result,omitempty"`
	Error  string             `json:"error,omitempty"`
}

// OrchestrationResult is the deterministic routing trace and worker output for one target.
type OrchestrationResult struct {
	Status  string             `json:"status"`
	Phases  []string           `json:"phases"`
	Workers []WorkerAssignment `json:"workers"`
	Result  agent.AgentResult  `json:"result"`
}

// Orchestrate routes loaded context to the applicable workers. Janitor performs no model-generated
// actions: routing and synthesis are deterministic, and workers only return validated suggestions.
func (r *Runtime) Orchestrate(ctx context.Context, input *agentcontext.AggregatedContext) (*OrchestrationResult, error) {
	if input == nil {
		return nil, fmt.Errorf("cannot orchestrate without loaded target context")
	}
	assignments := routeWorkers(input)
	out := &OrchestrationResult{
		Status:  "complete",
		Phases:  []string{"context_ready", "routing_complete"},
		Workers: make([]WorkerAssignment, 0, len(assignments)),
		Result:  agent.AgentResult{Role: string(agent.RoleJanitor), Findings: []agent.Finding{}},
	}
	var summaries []string
	var findings []agent.Finding
	for _, assignment := range assignments {
		missingInput := ""
		if (assignment.Role == agent.RoleReviewer || assignment.Role == agent.RoleSentinel) && !hasUsableDiff(input.Diff) {
			missingInput = "a pull request diff is not available; code-level findings cannot be verified"
			if input.DiffLoadError != "" {
				missingInput += ". Diff fetch detail: " + input.DiffLoadError
			}
			out.Status = "needs_input"
			out.Phases = append(out.Phases, string(assignment.Role)+"_limited_missing_diff")
		}
		if assignment.Role == agent.RoleDiagnostics && strings.TrimSpace(input.CILogs) == "" {
			missingInput = "failed-check logs are not available; Diagnostics can only assess the reported CI status"
			out.Status = "needs_input"
			out.Phases = append(out.Phases, string(assignment.Role)+"_limited_missing_logs")
		}
		out.Phases = append(out.Phases, string(assignment.Role)+"_started")
		workerCfg, prompt, chunkDiff := workerInput(input, assignment.Role)
		if missingInput != "" {
			prompt += "\n\nEVIDENCE LIMITATION:\n" + missingInput + ". Analyze only the supplied metadata, do not infer code or log details, and state what evidence is needed."
			chunkDiff = false
		}
		var result *agent.AgentResult
		var err error
		if chunkDiff {
			chunkBudget := r.DiffChunkBudget(workerCfg, prompt)
			if chunkBudget <= 0 {
				err = fmt.Errorf("agent instructions and task metadata exceed the configured token budget")
			} else {
				result, err = r.ChunkedExecute(ctx, workerCfg, prompt, agentcontext.ChunkDiff(input.Diff, chunkBudget))
			}
		} else {
			result, err = r.Execute(ctx, workerCfg, prompt)
		}
		if err != nil {
			assignment.Status = "failed"
			assignment.Error = err.Error()
			out.Phases = append(out.Phases, string(assignment.Role)+"_failed")
			if out.Status == "complete" {
				out.Status = "partial"
			}
		} else {
			assignment.Status = "complete"
			if missingInput != "" {
				assignment.Status = "needs_input"
				assignment.Error = missingInput
			}
			out.Phases = append(out.Phases, string(assignment.Role)+"_completed")
			assignment.Result = result
			if len(result.Warnings) > 0 {
				if missingInput != "" {
					assignment.Status = "needs_input"
					assignment.Error = missingInput + "; " + strings.Join(result.Warnings, "; ")
					out.Status = "needs_input"
				} else {
					assignment.Status = "partial"
					assignment.Error = strings.Join(result.Warnings, "; ")
					if out.Status == "complete" {
						out.Status = "partial"
					}
				}
			}
			if result.Summary != "" {
				summaries = append(summaries, string(assignment.Role)+": "+result.Summary)
			}
			findings = append(findings, result.Findings...)
		}
		out.Workers = append(out.Workers, assignment)
	}
	out.Result.Summary = strings.Join(summaries, "\n")
	out.Result.Findings = SortFindings(FilterFindings(DeduplicateFindings(findings), r.cfg.MinConfidence))
	if len(summaries) == 0 && out.Status == "complete" {
		out.Status = "needs_input"
	}
	out.Phases = append(out.Phases, "synthesis_complete")
	return out, nil
}

func hasUsableDiff(diff string) bool {
	trimmed := strings.TrimSpace(diff)
	return trimmed != "" && !strings.Contains(trimmed, "[Unable to load diff automatically]")
}

func routeWorkers(input *agentcontext.AggregatedContext) []WorkerAssignment {
	if strings.EqualFold(input.TargetType, "issue") {
		return []WorkerAssignment{{Role: agent.RoleTriage, Reason: "target is a GitHub issue"}}
	}
	workers := []WorkerAssignment{{Role: agent.RoleReviewer, Reason: "pull request or code diff analysis"}}
	if securityRelevant(input.Diff) {
		workers = append(workers, WorkerAssignment{Role: agent.RoleSentinel, Reason: "changed paths or diff content match security-sensitive signals"})
	}
	status := strings.ToUpper(input.CIStatus)
	if strings.Contains(status, "FAIL") || strings.Contains(status, "ACTION_REQUIRED") {
		workers = append(workers, WorkerAssignment{Role: agent.RoleDiagnostics, Reason: "CI status reports a failed or action-required check"})
	}
	return workers
}

func securityRelevant(diff string) bool {
	text := strings.ToLower(diff)
	for _, signal := range []string{
		"auth", "login", "permission", "credential", "secret", "token", "password", "crypto",
		"sql", "query", "exec.command", "http://", "https://", "deserialize", "upload", "path",
	} {
		if strings.Contains(text, signal) {
			return true
		}
	}
	return false
}

func workerInput(input *agentcontext.AggregatedContext, role agent.AgentRole) (agent.AgentConfig, string, bool) {
	switch role {
	case agent.RoleReviewer:
		return roles.NewReviewerConfig(), formatPRMetadataPrompt(input), true
	case agent.RoleSentinel:
		return roles.NewSentinelConfig(), formatPRMetadataPrompt(input), true
	case agent.RoleTriage:
		return roles.NewTriageConfig(), agentcontext.FormatIssuePrompt(input, input.AvailableLabels), false
	case agent.RoleDiagnostics:
		prompt := fmt.Sprintf("CI STATUS: %s\n\nFAILED CHECK LOGS:\n%s\n\nRECENT COMMITS:\n%s", input.CIStatus, input.CILogs, strings.Join(input.RecentCommits, "\n"))
		return roles.NewDiagnosticsConfig(), prompt, false
	default:
		return agent.AgentConfig{}, "", false
	}
}

// formatPRMetadataPrompt deliberately omits the diff. Diff chunks are appended
// one at a time by ChunkedExecute, avoiding the former full-diff duplication.
func formatPRMetadataPrompt(input *agentcontext.AggregatedContext) string {
	withoutDiff := *input
	withoutDiff.Diff = ""
	return agentcontext.FormatPRPrompt(&withoutDiff)
}
