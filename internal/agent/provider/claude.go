package provider

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
)

// ClaudeCLIAdapter runs Claude Code in headless mode (-p/--print).
// File/tool access is strictly disabled; input is provided via prompt and JSON is extracted from output.
type ClaudeCLIAdapter struct {
	binaryPath string
}

func NewClaudeCLIAdapter() *ClaudeCLIAdapter {
	path, _ := exec.LookPath("claude")
	return &ClaudeCLIAdapter{binaryPath: path}
}

func (c *ClaudeCLIAdapter) ID() string {
	return "claude-cli"
}

func (c *ClaudeCLIAdapter) Name() string {
	return "Claude Code CLI (Headless)"
}

func (c *ClaudeCLIAdapter) Description() string {
	return "Executes prompts via locally installed 'claude' CLI in headless mode without filesystem tools."
}

func (c *ClaudeCLIAdapter) IsAvailable(ctx context.Context) bool {
	if c.binaryPath == "" {
		path, err := exec.LookPath("claude")
		if err != nil {
			return false
		}
		c.binaryPath = path
	}
	return true
}

func claudeCLIArgs(model string) []string {
	args := []string{
		"--safe-mode", "--tools", "", "--disallowedTools", "mcp__*",
		"--disable-slash-commands", "--no-session-persistence", "--max-turns", "1", "-p",
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
}

func (c *ClaudeCLIAdapter) Complete(ctx context.Context, req CompletionRequest) (*agent.AgentResult, error) {
	if !c.IsAvailable(ctx) {
		return nil, fmt.Errorf("claude CLI is not installed or not found in PATH")
	}

	var fullPrompt strings.Builder
	fullPrompt.WriteString("SYSTEM INSTRUCTIONS:\n")
	fullPrompt.WriteString(req.SystemPrompt)
	fullPrompt.WriteString("\n\nSTRICT JSON OUTPUT REQUIREMENT:\n")
	fullPrompt.WriteString("You MUST respond ONLY with a valid JSON object adhering to the schema below. Do NOT execute any tools or write files.\n")
	fullPrompt.WriteString(req.JSONSchema)
	fullPrompt.WriteString("\n\nUSER PROMPT:\n")
	fullPrompt.WriteString(req.UserPrompt)

	// The prompt already contains all relevant repository context. Run from the
	// system temp directory and explicitly disable built-in and MCP tools so this
	// adapter cannot edit files or execute project commands.
	cmd := exec.CommandContext(ctx, c.binaryPath, claudeCLIArgs(req.Model)...)
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader(fullPrompt.String())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput == "" {
			errOutput = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("claude CLI error (%v): %s", err, errOutput)
	}

	rawOutput := stdout.String()
	result, err := agent.ParseAgentResult(rawOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse claude CLI output as JSON: %w (output: %q)", err, rawOutput)
	}
	result.RawResult = rawOutput
	if result.Role == "" {
		result.Role = req.Role
	}

	return result, nil
}
