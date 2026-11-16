package context

import (
	"path/filepath"
	"strings"
)

// DefaultTokenBudget is the standard budget for prompt + completion tokens.
const DefaultTokenBudget = 32000

// IgnoredFilePatterns contains filename patterns that should be stripped from diff context to save tokens.
var IgnoredFilePatterns = []string{
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"go.sum",
	"devbox.lock",
	"Gemfile.lock",
	"composer.lock",
	"Cargo.lock",
	"poetry.lock",
	"*.min.js",
	"*.min.css",
	"*.map",
	"*.svg",
	"*.png",
	"*.jpg",
	"*.jpeg",
	"*.gif",
	"*.ico",
	"*.webp",
	"*.pdf",
	"*.woff",
	"*.woff2",
	"*.ttf",
	"*.eot",
}

// ShouldIgnoreFile checks if a file path matches any ignored file patterns.
func ShouldIgnoreFile(filePath string) bool {
	base := filepath.Base(filePath)
	cleanPath := strings.ToLower(filePath)
	cleanBase := strings.ToLower(base)

	for _, pattern := range IgnoredFilePatterns {
		if strings.HasPrefix(pattern, "*.") {
			ext := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(cleanPath, ext) {
				return true
			}
		} else if cleanBase == strings.ToLower(pattern) {
			return true
		}
	}
	return false
}

// EstimateTokens provides a lightweight token estimate (approx 4 chars per token).
func EstimateTokens(text string) int {
	return len(text) / 4
}

// SanitizeDiff processes a git/PR diff, removing ignored lockfiles, binaries, and generated files.
func SanitizeDiff(diff string) string {
	if diff == "" {
		return ""
	}

	lines := strings.Split(diff, "\n")
	var cleaned strings.Builder
	skippingFile := false

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				filePath := strings.TrimPrefix(parts[3], "b/")
				if ShouldIgnoreFile(filePath) {
					skippingFile = true
					cleaned.WriteString(line + "\n[Binary or generated lockfile skipped for cost control]\n")
					continue
				} else {
					skippingFile = false
				}
			}
		}
		if !skippingFile {
			cleaned.WriteString(line + "\n")
		}
	}

	return cleaned.String()
}

// ChunkDiff breaks a large diff into smaller chunks if it exceeds maxTokens budget.
func ChunkDiff(diff string, maxTokens int) []string {
	if maxTokens <= 0 {
		maxTokens = 8000
	}

	sanitized := SanitizeDiff(diff)
	if EstimateTokens(sanitized) <= maxTokens {
		return []string{sanitized}
	}

	var chunks []string
	var currentChunk strings.Builder
	fileDiffs := strings.Split(sanitized, "diff --git ")

	for i, fd := range fileDiffs {
		if strings.TrimSpace(fd) == "" {
			continue
		}
		prefix := ""
		if i > 0 || strings.HasPrefix(sanitized, "diff --git ") {
			prefix = "diff --git "
		}
		fullBlock := prefix + fd

		if EstimateTokens(currentChunk.String()+fullBlock) > maxTokens && currentChunk.Len() > 0 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
		}
		currentChunk.WriteString(fullBlock)
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}
