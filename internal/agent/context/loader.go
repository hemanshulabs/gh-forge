package context

import (
	"context"
)

// AggregatedContext represents the bundle of context gathered by the runtime for an agent.
type AggregatedContext struct {
	TargetType  string   // "pr", "issue", "branch", "repo"
	Repo        string   // "owner/repo"
	Number      int      // PR or Issue number
	Title       string   // PR or Issue title
	Body        string   // PR or Issue description
	Diff        string   // Cleaned git/PR diff
	Comments    []string // Recent discussion/activity
	Labels      []string // Existing issue/PR labels
	RecentCommits []string
	CIStatus    string   // "SUCCESS", "FAILURE", etc.
	CILogs      string   // Failure logs if CI failed
}

// ContextLoader defines a method for extracting context for a given target.
type ContextLoader interface {
	Load(ctx context.Context, target string) (*AggregatedContext, error)
}
