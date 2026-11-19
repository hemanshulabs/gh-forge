package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
	agentcontext "github.com/hemanshulabs/gh-forge/v4/internal/agent/context"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/provider"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/roles"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/runtime"
)

// RunAssigned loads a single PR or issue and runs the requested read-only agent.
// It never applies findings or writes to GitHub or the checked-out repository.
func RunAssigned(ctx context.Context, repo, kind string, number int, role, providerID string) (string, string, error) {
	result, err := RunAssignedDetailed(ctx, repo, kind, number, role, providerID)
	return result.Summary, result.Status, err
}

type RunResult struct {
	Summary string
	Status  string
	Phases  []string
}

// RunAssignedDetailed includes an execution trace for the TUI while preserving
// the same read-only runner used by RunAssigned.
func RunAssignedDetailed(ctx context.Context, repo, kind string, number int, role, providerID string) (RunResult, error) {
	trace := RunResult{Status: "failed", Phases: []string{"Assignment accepted"}}
	if repo == "" || number <= 0 {
		return trace, fmt.Errorf("an owner/repo and positive task number are required")
	}
	trace.Phases = append(trace.Phases, "Checking selected CLI")
	selected, err := provider.NewRegistry().Detect(ctx, providerID, false)
	if err != nil {
		trace.Phases = append(trace.Phases, "CLI unavailable")
		return trace, err
	}
	trace.Phases = append(trace.Phases, "CLI ready", "Loading "+strings.ToUpper(kind)+" context")
	var input *agentcontext.AggregatedContext
	switch strings.ToLower(kind) {
	case "pr":
		input, err = agentcontext.NewPRLoader(repo).Load(ctx, fmt.Sprint(number))
	case "issue":
		input, err = agentcontext.NewIssueLoader(repo).Load(ctx, fmt.Sprint(number))
	default:
		return trace, fmt.Errorf("unsupported task kind %q", kind)
	}
	if err != nil {
		trace.Phases = append(trace.Phases, "Context load failed")
		return trace, err
	}
	trace.Phases = append(trace.Phases, "Context loaded")
	if strings.EqualFold(kind, "issue") {
		input.AvailableLabels, _ = agentcontext.FetchAvailableRepoLabels(ctx, repo)
	}

	rt := runtime.New(selected, runtime.Config{MinConfidence: 0.75, TokenBudget: runtime.DefaultTokenBudget, MaxRetries: 1})
	var agentResult *agent.AgentResult
	if strings.EqualFold(role, string(agent.RoleJanitor)) {
		trace.Phases = append(trace.Phases, "Janitor routing started")
		orchestration, runErr := rt.Orchestrate(ctx, input)
		if runErr != nil {
			trace.Phases = append(trace.Phases, "Janitor routing failed")
			return trace, runErr
		}
		trace.Phases = append(trace.Phases, orchestration.Phases...)
		trace.Status = orchestration.Status
		agentResult = &orchestration.Result
		reply := formatAgentReply("Janitor", agentResult)
		if reply == "" {
			reply = workerErrors(orchestration)
		}
		if orchestration.Status == "needs_input" {
			if workerReply := workerErrors(orchestration); workerReply != "" {
				reply += "\n" + workerReply
			}
		}
		trace.Summary = reply
		return trace, nil
	} else {
		var cfg agent.AgentConfig
		var prompt string
		switch strings.ToLower(role) {
		case "reviewer":
			if input.TargetType != "pr" {
				return trace, fmt.Errorf("Reviewer requires a pull request")
			}
			cfg, prompt = roles.NewReviewerConfig(), formatPRMetadataPrompt(input)
		case "sentinel":
			if input.TargetType != "pr" {
				return trace, fmt.Errorf("Sentinel requires a pull request")
			}
			cfg, prompt = roles.NewSentinelConfig(), formatPRMetadataPrompt(input)
		case "triage":
			if input.TargetType != "issue" {
				return trace, fmt.Errorf("Triage requires an issue")
			}
			cfg, prompt = roles.NewTriageConfig(), agentcontext.FormatIssuePrompt(input, input.AvailableLabels)
		case "diagnostics":
			cfg = roles.NewDiagnosticsConfig()
			prompt = fmt.Sprintf("CI STATUS: %s\n\nFAILED CHECK LOGS:\n%s\n\nRECENT COMMITS:\n%s", input.CIStatus, input.CILogs, strings.Join(input.RecentCommits, "\n"))
		default:
			return trace, fmt.Errorf("unknown agent role %q", role)
		}
		trace.Phases = append(trace.Phases, string(cfg.Role)+" analysis started")
		if (cfg.Role == agent.RoleReviewer || cfg.Role == agent.RoleSentinel) && agentcontext.EstimateTokens(input.Diff) > 0 {
			chunkBudget := rt.DiffChunkBudget(cfg, prompt)
			if chunkBudget <= 0 {
				err = fmt.Errorf("agent instructions and task metadata exceed the configured token budget")
			} else {
				chunks := agentcontext.ChunkDiff(input.Diff, chunkBudget)
				trace.Phases = append(trace.Phases, fmt.Sprintf("Analyzing %d diff chunk(s)", len(chunks)))
				agentResult, err = rt.ChunkedExecute(ctx, cfg, prompt, chunks)
			}
		} else {
			agentResult, err = rt.Execute(ctx, cfg, prompt)
		}
		if err != nil {
			trace.Phases = append(trace.Phases, "Analysis failed")
			return trace, err
		}
	}
	if agentResult == nil {
		return trace, fmt.Errorf("agent returned no result")
	}
	trace.Summary = formatAgentReply(role, agentResult)
	if trace.Status == "failed" {
		trace.Status = "complete"
	}
	trace.Phases = append(trace.Phases, "Response validated", "Complete")
	return trace, nil
}

