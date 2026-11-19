package context

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hemanshulabs/gh-forge/v4/internal/data"
)

// IssueLoader loads context for issue triage.
type IssueLoader struct {
	Repo string
}

func NewIssueLoader(repo string) *IssueLoader {
	return &IssueLoader{Repo: repo}
}

type ghIssueViewData struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Comments []struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Body string `json:"body"`
	} `json:"comments"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (l *IssueLoader) Load(ctx context.Context, target string) (*AggregatedContext, error) {
	args := []string{
		"issue", "view", target,
		"--json", "number,title,body,state,author,comments,labels",
	}
	if l.Repo != "" {
		args = append(args, "-R", l.Repo)
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue metadata via 'gh issue view': %w", err)
	}

	var data ghIssueViewData
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to decode issue metadata: %w", err)
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

	return &AggregatedContext{
		TargetType: "issue",
		Repo:       l.Repo,
		Number:     data.Number,
		Title:      data.Title,
		Body:       data.Body,
		Comments:   comments,
		Labels:     labels,
	}, nil
}

// FetchAvailableRepoLabels retrieves available labels in the repo to provide to Triage.
func FetchAvailableRepoLabels(ctx context.Context, repo string) ([]string, error) {
	labels, err := data.FetchRepoLabels(repo)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	return names, nil
}

// ParseTargetIssue extracts the issue number if target is numeric or a URL.
func ParseTargetIssue(target string) (int, error) {
	clean := strings.TrimSpace(target)
	if num, err := strconv.Atoi(clean); err == nil {
		return num, nil
	}
	if strings.Contains(clean, "/issues/") {
		parts := strings.Split(clean, "/issues/")
		if len(parts) >= 2 {
			sub := strings.Split(parts[1], "/")[0]
			sub = strings.Split(sub, "#")[0]
			return strconv.Atoi(sub)
		}
	}
	return 0, fmt.Errorf("invalid issue identifier: %q", target)
}

// FormatIssuePrompt formats AggregatedContext and available repository labels for the Triage agent.
func FormatIssuePrompt(actx *AggregatedContext, availableLabels []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("ISSUE #%d: %s\n\n", actx.Number, actx.Title))
	if actx.Body != "" {
		b.WriteString(fmt.Sprintf("ISSUE BODY:\n%s\n\n", boundedText(actx.Body, 12000)))
	}
	if len(actx.Labels) > 0 {
		b.WriteString(fmt.Sprintf("CURRENT LABELS: %s\n\n", strings.Join(actx.Labels, ", ")))
	}
	comments := recentComments(actx.Comments, 12, 1800)
	if len(comments) > 0 {
		b.WriteString("COMMENTS / DISCUSSION:\n")
		for _, c := range comments {
			b.WriteString(fmt.Sprintf("- %s\n", c))
		}
		b.WriteString("\n")
	}
	if len(availableLabels) > 0 {
		b.WriteString("AVAILABLE REPOSITORY LABELS (suggest from this list when applicable):\n")
		labels := availableLabels
		if len(labels) > 300 {
			labels = labels[:300]
		}
		b.WriteString(strings.Join(labels, ", "))
		b.WriteString("\n")
	}

	return b.String()
}
