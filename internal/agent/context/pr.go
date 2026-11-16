package context

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PRLoader loads full context for pull request analysis.
type PRLoader struct {
	Repo string
}

func NewPRLoader(repo string) *PRLoader {
	return &PRLoader{Repo: repo}
}

type ghPRViewData struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	HeadRef  string `json:"headRefName"`
	BaseRef  string `json:"baseRefName"`
	Comments []struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Body string `json:"body"`
	} `json:"comments"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	StatusCheckRollup []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"statusCheckRollup"`
}

func (l *PRLoader) Load(ctx context.Context, target string) (*AggregatedContext, error) {
	args := []string{
		"pr", "view", target,
		"--json", "number,title,body,headRefName,baseRefName,comments,labels,statusCheckRollup",
	}
	if l.Repo != "" {
		args = append(args, "-R", l.Repo)
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR metadata via 'gh pr view': %w", err)
	}

	var data ghPRViewData
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to decode PR metadata: %w", err)
	}

	// Fetch diff
	diff, err := FetchPRDiff(ctx, l.Repo, data.Number)
	if err != nil {
		diff = "[Unable to load diff automatically]"
	}

	comments := make([]string, 0, len(data.Comments))
	for _, c := range data.Comments {
		if strings.TrimSpace(c.Body) != "" {
			comments = append(comments, fmt.Sprintf("@%s: %s", c.Author.Login, c.Body))
		}
	}

	labels := make([]string, 0, len(data.Labels))
	for _, lbl := range data.Labels {
		labels = append(labels, lbl.Name)
	}

	ciStatus := "UNKNOWN"
	var failingChecks []string
	for _, check := range data.StatusCheckRollup {
		if check.Conclusion == "FAILURE" || check.Conclusion == "ACTION_REQUIRED" {
			failingChecks = append(failingChecks, fmt.Sprintf("%s (%s)", check.Name, check.Conclusion))
		}
	}
	if len(failingChecks) > 0 {
		ciStatus = fmt.Sprintf("FAILED (%s)", strings.Join(failingChecks, ", "))
	} else if len(data.StatusCheckRollup) > 0 {
		ciStatus = "PASSED"
	}

	return &AggregatedContext{
		TargetType: "pr",
		Repo:       l.Repo,
		Number:     data.Number,
		Title:      data.Title,
		Body:       data.Body,
		Diff:       diff,
		Comments:   comments,
		Labels:     labels,
		CIStatus:   ciStatus,
	}, nil
}

// ParseTargetPR extracts the PR number if target is numeric or a URL.
func ParseTargetPR(target string) (int, error) {
	clean := strings.TrimSpace(target)
	if num, err := strconv.Atoi(clean); err == nil {
		return num, nil
	}
	if strings.Contains(clean, "/pull/") {
		parts := strings.Split(clean, "/pull/")
		if len(parts) >= 2 {
			sub := strings.Split(parts[1], "/")[0]
			sub = strings.Split(sub, "#")[0]
			return strconv.Atoi(sub)
		}
	}
	return 0, fmt.Errorf("invalid PR identifier: %q", target)
}

// FormatPRPrompt converts AggregatedContext into a focused user prompt for Reviewer / Sentinel.
func FormatPRPrompt(actx *AggregatedContext) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("PULL REQUEST #%d: %s\n\n", actx.Number, actx.Title))
	if actx.Body != "" {
		b.WriteString(fmt.Sprintf("DESCRIPTION:\n%s\n\n", actx.Body))
	}
	if actx.CIStatus != "" && actx.CIStatus != "UNKNOWN" {
		b.WriteString(fmt.Sprintf("CI STATUS: %s\n\n", actx.CIStatus))
	}
	if len(actx.Comments) > 0 {
		b.WriteString("RECENT COMMENTS / DISCUSSION:\n")
		for _, c := range actx.Comments {
			b.WriteString(fmt.Sprintf("- %s\n", c))
		}
		b.WriteString("\n")
	}
	b.WriteString("CODE DIFF:\n")
	b.WriteString(SanitizeDiff(actx.Diff))

	return b.String()
}
