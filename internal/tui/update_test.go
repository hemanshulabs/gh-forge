package tui

import (
	"errors"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/hemanshulabs/gh-forge/v4/internal/tui/components/footer"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/components/tabs"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/context"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/keys"
	"github.com/hemanshulabs/gh-forge/v4/internal/tui/theme"
)

func TestUpdateCLI_KeyBinding(t *testing.T) {
	keyMsg := tea.KeyPressMsg{Text: "U"}
	require.True(t, key.Matches(keyMsg, keys.Keys.UpdateCLI))
}

func TestUpdateCLI_UpToDate(t *testing.T) {
	ctx := &context.ProgramContext{
		Version: "v0.1.0",
		Theme:   *theme.DefaultTheme,
		Styles:  context.DefaultStyles,
	}
	m := Model{
		ctx:         ctx,
		keys:        keys.Keys,
		tabs:        tabs.NewModel(ctx),
		footer:      footer.NewModel(ctx),
		tasks:       make(map[string]context.Task),
		taskSpinner: spinner.Model{Spinner: spinner.Dot},
	}

	updatedModel, cmd := m.Update(cliUpdateCheckMsg{latestVersion: "v0.1.0"})
	require.NotNil(t, cmd)
	mPtr := updatedModel.(*Model)
	task, ok := mPtr.tasks["cli_update"]
	require.True(t, ok)
	require.Equal(t, context.TaskFinished, task.State)
	require.Contains(t, task.FinishedText, "forge is up to date (v0.1.0)")
}

func TestUpdateCLI_NewVersionAvailable(t *testing.T) {
	ctx := &context.ProgramContext{
		Version: "v0.1.0",
		Theme:   *theme.DefaultTheme,
		Styles:  context.DefaultStyles,
	}
	m := Model{
		ctx:         ctx,
		keys:        keys.Keys,
		tabs:        tabs.NewModel(ctx),
		footer:      footer.NewModel(ctx),
		tasks:       make(map[string]context.Task),
		taskSpinner: spinner.Model{Spinner: spinner.Dot},
	}

	updatedModel, cmd := m.Update(cliUpdateCheckMsg{latestVersion: "v0.2.0"})
	require.NotNil(t, cmd)
	mPtr := updatedModel.(*Model)
	task, ok := mPtr.tasks["cli_update"]
	require.True(t, ok)
	require.Equal(t, context.TaskStart, task.State)
	require.Contains(t, task.StartText, "New version v0.2.0 available! Updating forge...")
	require.Equal(t, "v0.2.0", mPtr.tabs.LatestVersion())
}

func TestUpdateCLI_UpgradeFinishedSuccess(t *testing.T) {
	ctx := &context.ProgramContext{
		Version: "v0.1.0",
		Theme:   *theme.DefaultTheme,
		Styles:  context.DefaultStyles,
	}
	m := Model{
		ctx:         ctx,
		keys:        keys.Keys,
		tabs:        tabs.NewModel(ctx),
		footer:      footer.NewModel(ctx),
		tasks:       make(map[string]context.Task),
		taskSpinner: spinner.Model{Spinner: spinner.Dot},
	}

	updatedModel, cmd := m.Update(cliUpgradeFinishedMsg{version: "v0.2.0"})
	require.NotNil(t, cmd)
	mPtr := updatedModel.(*Model)
	task, ok := mPtr.tasks["cli_update"]
	require.True(t, ok)
	require.Equal(t, context.TaskFinished, task.State)
	require.Contains(t, task.FinishedText, "Updated to v0.2.0! Please restart forge.")
	require.Equal(t, "v0.2.0", mPtr.ctx.Version)
	require.Equal(t, "v0.2.0", mPtr.tabs.LatestVersion())
}

func TestUpdateCLI_UpgradeFinishedError(t *testing.T) {
	ctx := &context.ProgramContext{
		Version: "v0.1.0",
		Theme:   *theme.DefaultTheme,
		Styles:  context.DefaultStyles,
	}
	m := Model{
		ctx:         ctx,
		keys:        keys.Keys,
		tabs:        tabs.NewModel(ctx),
		footer:      footer.NewModel(ctx),
		tasks:       make(map[string]context.Task),
		taskSpinner: spinner.Model{Spinner: spinner.Dot},
	}

	testErr := errors.New("network failure")
	updatedModel, cmd := m.Update(cliUpgradeFinishedMsg{version: "v0.2.0", err: testErr})
	require.NotNil(t, cmd)
	mPtr := updatedModel.(*Model)
	task, ok := mPtr.tasks["cli_update"]
	require.True(t, ok)
	require.Equal(t, context.TaskError, task.State)
	require.Equal(t, testErr, task.Error)
}

func TestUpdateCLI_CheckError(t *testing.T) {
	ctx := &context.ProgramContext{
		Version: "v0.1.0",
		Theme:   *theme.DefaultTheme,
		Styles:  context.DefaultStyles,
	}
	m := Model{
		ctx:         ctx,
		keys:        keys.Keys,
		tabs:        tabs.NewModel(ctx),
		footer:      footer.NewModel(ctx),
		tasks:       make(map[string]context.Task),
		taskSpinner: spinner.Model{Spinner: spinner.Dot},
	}

	testErr := errors.New("cannot connect to api")
	updatedModel, cmd := m.Update(cliUpdateCheckMsg{err: testErr})
	require.NotNil(t, cmd)
	mPtr := updatedModel.(*Model)
	task, ok := mPtr.tasks["cli_update"]
	require.True(t, ok)
	require.Equal(t, context.TaskError, task.State)
	require.Contains(t, task.Error.Error(), "failed checking for updates")
}
