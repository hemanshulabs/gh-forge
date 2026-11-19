package tasks

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent/runner"
	tuictx "github.com/hemanshulabs/gh-forge/v4/internal/tui/context"
)

var agentRunCancels = struct {
	sync.Mutex
	runs map[string]agentRunCancel
}{runs: make(map[string]agentRunCancel)}

type agentRunCancel struct {
	cancel context.CancelFunc
	token  *byte
}

// CancelAgent stops the active run for a task key, regardless of which view started it.
func CancelAgent(key string) bool {
	agentRunCancels.Lock()
	run := agentRunCancels.runs[key]
	agentRunCancels.Unlock()
	if run.cancel == nil {
		return false
	}
	run.cancel()
	return true
}

type AgentRunFinishedMsg struct {
	Key         string
	SectionID   int
	SectionType string
	Summary     string
	Phases      []string
	Status      string
	Err         error
}

type AgentTaskUpdatedMsg struct {
	Task tuictx.AgentTask
}

func RunAgent(ctx context.Context, key string, sectionID int, sectionType, repo, kind string, number int, role, providerID string) tea.Cmd {
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	token := new(byte)
	agentRunCancels.Lock()
	if previous := agentRunCancels.runs[key]; previous.cancel != nil {
		previous.cancel()
	}
	agentRunCancels.runs[key] = agentRunCancel{cancel: cancel, token: token}
	agentRunCancels.Unlock()
	return func() tea.Msg {
		defer func() {
			cancel()
			agentRunCancels.Lock()
			if current := agentRunCancels.runs[key]; current.token == token {
				delete(agentRunCancels.runs, key)
			}
			agentRunCancels.Unlock()
		}()
		log.Info("Starting assigned agent", "repo", repo, "kind", kind, "number", number, "role", role, "provider", providerID)
		result, err := runner.RunAssignedDetailed(runCtx, repo, kind, number, role, providerID)
		if err != nil {
			log.Error("Assigned agent failed", "repo", repo, "kind", kind, "number", number, "role", role, "provider", providerID, "error", err)
		} else {
			log.Info("Assigned agent finished", "repo", repo, "kind", kind, "number", number, "role", role, "provider", providerID, "status", result.Status)
		}
		return AgentRunFinishedMsg{Key: key, SectionID: sectionID, SectionType: sectionType, Summary: result.Summary, Phases: result.Phases, Status: result.Status, Err: err}
	}
}
