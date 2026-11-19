package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	agentcontext "github.com/hemanshulabs/gh-forge/v4/internal/agent/context"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/provider"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/roles"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/runtime"
	agentui "github.com/hemanshulabs/gh-forge/v4/internal/agent/ui"
)

var (
	diagRepo       string
	diagLogsFile   string
	diagProvider   string
	diagModel      string
	diagPrivate    bool
	diagConfidence float64
	diagTokenLimit int
	diagFormat     string
)

var diagCmd = &cobra.Command{
	Use:     "diag [pr-number-or-url]",
	Aliases: []string{"diagnose"},
	Short:   "Analyze supplied CI failure logs with Diagnostics",
	Args:    cobra.MaximumNArgs(1),
	Long:    "Diagnostics needs actual failure logs. Supply them with --logs-file; GitHub check names alone are not treated as logs.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if diagFormat != "pretty" && diagFormat != "markdown" && diagFormat != "json" {
			return fmt.Errorf("unsupported format %q; use pretty, markdown, or json", diagFormat)
		}
		if strings.TrimSpace(diagLogsFile) == "" {
			return fmt.Errorf("Diagnostics needs failure logs; supply a local log file with --logs-file")
		}
		budget := diagTokenLimit
		if budget <= 0 {
			budget = 16000
		}
		if budget > 100000 {
			return fmt.Errorf("--token-budget cannot exceed 100000")
		}
		logs, err := readAgentLogs(diagLogsFile, budget)
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		repo := diagRepo
		if repo == "" && len(args) == 1 {
			repo, err = agentTargetRepository(args[0])
			if err != nil {
				return err
			}
		}
		if repo == "" {
			_, ghRepo, _ := getCurrentGitAndGitHubRepos()
			if ghRepo.Name != "" {
				repo = fmt.Sprintf("%s/%s", ghRepo.Owner, ghRepo.Name)
			}
		}
		loaded := &agentcontext.AggregatedContext{TargetType: "ci", Repo: repo, Title: "CI failure", CILogs: logs}
		if len(args) == 1 {
			prNumber, parseErr := agentcontext.ParseTargetPR(args[0])
			if parseErr != nil {
				return fmt.Errorf("invalid pull request target: %w", parseErr)
			}
			loaded, err = agentcontext.NewPRLoader(repo).Load(ctx, fmt.Sprint(prNumber))
			if err != nil {
				return fmt.Errorf("failed to load pull request context: %w", err)
			}
			loaded.CILogs = logs
		}
		selected, err := provider.NewRegistry().Detect(ctx, diagProvider, diagPrivate)
		if err != nil {
			return err
		}
		rt := runtime.New(selected, runtime.Config{MinConfidence: diagConfidence, TokenBudget: budget, MaxRetries: 1, Model: diagModel})
		prompt := fmt.Sprintf("CI STATUS: %s\n\nFAILED CHECK LOGS:\n%s\n\nAnalyze the supplied evidence only. If the logs do not identify a root cause, say so.", loaded.CIStatus, loaded.CILogs)
		result, err := rt.Execute(ctx, roles.NewDiagnosticsConfig(), prompt)
		if err != nil {
			return fmt.Errorf("Diagnostics failed: %w", err)
		}
		if diagFormat == "json" {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		}
		formatted, err := agentui.RenderResult(result, diagFormat)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), formatted)
		return nil
	},
}

func readAgentLogs(path string, tokenBudget int) (string, error) {
	if tokenBudget <= 0 || tokenBudget > runtime.MaxTokenBudget {
		return "", fmt.Errorf("log token budget must be between 1 and %d", runtime.MaxTokenBudget)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("could not open requested log file: %w", err)
	}
	defer file.Close()
	maxBytes := int64(tokenBudget) * 4
	logs, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return "", fmt.Errorf("could not read requested log file: %w", err)
	}
	if int64(len(logs)) > maxBytes {
		return "", fmt.Errorf("log file exceeds the configured context budget (%d estimated tokens); reduce it or increase --token-budget", tokenBudget)
	}
	return string(logs), nil
}

func init() {
	diagCmd.Flags().StringVarP(&diagLogsFile, "logs-file", "l", "", "path to a local CI failure log file to analyze")
	diagCmd.Flags().StringVarP(&diagRepo, "repo", "R", "", "target GitHub repository (owner/repo)")
	diagCmd.Flags().StringVar(&diagProvider, "provider", "", "override provider (claude-cli, codex-cli, opencode-cli, agy-cli, gemini-cli, direct-api, ollama)")
	diagCmd.Flags().StringVar(&diagModel, "model", "", "override backend model name")
	diagCmd.Flags().BoolVar(&diagPrivate, "private", false, "use only local Ollama")
	diagCmd.Flags().Float64Var(&diagConfidence, "confidence", 0.75, "minimum confidence for findings (minimum 0.75)")
	diagCmd.Flags().IntVar(&diagTokenLimit, "token-budget", 16000, "maximum context token budget")
	diagCmd.Flags().StringVarP(&diagFormat, "format", "f", "pretty", "output format: pretty, markdown, or json")
	rootCmd.AddCommand(diagCmd)
}
