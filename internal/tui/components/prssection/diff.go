package prssection

import (
	tea "charm.land/bubbletea/v2"

	"github.com/hemanshulabs/gh-forge/v4/internal/tui/common"
)

func (m Model) diff() tea.Cmd {
	currRowData := m.GetCurrRow()
	if currRowData == nil {
		return nil
	}

	return common.DiffPR(
		currRowData.GetNumber(),
		currRowData.GetRepoNameWithOwner(),
		m.Ctx.Config.GetFullScreenDiffPagerEnv(),
	)
}
