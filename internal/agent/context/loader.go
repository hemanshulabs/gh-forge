package context

import (
	"context"
	"strings"
	"unicode/utf8"
)

func boundedText(value string, maxChars int) string {
	if utf8.RuneCountInString(value) <= maxChars {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxChars]) + "\n[context truncated for token budget]"
}

func recentComments(comments []string, maxCount, maxChars int) []string {
	start := max(0, len(comments)-maxCount)
	out := make([]string, 0, len(comments)-start)
	for _, comment := range comments[start:] {
		out = append(out, boundedText(strings.TrimSpace(comment), maxChars))
	}
	return out
}

// AggregatedContext represents the bundle of context gathered by the runtime for an agent.
type AggregatedContext struct {
	TargetType      string   // "pr", "issue", "branch", "repo"
	Repo            string   // "owner/repo"
	Number          int      // PR or Issue number
	Title           string   // PR or Issue title
	Body            string   // PR or Issue description
	Diff            string   // Cleaned git/PR diff
	DiffLoadError   string   // Error returned when a PR diff could not be loaded
	Comments        []string // Recent discussion/activity
	Labels          []string // Existing issue/PR labels
	AvailableLabels []string // Repository labels available for triage recommendations
	RecentCommits   []string
	CIStatus        string // "SUCCESS", "FAILURE", etc.
	CILogs          string // Failure logs if CI failed
}

// ContextLoader defines a method for extracting context for a given target.
type ContextLoader interface {
	Load(ctx context.Context, target string) (*AggregatedContext, error)
}
