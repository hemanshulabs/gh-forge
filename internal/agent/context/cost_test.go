package context_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent/context"
)

func TestShouldIgnoreFile(t *testing.T) {
	require.True(t, context.ShouldIgnoreFile("package-lock.json"))
	require.True(t, context.ShouldIgnoreFile("path/to/go.sum"))
	require.True(t, context.ShouldIgnoreFile("bundle.min.js"))
	require.True(t, context.ShouldIgnoreFile("assets/logo.png"))
	require.False(t, context.ShouldIgnoreFile("main.go"))
	require.False(t, context.ShouldIgnoreFile("internal/auth/token.go"))
}

func TestSanitizeDiff(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
index 123..456 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
+package main
diff --git a/go.sum b/go.sum
index 789..abc 100644
--- a/go.sum
+++ b/go.sum
@@ -1,1 +1,2 @@
+github.com/some/pkg v1.0.0
`

	sanitized := context.SanitizeDiff(diff)
	require.Contains(t, sanitized, "diff --git a/main.go b/main.go")
	require.Contains(t, sanitized, "+package main")
	require.Contains(t, sanitized, "[Binary or generated lockfile skipped for cost control]")
	require.NotContains(t, sanitized, "github.com/some/pkg v1.0.0")
}

func TestChunkDiff(t *testing.T) {
	diff := strings.Repeat("diff --git a/file1.go b/file1.go\n+some code line\n", 50)
	chunks := context.ChunkDiff(diff, 100)
	require.Greater(t, len(chunks), 1)
}
