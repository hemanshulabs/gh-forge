/*
Copyright © 2026 hemanshulabs
*/
package cmd

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	agentcontext "github.com/dlvhdr/gh-dash/v4/internal/agent/context"
	"github.com/dlvhdr/gh-dash/v4/internal/agent/provider"
	"github.com/dlvhdr/gh-dash/v4/internal/agent/roles"
	"github.com/dlvhdr/gh-dash/v4/internal/agent/runtime"
	agentui "github.com/dlvhdr/gh-dash/v4/internal/agent/ui"
)

var (
	reviewConfidence  float64
	reviewTokenBudget int
	reviewProvider    string
	reviewModel       string
	reviewFormat      string
	reviewComment     bool
	reviewDryRun      bool
	reviewPrivate     bool
	reviewRepo        string
)

var reviewCmd = &cobra.Command{
	Use:   "review [pr-number or branch or url]",
	Short: "Run the Reviewer agent on a pull request",
	Long: `Run the Reviewer agent to analyze a pull request diff for:
  • Code quality, maintainability, and clean code principles
  • Style consistency and idiomatic language usage
  • Potential runtime regressions and missing tests
  • High-level summary of proposed changes

By default, the Reviewer agent is strictly read-only.
Use --comment to post findings as a review comment on GitHub (supports --dry-run).`,
	Example: `  # Review PR #42 in the current GitHub repo
  forge review 42

  # Review a PR by full URL
  forge review https://github.com/owner/repo/pull/42

  # Output raw JSON with minimum confidence of 0.8
  forge review 42 --confidence 0.8 --format json

  # Post findings to the PR (read-only by default, explicit flag required)
  forge review 42 --comment

  # Preview comment without writing to GitHub
  forge review 42 --comment --dry-run

  # Run in Private Mode using local Ollama
  forge review 42 --private`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		target := "HEAD"
		if len(args) > 0 {
			target = args[0]
		}

		// 1. Resolve repository context
		repo := reviewRepo
		if repo == "" {
			_, ghRepo, _ := getCurrentGitAndGitHubRepos()
			if ghRepo.Name != "" {
				repo = fmt.Sprintf("%s/%s", ghRepo.Owner, ghRepo.Name)
			}
		}

		// 2. Load PR context
		prNum, err := agentcontext.ParseTargetPR(target)
		var actx *agentcontext.AggregatedContext
		if err == nil {
			loader := agentcontext.NewPRLoader(repo)
			actx, err = loader.Load(ctx, fmt.Sprint(prNum))
			if err != nil {
				return fmt.Errorf("failed to load PR #%d context: %w", prNum, err)
			}
		} else {
			// Local diff fallback if not a numeric PR
			diff, diffErr := agentcontext.FetchLocalDiff(ctx)
			if diffErr != nil {
				return fmt.Errorf("could not resolve PR or local git diff: %w", diffErr)
			}
			actx = &agentcontext.AggregatedContext{
				TargetType: "local",
				Repo:       repo,
				Title:      "Local git changes",
				Diff:       diff,
			}
		}

		// 3. Resolve Provider
		reg := provider.NewRegistry()
		selectedProvider, err := reg.Detect(ctx, reviewProvider, reviewPrivate)
		if err != nil {
			return err
		}

		// 4. Initialize Agent Runtime
		agentCfg := roles.NewReviewerConfig()
		rt := runtime.New(selectedProvider, runtime.Config{
			MinConfidence: reviewConfidence,
			TokenBudget:   reviewTokenBudget,
			MaxRetries:    2,
			DryRun:        reviewDryRun,
		})

		// 5. Chunk diff if necessary and execute
		basePrompt := agentcontext.FormatPRPrompt(actx)
		diffChunks := agentcontext.ChunkDiff(actx.Diff, reviewTokenBudget/2)

		log.Debug("Executing Reviewer agent", "provider", selectedProvider.ID(), "chunks", len(diffChunks))
		result, err := rt.ChunkedExecute(ctx, agentCfg, basePrompt, diffChunks)
		if err != nil {
			return fmt.Errorf("reviewer agent failed: %w", err)
		}

		// 6. Output findings
		formattedOutput, err := agentui.RenderResult(result, reviewFormat)
		if err != nil {
			return err
		}
		fmt.Println(formattedOutput)

		// 7. Optional write action (comment)
		if reviewComment && actx.Number > 0 {
			res, writeErr := rt.Executor().PostPRComment(ctx, actx.Repo, actx.Number, result)
			if writeErr != nil {
				return writeErr
			}
			fmt.Println(res)
		}

		return nil
	},
}

func init() {
	reviewCmd.Flags().Float64Var(&reviewConfidence, "confidence", 0.75, "drop findings with confidence below this threshold (0.0 - 1.0)")
	reviewCmd.Flags().IntVar(&reviewTokenBudget, "token-budget", 32000, "maximum token budget for prompt context and diffs")
	reviewCmd.Flags().StringVar(&reviewProvider, "provider", "", "override AI provider (claude-cli, direct-api, ollama)")
	reviewCmd.Flags().StringVar(&reviewModel, "model", "", "override backend model name")
	reviewCmd.Flags().StringVarP(&reviewFormat, "format", "f", "pretty", "output format: pretty, json, markdown")
	reviewCmd.Flags().BoolVar(&reviewComment, "comment", false, "post findings as a review comment on GitHub (opt-in write action)")
	reviewCmd.Flags().BoolVar(&reviewDryRun, "dry-run", false, "simulate write operations without publishing to GitHub")
	reviewCmd.Flags().BoolVar(&reviewPrivate, "private", false, "opt-in private mode: strictly use local Ollama model")
	reviewCmd.Flags().StringVarP(&reviewRepo, "repo", "R", "", "target GitHub repository (e.g. owner/repo)")

	rootCmd.AddCommand(reviewCmd)
	agentsReviewCmd := *reviewCmd
	agentsCmd.AddCommand(&agentsReviewCmd)
}
