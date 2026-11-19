/*
Copyright © 2026 hemanshulabs
*/
package cmd

import (
	"context"
	"fmt"
	"strings"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	agentcontext "github.com/hemanshulabs/gh-forge/v4/internal/agent/context"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/provider"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/roles"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/runtime"
	agentui "github.com/hemanshulabs/gh-forge/v4/internal/agent/ui"
)

var (
	triageConfidence  float64
	triageTokenBudget int
	triageProvider    string
	triageModel       string
	triageFormat      string
	triageComment     bool
	triageApply       bool
	triageDryRun      bool
	triagePrivate     bool
	triageRepo        string
)

var triageCmd = &cobra.Command{
	Use:   "triage [issue-number or url]",
	Short: "Run the Triage agent on a GitHub issue",
	Long: `Run the Triage agent to evaluate a GitHub issue for:
  • Issue classification and appropriate repository labels
  • Reproduction completeness and missing environment details
  • Urgency and priority assessment (P0-Critical to P3-Low)
  • Component identification and routing

By default, the Triage agent is strictly read-only and suggestion-only.
Use --apply to apply suggested labels or --comment to post triage notes (supports --dry-run).`,
	Example: `  # Triage issue #105 in current repo
  forge triage 105

  # Triage issue by URL
  forge triage https://github.com/owner/repo/issues/105

  # Apply suggested labels to issue (requires explicit opt-in)
  forge triage 105 --apply

  # Preview label application without writing
  forge triage 105 --apply --dry-run

  # Run in Private Mode using local Ollama
  forge triage 105 --private`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		target := args[0]

		// 1. Resolve repository context
		repo := triageRepo
		if repo == "" {
			_, ghRepo, _ := getCurrentGitAndGitHubRepos()
			if ghRepo.Name != "" {
				repo = fmt.Sprintf("%s/%s", ghRepo.Owner, ghRepo.Name)
			}
		}

		// 2. Load Issue context and available repo labels
		issueNum, err := agentcontext.ParseTargetIssue(target)
		if err != nil {
			return fmt.Errorf("invalid issue target: %w", err)
		}

		loader := agentcontext.NewIssueLoader(repo)
		actx, err := loader.Load(ctx, fmt.Sprint(issueNum))
		if err != nil {
			return fmt.Errorf("failed to load issue #%d context: %w", issueNum, err)
		}

		var availableLabels []string
		if repo != "" {
			availableLabels, _ = agentcontext.FetchAvailableRepoLabels(ctx, repo)
		}

		// 3. Resolve Provider
		reg := provider.NewRegistry()
		selectedProvider, err := reg.Detect(ctx, triageProvider, triagePrivate)
		if err != nil {
			return err
		}

		// 4. Initialize Agent Runtime
		agentCfg := roles.NewTriageConfig()
		rt := runtime.New(selectedProvider, runtime.Config{
			MinConfidence: triageConfidence,
			TokenBudget:   triageTokenBudget,
			MaxRetries:    1,
			DryRun:        triageDryRun,
			Model:         triageModel,
		})

		// 5. Execute Triage agent
		userPrompt := agentcontext.FormatIssuePrompt(actx, availableLabels)
		log.Debug("Executing Triage agent", "provider", selectedProvider.ID(), "issue", issueNum)
		result, err := rt.Execute(ctx, agentCfg, userPrompt)
		if err != nil {
			return fmt.Errorf("triage agent failed: %w", err)
		}

		// 6. Output findings
		formattedOutput, err := agentui.RenderResult(result, triageFormat)
		if err != nil {
			return err
		}
		fmt.Println(formattedOutput)

		// 7. Optional write actions (apply labels, comment)
		if triageApply {
			// Extract suggested labels from findings/suggested_fix
			var labelsToApply []string
			for _, f := range result.Findings {
				if strings.Contains(strings.ToLower(f.Explanation), "label") || strings.Contains(strings.ToLower(f.SuggestedFix), "label") {
					for _, candidate := range availableLabels {
						if strings.Contains(strings.ToLower(f.SuggestedFix), strings.ToLower(candidate)) ||
							strings.Contains(strings.ToLower(f.Explanation), strings.ToLower(candidate)) {
							labelsToApply = append(labelsToApply, candidate)
						}
					}
				}
			}
			if len(labelsToApply) > 0 {
				res, writeErr := rt.Executor().ApplyIssueLabels(ctx, repo, actx.Number, labelsToApply)
				if writeErr != nil {
					return writeErr
				}
				fmt.Println(res)
			}
		}

		if triageComment {
			res, writeErr := rt.Executor().PostIssueComment(ctx, repo, actx.Number, result)
			if writeErr != nil {
				return writeErr
			}
			fmt.Println(res)
		}

		return nil
	},
}

func init() {
	triageCmd.Flags().Float64Var(&triageConfidence, "confidence", 0.75, "minimum finding confidence (minimum 0.75)")
	triageCmd.Flags().IntVar(&triageTokenBudget, "token-budget", 16000, "maximum token budget for prompt context")
	triageCmd.Flags().StringVar(&triageProvider, "provider", "", "override provider (claude-cli, codex-cli, opencode-cli, agy-cli, gemini-cli, direct-api, ollama)")
	triageCmd.Flags().StringVar(&triageModel, "model", "", "override backend model name")
	triageCmd.Flags().StringVarP(&triageFormat, "format", "f", "pretty", "output format: pretty, json, markdown")
	triageCmd.Flags().BoolVar(&triageApply, "apply", false, "apply suggested labels to the issue on GitHub (opt-in write action)")
	triageCmd.Flags().BoolVar(&triageComment, "comment", false, "post triage evaluation as a comment on GitHub (opt-in write action)")
	triageCmd.Flags().BoolVar(&triageDryRun, "dry-run", false, "simulate write operations without publishing to GitHub")
	triageCmd.Flags().BoolVar(&triagePrivate, "private", false, "opt-in private mode: strictly use local Ollama model")
	triageCmd.Flags().StringVarP(&triageRepo, "repo", "R", "", "target GitHub repository (e.g. owner/repo)")

	rootCmd.AddCommand(triageCmd)
	agentsTriageCmd := *triageCmd
	agentsCmd.AddCommand(&agentsTriageCmd)
}
