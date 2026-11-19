package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hemanshulabs/gh-forge/v4/internal/agent"
)

// RestrictedCLIAdapter invokes a user-installed coding CLI in a mode intended
// for read-only analysis. Prompts contain the evidence, so the process runs in
// an empty temporary directory rather than the user's repository.
type RestrictedCLIAdapter struct {
	id, name, description, binary string
}

func NewRestrictedCLIAdapter(id, name, description, binary string) *RestrictedCLIAdapter {
	return &RestrictedCLIAdapter{id: id, name: name, description: description, binary: binary}
}

func (c *RestrictedCLIAdapter) ID() string          { return c.id }
func (c *RestrictedCLIAdapter) Name() string        { return c.name }
func (c *RestrictedCLIAdapter) Description() string { return c.description }
func (c *RestrictedCLIAdapter) IsAvailable(context.Context) bool {
	_, err := exec.LookPath(c.binary)
	return err == nil
}

func restrictedCLIArgs(id, prompt, model, configDir string) ([]string, []string) {
	switch id {
	case "codex-cli":
		args := []string{"exec", "--sandbox", "read-only", "--ask-for-approval", "never", "--skip-git-repo-check"}
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(args, "-"), nil
	case "gemini-cli":
		args := []string{"--approval-mode", "plan", "--prompt", prompt, "--output-format", "text", "--extensions", ""}
		if model != "" {
			args = append(args, "--model", model)
		}
		return args, nil
	case "opencode-cli":
		// Runtime config has highest normal precedence and denies every tool
		// except read. The process also runs outside the user's repository.
		policy := `{"autoupdate":false,"permissions":[{"action":"*","resource":"*","effect":"deny"},{"action":"read","resource":"*","effect":"allow"}]}`
		// The JSON mode emits an event stream rather than one AgentResult object.
		// Keep the default text mode so the model's structured JSON answer is the stdout payload.
		args := []string{"run"}
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(args, prompt), []string{"OPENCODE_CONFIG_CONTENT=" + policy, "OPENCODE_CONFIG_DIR=" + configDir}
	case "agy-cli":
		// Send prompts over stdin. Windows limits the complete process command
		// line, and diff chunks can be much larger than that limit.
		args := []string{
			"--mode", "plan", "--sandbox", "--disable-slash-commands",
			"--input-format", "stream-json", "--output-format", "stream-json",
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		return args, nil
	default:
		return nil, nil
	}
}

func (c *RestrictedCLIAdapter) Complete(ctx context.Context, req CompletionRequest) (*agent.AgentResult, error) {
	binary, err := exec.LookPath(c.binary)
	if err != nil {
		return nil, fmt.Errorf("%s CLI not found in PATH", c.name)
	}
	workDir, err := os.MkdirTemp("", "forge-agent-*")
	if err != nil {
		return nil, fmt.Errorf("create isolated CLI working directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	configDir := filepath.Join(workDir, "config")
	if err := os.Mkdir(configDir, 0700); err != nil {
		return nil, fmt.Errorf("create isolated CLI config: %w", err)
	}

	prompt := "SYSTEM INSTRUCTIONS:\n" + req.SystemPrompt + "\n\nReturn only a JSON object matching this schema. Do not use tools, execute commands, or modify files.\n" + req.JSONSchema + "\n\nUSER PROMPT:\n" + req.UserPrompt
	args, extraEnv := restrictedCLIArgs(c.id, prompt, req.Model, configDir)
	if c.id == "agy-cli" {
		args = append(args, "--json-schema", req.JSONSchema)
		message, marshalErr := json.Marshal(map[string]any{
			"event":   "user",
			"message": map[string]string{"content": prompt},
		})
		if marshalErr != nil {
			return nil, fmt.Errorf("encode Antigravity prompt: %w", marshalErr)
		}
		prompt = string(message) + "\n"
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	if c.id == "codex-cli" || c.id == "agy-cli" {
		cmd.Stdin = strings.NewReader(prompt)
	}
	cmd.Env = os.Environ()
	for _, assignment := range extraEnv {
		if key, value, ok := strings.Cut(assignment, "="); ok {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if len(detail) > 1200 {
			detail = detail[:1200] + "..."
		}
		return nil, fmt.Errorf("%s CLI error (%v): %s", c.name, err, detail)
	}
	rawOutput := stdout.String()
	if c.id == "agy-cli" {
		rawOutput, err = antigravityResponse(rawOutput)
		if err != nil {
			return nil, fmt.Errorf("parse %s CLI output: %w", c.name, err)
		}
	}
	result, err := agent.ParseAgentResult(rawOutput)
	if err != nil {
		return nil, fmt.Errorf("parse %s CLI output: %w", c.name, err)
	}
	result.RawResult = rawOutput
	if result.Role == "" {
		result.Role = req.Role
	}
	return result, nil
}

// antigravityResponse extracts the final result payload from AGY's NDJSON
// event stream. The CLI wraps model output in result.response, and may also
// provide schema-validated structured_output.
func antigravityResponse(output string) (string, error) {
	type resultEvent struct {
		Event  string `json:"event"`
		Result struct {
			Status           string          `json:"status"`
			Response         string          `json:"response"`
			Error            string          `json:"error"`
			StructuredOutput json.RawMessage `json:"structured_output"`
		} `json:"result"`
	}
	var final *resultEvent
	scanner := bufio.NewScanner(strings.NewReader(output))
	// Increase Scanner's default 64 KiB limit for large structured findings.
	scanner.Buffer(make([]byte, 4096), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event resultEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Event == "result" {
			final = &event
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read result stream: %w", err)
	}
	if final == nil {
		return "", fmt.Errorf("Antigravity output did not contain a final result event")
	}
	if !strings.EqualFold(final.Result.Status, "SUCCESS") {
		if final.Result.Error != "" {
			return "", fmt.Errorf("Antigravity run ended with status %s: %s", final.Result.Status, final.Result.Error)
		}
		return "", fmt.Errorf("Antigravity run ended with status %s", final.Result.Status)
	}
	if len(final.Result.StructuredOutput) > 0 && string(final.Result.StructuredOutput) != "null" {
		return string(final.Result.StructuredOutput), nil
	}
	if strings.TrimSpace(final.Result.Response) == "" {
		return "", fmt.Errorf("Antigravity result contained no response")
	}
	return final.Result.Response, nil
}
