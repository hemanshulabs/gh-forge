/*
Copyright © 2026 hemanshulabs
*/
package cmd

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	agentcontext "github.com/hemanshulabs/gh-forge/v4/internal/agent/context"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/provider"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/roles"
	"github.com/hemanshulabs/gh-forge/v4/internal/agent/runtime"
	agentui "github.com/hemanshulabs/gh-forge/v4/internal/agent/ui"
)

var (
	scanConfidence  float64
	scanTokenBudget int
	scanProvider    string
	scanModel       string
	scanFormat      string
	scanComment     bool
	scanDryRun      bool
	scanPrivate     bool
	scanRepo        string
)

var scanCmd = &cobra.Command{
	Use:   "scan [pr-number or branch or url]",
	Short: "Run the Sentinel security agent on a pull request or code changes",
	Long: `Run the Sentinel security agent to proactively audit changes for:
  • Authentication and authorization gaps
  • Injection flaws (SQL, command, XSS, SSRF, path traversal)
  • Exposed secrets, tokens, and hardcoded credentials
  • Concurrency vulnerabilities and race conditions
  • Critical edge cases that could break or be exploited

By default, Sentinel is strictly read-only and suggestion-only.
Use --comment to post security findings as a review comment on GitHub (supports --dry-run).`,
	Example: `  # Security scan PR #42 in current repo
  forge scan 42

  # Security scan local uncommitted/recent changes
  forge scan

  # Scan with high confidence threshold
  forge scan 42 --confidence 0.85

  # Run security scan in Private Mode (offline via Ollama)
  forge scan 42 --private`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		target := "HEAD"
		if len(args) > 0 {
			target = args[0]
		}

		// 1. Resolve repository context
		repo := scanRepo
		if repo == "" {
			_, ghRepo, _ := getCurrentGitAndGitHubRepos()
			if ghRepo.Name != "" {
				repo = fmt.Sprintf("%s/%s", ghRepo.Owner, ghRepo.Name)
			}
		}

		// 2. Load context (PR diff or local git diff)
		prNum, err := agentcontext.ParseTargetPR(target)
		var actx *agentcontext.AggregatedContext
		if err == nil {
			loader := agentcontext.NewPRLoader(repo)
			actx, err = loader.Load(ctx, fmt.Sprint(prNum))
			if err != nil {
				return fmt.Errorf("failed to load PR #%d context: %w", prNum, err)
			}
		} else {
			diff, diffErr := agentcontext.FetchLocalDiff(ctx)
			if diffErr != nil {
				return fmt.Errorf("could not resolve PR or local git diff: %w", diffErr)
			}
			actx = &agentcontext.AggregatedContext{
				TargetType: "local",
				Repo:       repo,
				Title:      "Local changes",
				Diff:       diff,
			}
		}

		// 3. Resolve Provider
		reg := provider.NewRegistry()
		selectedProvider, err := reg.Detect(ctx, scanProvider, scanPrivate)
		if err != nil {
			return err
		}

		// 4. Initialize Agent Runtime
		agentCfg := roles.NewSentinelConfig()
		rt := runtime.New(selectedProvider, runtime.Config{
			MinConfidence: scanConfidence,
			TokenBudget:   scanTokenBudget,
			MaxRetries:    1,
			DryRun:        scanDryRun,
			Model:         scanModel,
		})

		// 5. Chunk diff if necessary and execute
		basePrompt := agentcontext.FormatPRPrompt(actx)
		diffChunks := agentcontext.ChunkDiff(actx.Diff, scanTokenBudget/2)

		log.Debug("Executing Sentinel agent", "provider", selectedProvider.ID(), "chunks", len(diffChunks))
		result, err := rt.ChunkedExecute(ctx, agentCfg, basePrompt, diffChunks)
		if err != nil {
			return fmt.Errorf("sentinel security scan failed: %w", err)
		}

		// 6. Output findings
		formattedOutput, err := agentui.RenderResult(result, scanFormat)
		if err != nil {
			return err
		}
		fmt.Println(formattedOutput)

		// 7. Optional write action (comment)
		if scanComment && actx.Number > 0 {
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
	scanCmd.Flags().Float64Var(&scanConfidence, "confidence", 0.80, "minimum finding confidence (minimum 0.75)")
	scanCmd.Flags().IntVar(&scanTokenBudget, "token-budget", 32000, "maximum token budget for prompt context and diffs")
	scanCmd.Flags().StringVar(&scanProvider, "provider", "", "override provider (claude-cli, codex-cli, opencode-cli, agy-cli, gemini-cli, direct-api, ollama)")
	scanCmd.Flags().StringVar(&scanModel, "model", "", "override backend model name")
	scanCmd.Flags().StringVarP(&scanFormat, "format", "f", "pretty", "output format: pretty, json, markdown")
	scanCmd.Flags().BoolVar(&scanComment, "comment", false, "post findings as a review comment on GitHub (opt-in write action)")
	scanCmd.Flags().BoolVar(&scanDryRun, "dry-run", false, "simulate write operations without publishing to GitHub")
	scanCmd.Flags().BoolVar(&scanPrivate, "private", false, "opt-in private mode: strictly use local Ollama model")
	scanCmd.Flags().StringVarP(&scanRepo, "repo", "R", "", "target GitHub repository (e.g. owner/repo)")

	rootCmd.AddCommand(scanCmd)
	agentsScanCmd := *scanCmd
	agentsCmd.AddCommand(&agentsScanCmd)
}