func formatPRMetadataPrompt(input *agentcontext.AggregatedContext) string {
	withoutDiff := *input
	withoutDiff.Diff = ""
	return agentcontext.FormatPRPrompt(&withoutDiff)
}

func formatAgentReply(role string, result *agent.AgentResult) string {
	if result == nil {
		return ""
	}
	var reply strings.Builder
	if role != "" {
		fmt.Fprintf(&reply, "%s agent reply", role)
	}
	if strings.TrimSpace(result.Summary) != "" {
		if reply.Len() > 0 {
			reply.WriteString("\n")
		}
		reply.WriteString(strings.TrimSpace(result.Summary))
	}
	limit := min(len(result.Findings), 8)
	for i, finding := range result.Findings[:limit] {
		if reply.Len() > 0 {
			reply.WriteString("\n")
		}
		fmt.Fprintf(&reply, "%d. [%s] %s", i+1, finding.Severity, strings.TrimSpace(finding.Explanation))
		if finding.File != "" {
			fmt.Fprintf(&reply, " (%s", finding.File)
			if finding.Line > 0 {
				fmt.Fprintf(&reply, ":%d", finding.Line)
			}
			reply.WriteString(")")
		}
		if finding.SuggestedFix != "" {
			fmt.Fprintf(&reply, " Suggested fix: %s", strings.TrimSpace(finding.SuggestedFix))
		}
	}
	if remaining := len(result.Findings) - limit; remaining > 0 {
		fmt.Fprintf(&reply, "\n%d additional finding(s) are available in the full run output.", remaining)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(&reply, "\nWarning: %s", strings.TrimSpace(warning))
	}
	if len(result.Findings) == 0 && strings.TrimSpace(result.Summary) == "" && len(result.Warnings) == 0 {
		reply.WriteString("\nAnalysis completed without a summary or findings.")
	}
	return reply.String()
}

func workerErrors(result *runtime.OrchestrationResult) string {
	if result == nil {
		return "The agent orchestration produced no worker output."
	}
	var messages []string
	for _, worker := range result.Workers {
		if worker.Error != "" {
			messages = append(messages, fmt.Sprintf("%s (%s): %s", worker.Role, worker.Status, worker.Error))
		}
	}
	if len(messages) == 0 {
		return "No analysis reply was produced. Check that the PR diff or issue context is available."
	}
	return strings.Join(messages, "\n")
}
