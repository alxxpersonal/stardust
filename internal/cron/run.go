package cron

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Execute runs the job now, streaming combined output to w and to a timestamped
// log under the job's runs/ directory. stardustBin is re-execed for command
// jobs; root is the working directory (the vault root).
func (j Job) Execute(ctx context.Context, stardustBin, root string, w io.Writer) error {
	runsDir := filepath.Join(j.Dir, "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return fmt.Errorf("create runs dir: %w", err)
	}
	logPath := filepath.Join(runsDir, time.Now().Format("20060102-150405")+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create run log: %w", err)
	}
	defer func() { _ = logFile.Close() }()
	out := io.MultiWriter(w, logFile)

	cmd, err := j.buildCommand(ctx, stardustBin)
	if err != nil {
		return err
	}
	cmd.Dir = root
	cmd.Stdout = out
	cmd.Stderr = out
	fmt.Fprintf(out, "[stardust cron] running %q (%s)\n", j.Name, j.Run.Kind)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("job %s failed: %w", j.Name, err)
	}
	return nil
}

// buildCommand constructs the exec.Cmd for the job's run kind.
func (j Job) buildCommand(ctx context.Context, stardustBin string) (*exec.Cmd, error) {
	switch j.Run.Kind {
	case "command":
		return exec.CommandContext(ctx, stardustBin, strings.Fields(j.Run.Command)...), nil
	case "exec":
		return exec.CommandContext(ctx, "sh", "-c", j.Run.Exec), nil
	case "agent":
		prompt, err := os.ReadFile(filepath.Join(j.Dir, j.Run.Prompt))
		if err != nil {
			return nil, fmt.Errorf("read agent prompt: %w", err)
		}
		if j.ResolvedRunner() == "claude" {
			return j.buildClaudeCommand(ctx, string(prompt)), nil
		}
		return j.buildCodexCommand(ctx, string(prompt)), nil
	default:
		return nil, fmt.Errorf("unknown run kind %q", j.Run.Kind)
	}
}

// buildCodexCommand runs the prompt as a codex agent. --ignore-user-config
// skips the user's ~/.codex config (which may carry an unsupported service_tier
// and auto-load MCP servers); the model defaults to a ChatGPT-account-supported
// one when the job names none.
func (j Job) buildCodexCommand(ctx context.Context, prompt string) *exec.Cmd {
	model := j.Run.Model
	if model == "" {
		model = DefaultCodexModel
	}
	args := []string{"exec", "--skip-git-repo-check", "--ignore-user-config", "-m", model}
	if j.Run.Sandbox != "" {
		args = append(args, "--sandbox", j.Run.Sandbox)
	}
	args = append(args, prompt)
	return exec.CommandContext(ctx, "codex", args...)
}

// buildClaudeCommand runs the prompt as an unattended claude agent. The daemon
// has no TTY to approve tool permissions, so scheduled jobs run autonomously
// (--permission-mode bypassPermissions); they are user-authored automations.
func (j Job) buildClaudeCommand(ctx context.Context, prompt string) *exec.Cmd {
	args := []string{"-p", "--permission-mode", "bypassPermissions"}
	if j.Run.Model != "" {
		args = append(args, "--model", j.Run.Model)
	}
	args = append(args, prompt)
	return exec.CommandContext(ctx, "claude", args...)
}
