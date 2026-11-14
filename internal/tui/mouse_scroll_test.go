package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/branchsidebar"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/footer"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issueview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/notificationview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/sidebar"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tabs"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

func newTestMouseModel() Model {
	zone.NewGlobal()
	zone.SetEnabled(true)

	cfg := &config.Config{
		Defaults: config.Defaults{
			Preview: config.PreviewConfig{
				Open:     true,
				Width:    30,
				Position: "right",
			},
		},
		Theme: &config.ThemeConfig{},
	}

	ctx := &context.ProgramContext{
		ScreenWidth:         100,
		ScreenHeight:        30,
		MainContentWidth:    70,
		MainContentHeight:   27,
		DynamicPreviewWidth: 30,
		PreviewPosition:     "right",
		View:                config.PRsView,
		Config:              cfg,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)
	ctx.StartTask = func(task context.Task) tea.Cmd { return nil }

	sb := sidebar.NewModel()
	sb.IsOpen = true
	sb.UpdateProgramContext(ctx)
	sb.SetContent(strings.Repeat("Line content here\n", 40))

	tabsModel := tabs.NewModel(ctx)
	footerModel := footer.NewModel(ctx)

	prSec := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	prSec.Prs = []prrow.Data{
		{Primary: &data.PullRequestData{Number: 1, Title: "PR 1"}},
		{Primary: &data.PullRequestData{Number: 2, Title: "PR 2"}},
		{Primary: &data.PullRequestData{Number: 3, Title: "PR 3"}},
		{Primary: &data.PullRequestData{Number: 4, Title: "PR 4"}},
		{Primary: &data.PullRequestData{Number: 5, Title: "PR 5"}},
	}
	prSec.Table.SetRows(prSec.BuildRows())

	return Model{
		ctx:              ctx,
		keys:             keys.Keys,
		sidebar:          sb,
		tabs:             tabsModel,
		footer:           footerModel,
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		branchSidebar:    branchsidebar.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
		prs:              []section.Section{&prSec},
	}
}

func TestMouseScrollTargeting(t *testing.T) {
	m := newTestMouseModel()

	// 1. Mouse over header tabs (Y < common.TabsHeight)
	msgHeader := tea.MouseWheelMsg{X: 30, Y: 1, Button: tea.MouseWheelDown}
	require.False(t, m.isMouseOverSidebar(msgHeader), "header should not be sidebar")
	require.False(t, m.isMouseOverMainSection(msgHeader), "header should not be main section")

	// 2. Mouse over footer (Y >= ScreenHeight - FooterHeight)
	msgFooter := tea.MouseWheelMsg{X: 30, Y: 29, Button: tea.MouseWheelDown}
	require.False(t, m.isMouseOverSidebar(msgFooter), "footer should not be sidebar")
	require.False(t, m.isMouseOverMainSection(msgFooter), "footer should not be main section")

	// 3. Mouse over main section (X = 35 < 70, Y = 10)
	msgTable := tea.MouseWheelMsg{X: 35, Y: 10, Button: tea.MouseWheelDown}
	require.False(t, m.isMouseOverSidebar(msgTable), "main section should not be sidebar")
	require.True(t, m.isMouseOverMainSection(msgTable), "expected mouse to be over main section")

	// 4. Mouse over sidebar (X = 85 >= 70, Y = 10)
	msgSidebar := tea.MouseWheelMsg{X: 85, Y: 10, Button: tea.MouseWheelDown}
	require.True(t, m.isMouseOverSidebar(msgSidebar), "expected mouse to be over sidebar")
	require.False(t, m.isMouseOverMainSection(msgSidebar), "sidebar should not be main section")

	// 5. When sidebar is closed
	m.sidebar.IsOpen = false
	require.False(t, m.isMouseOverSidebar(msgSidebar), "closed sidebar should never match")
	require.True(t, m.isMouseOverMainSection(msgSidebar), "when sidebar closed, whole width is main section")
}

