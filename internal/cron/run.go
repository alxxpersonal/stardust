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
		args := []string{"exec"}
		if j.Run.Model != "" {
			args = append(args, "-m", j.Run.Model)
		}
		if j.Run.Sandbox != "" {
			args = append(args, "--sandbox", j.Run.Sandbox)
		}
		args = append(args, string(prompt))
		return exec.CommandContext(ctx, "codex", args...), nil
	default:
		return nil, fmt.Errorf("unknown run kind %q", j.Run.Kind)
	}
}
