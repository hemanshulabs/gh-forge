package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeCLIArgsDisableToolsAndPersistence(t *testing.T) {
	args := claudeCLIArgs("model-x")
	require.Contains(t, args, "--safe-mode")
	require.Contains(t, args, "--no-session-persistence")
	require.Contains(t, args, "--disable-slash-commands")
	require.Contains(t, args, "mcp__*")
	require.Contains(t, args, "model-x")

	toolsIndex := -1
	for i, arg := range args {
		if arg == "--tools" {
			toolsIndex = i
			break
		}
	}
	require.GreaterOrEqual(t, toolsIndex, 0)
	require.Less(t, toolsIndex+1, len(args))
	require.Empty(t, args[toolsIndex+1])
}
