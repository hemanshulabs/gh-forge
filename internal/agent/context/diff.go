package context

import (
	"context"
	"fmt"
	"os/exec"
)

// FetchPRDiff retrieves the diff for a pull request using the 'gh' CLI.
func FetchPRDiff(ctx context.Context, repo string, prNumber int) (string, error) {
	args := []string{"pr", "diff", fmt.Sprint(prNumber)}
	if repo != "" {
		args = append(args, "-R", repo)
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to fetch PR diff: %w: %s", err, string(output))
	}

	return SanitizeDiff(string(output)), nil
}

// FetchLocalDiff retrieves the diff in the current working directory compared to origin/main (or HEAD~1).
func FetchLocalDiff(ctx context.Context) (string, error) {
	// Try diffing against origin/main or main
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD~1")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to git diff staged/unstaged
		cmd = exec.CommandContext(ctx, "git", "diff", "HEAD")
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to fetch local git diff: %w", err)
		}
	}

	return SanitizeDiff(string(output)), nil
}