func TestSidebarMouseWheelScrolling(t *testing.T) {
	m := newTestMouseModel()
	require.Equal(t, 0, m.sidebar.YOffset())

	// Scroll down over sidebar
	msgSidebarDown := tea.MouseWheelMsg{X: 85, Y: 10, Button: tea.MouseWheelDown}
	updated, _ := m.Update(msgSidebarDown)
	m = *updated.(*Model)
	require.Equal(t, 3, m.sidebar.YOffset(), "sidebar should scroll down 3 lines")

	// Scroll down again
	updated, _ = m.Update(msgSidebarDown)
	m = *updated.(*Model)
	require.Equal(t, 6, m.sidebar.YOffset(), "sidebar should scroll down another 3 lines")

	// Scroll up over sidebar
	msgSidebarUp := tea.MouseWheelMsg{X: 85, Y: 10, Button: tea.MouseWheelUp}
	updated, _ = m.Update(msgSidebarUp)
	m = *updated.(*Model)
	require.Equal(t, 3, m.sidebar.YOffset(), "sidebar should scroll up 3 lines")

	// Scroll over header tabs should NOT scroll sidebar
	msgHeader := tea.MouseWheelMsg{X: 85, Y: 0, Button: tea.MouseWheelDown}
	updated, _ = m.Update(msgHeader)
	m = *updated.(*Model)
	require.Equal(t, 3, m.sidebar.YOffset(), "scrolling over header must not scroll sidebar")

	// Scroll over footer should NOT scroll sidebar
	msgFooter := tea.MouseWheelMsg{X: 85, Y: 29, Button: tea.MouseWheelDown}
	updated, _ = m.Update(msgFooter)
	m = *updated.(*Model)
	require.Equal(t, 3, m.sidebar.YOffset(), "scrolling over footer must not scroll sidebar")
}

func TestTableMouseWheelScrolling(t *testing.T) {
	m := newTestMouseModel()
	currSec := m.getCurrSection()
	require.NotNil(t, currSec)
	require.Equal(t, 0, currSec.CurrRow())

	// Mouse over table (X = 35, Y = 10)
	msgTableDown := tea.MouseWheelMsg{X: 35, Y: 10, Button: tea.MouseWheelDown}

	// Scroll down 1 row
	updated, _ := m.Update(msgTableDown)
	m = *updated.(*Model)
	require.Equal(t, 1, m.getCurrSection().CurrRow(), "table should move to row 1")
	require.Equal(t, 0, m.sidebar.YOffset(), "table scroll must not scroll sidebar")

	// Scroll down another row
	updated, _ = m.Update(msgTableDown)
	m = *updated.(*Model)
	require.Equal(t, 2, m.getCurrSection().CurrRow(), "table should move to row 2")

	// Scroll up 1 row
	msgTableUp := tea.MouseWheelMsg{X: 35, Y: 10, Button: tea.MouseWheelUp}
	updated, _ = m.Update(msgTableUp)
	m = *updated.(*Model)
	require.Equal(t, 1, m.getCurrSection().CurrRow(), "table should move back to row 1")

	// Scroll up back to top (row 0)
	updated, _ = m.Update(msgTableUp)
	m = *updated.(*Model)
	require.Equal(t, 0, m.getCurrSection().CurrRow(), "table should be at row 0")

	// Scroll up past top - should stay at 0
	updated, _ = m.Update(msgTableUp)
	m = *updated.(*Model)
	require.Equal(t, 0, m.getCurrSection().CurrRow(), "table should stay at row 0")

	// Scrolling over header should NOT move table row
	msgHeader := tea.MouseWheelMsg{X: 35, Y: 0, Button: tea.MouseWheelDown}
	updated, _ = m.Update(msgHeader)
	m = *updated.(*Model)
	require.Equal(t, 0, m.getCurrSection().CurrRow(), "scrolling over header must not move table row")

	// Scrolling over footer should NOT move table row
	msgFooter := tea.MouseWheelMsg{X: 35, Y: 29, Button: tea.MouseWheelDown}
	updated, _ = m.Update(msgFooter)
	m = *updated.(*Model)
	require.Equal(t, 0, m.getCurrSection().CurrRow(), "scrolling over footer must not move table row")
}
