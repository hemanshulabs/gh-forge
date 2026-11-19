package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	agentcontext "github.com/hemanshulabs/gh-forge/v4/internal/agent/context"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/provider"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/runtime"
	agentui "github.com/hemanshulabs/gh-forge/v4/internal/agent/ui"
)

var (
	janitorType       string
	janitorRepo       string
	janitorLogsFile   string
	janitorProvider   string
	janitorModel      string
	janitorPrivate    bool
	janitorConfidence float64
	janitorTokenLimit int
	janitorFormat     string
)

var janitorCmd = &cobra.Command{
	Use:   "janitor [pr-number-or-issue-number-or-url]",
	Short: "Route a GitHub task to Forge's specialized agents",
	Args:  cobra.ExactArgs(1),
	Long:  "Janitor analyzes loaded PR or issue context, dispatches applicable read-only workers, and prints their validated findings.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		target := strings.TrimSpace(args[0])
		targetType := janitorType
		if targetType == "" {
			switch {
			case strings.Contains(target, "/issues/"):
				targetType = "issue"
			default:
				targetType = "pr"
			}
		}
		repo := janitorRepo
		if repo == "" {
			targetRepo, repoErr := agentTargetRepository(target)
			if repoErr != nil {
				return repoErr
			}
			repo = targetRepo
		}
		if repo == "" {
			_, ghRepo, _ := getCurrentGitAndGitHubRepos()
			if ghRepo.Name != "" {
				repo = fmt.Sprintf("%s/%s", ghRepo.Owner, ghRepo.Name)
			}
		}
		var loaded *agentcontext.AggregatedContext
		var err error
		switch targetType {
		case "issue":
			issueNumber, parseErr := agentcontext.ParseTargetIssue(target)
			if parseErr != nil {
				return fmt.Errorf("invalid issue target: %w", parseErr)
			}
			loaded, err = agentcontext.NewIssueLoader(repo).Load(ctx, fmt.Sprint(issueNumber))
		case "pr":
			prNumber, parseErr := agentcontext.ParseTargetPR(target)
			if parseErr != nil {
				return fmt.Errorf("invalid pull request target: %w", parseErr)
			}
			loaded, err = agentcontext.NewPRLoader(repo).Load(ctx, fmt.Sprint(prNumber))
		default:
			return fmt.Errorf("unsupported target type %q; use pr or issue", targetType)
		}
		if err != nil {
			return fmt.Errorf("failed to load %s context: %w", targetType, err)
		}
		if loaded.Repo == "" {
			loaded.Repo = repo
		}
		if janitorLogsFile != "" {
			budget := janitorTokenLimit
			if budget <= 0 {
				budget = runtime.DefaultTokenBudget
			}
			loaded.CILogs, err = readAgentLogs(janitorLogsFile, budget)
			if err != nil {
				return err
			}
		}
		if targetType == "issue" && repo != "" {
			loaded.AvailableLabels, _ = agentcontext.FetchAvailableRepoLabels(ctx, repo)
		}
		if janitorFormat != "pretty" && janitorFormat != "json" {
			return fmt.Errorf("unsupported format %q; use pretty or json", janitorFormat)
		}
		selected, err := provider.NewRegistry().Detect(ctx, janitorProvider, janitorPrivate)
		if err != nil {
			return err
		}
		rt := runtime.New(selected, runtime.Config{MinConfidence: janitorConfidence, TokenBudget: janitorTokenLimit, MaxRetries: 1, Model: janitorModel})
		result, err := rt.Orchestrate(ctx, loaded)
		if err != nil {
			return err
		}
		if janitorFormat == "json" {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Janitor status: %s\n", result.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "Phases: %s\n\n", strings.Join(result.Phases, " -> "))
		for _, worker := range result.Workers {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s — %s\n", worker.Role, worker.Status, worker.Reason)
			if worker.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", worker.Error)
			}
			if worker.Result != nil {
				formatted, renderErr := agentui.RenderResult(worker.Result, "pretty")
				if renderErr != nil {
					return renderErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), formatted)
			}
		}
		return nil
	},
}

func init() {
	janitorCmd.Flags().StringVar(&janitorType, "type", "", "target type: pr or issue (inferred from URL when possible)")
	janitorCmd.Flags().StringVarP(&janitorRepo, "repo", "R", "", "target GitHub repository (owner/repo)")
	janitorCmd.Flags().StringVarP(&janitorLogsFile, "logs-file", "l", "", "path to a local CI failure log file for Diagnostics")
	janitorCmd.Flags().StringVar(&janitorProvider, "provider", "", "override provider (claude-cli, codex-cli, opencode-cli, agy-cli, gemini-cli, direct-api, ollama)")
	janitorCmd.Flags().StringVar(&janitorModel, "model", "", "override backend model name")
	janitorCmd.Flags().BoolVar(&janitorPrivate, "private", false, "use only local Ollama")
	janitorCmd.Flags().Float64Var(&janitorConfidence, "confidence", 0.75, "minimum confidence for findings (minimum 0.75)")
	janitorCmd.Flags().IntVar(&janitorTokenLimit, "token-budget", 32000, "maximum context token budget")
	janitorCmd.Flags().StringVar(&janitorFormat, "format", "pretty", "output format: pretty or json")
	rootCmd.AddCommand(janitorCmd)
}

func agentTargetRepository(target string) (string, error) {
	parsed, err := url.Parse(target)
	if err != nil || parsed.Host == "" {
		return "", nil
	}
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "github.com") {
		return "", fmt.Errorf("unsupported GitHub target URL %q; use a https://github.com/owner/repo/... URL", target)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 || (parts[2] != "issues" && parts[2] != "pull") {
		return "", fmt.Errorf("invalid GitHub issue or pull request URL %q", target)
	}
	return parts[0] + "/" + parts[1], nil
}
