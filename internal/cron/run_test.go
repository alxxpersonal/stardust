package cron

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentJob builds a kind=agent job backed by a real prompt file on disk.
func agentJob(t *testing.T, runner, model, sandbox string) Job {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prompt.md"), []byte("do the thing"), 0o644))
	return Job{
		Name: "j",
		Dir:  dir,
		Run:  Run{Kind: "agent", Prompt: "prompt.md", Runner: runner, Model: model, Sandbox: sandbox},
	}
}

func TestBuildCommandCodexDefault(t *testing.T) {
	cmd, err := agentJob(t, "", "", "").buildCommand(context.Background(), "stardust")
	require.NoError(t, err)
	args := cmd.Args
	assert.Equal(t, "codex", filepath.Base(args[0]), "empty runner defaults to codex")
	assert.Contains(t, args, "exec")
	assert.Contains(t, args, "--ignore-user-config")
	assert.Contains(t, args, "--skip-git-repo-check")
	assert.Contains(t, args, "-m")
	assert.Contains(t, args, DefaultCodexModel, "defaults to the pinned codex model")
	assert.Equal(t, "do the thing", args[len(args)-1], "prompt is the final arg")
}

func TestBuildCommandCodexModelAndSandbox(t *testing.T) {
	cmd, err := agentJob(t, "codex", "gpt-5.5-mini", "read-only").buildCommand(context.Background(), "stardust")
	require.NoError(t, err)
	assert.Contains(t, cmd.Args, "gpt-5.5-mini")
	assert.Contains(t, cmd.Args, "--sandbox")
	assert.Contains(t, cmd.Args, "read-only")
}

func TestBuildCommandClaude(t *testing.T) {
	cmd, err := agentJob(t, "claude", "opus", "").buildCommand(context.Background(), "stardust")
	require.NoError(t, err)
	args := cmd.Args
	assert.Equal(t, "claude", filepath.Base(args[0]))
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "--permission-mode")
	assert.Contains(t, args, "bypassPermissions")
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "opus")
	assert.Equal(t, "do the thing", args[len(args)-1])
}

func TestValidateRejectsBadRunner(t *testing.T) {
	j := Job{Run: Run{Kind: "agent", Prompt: "p.md", Runner: "gpt"}}
	assert.Error(t, j.validate(), "runner must be codex or claude")
}

func TestResolvedRunnerLegacyDefault(t *testing.T) {
	assert.Equal(t, "codex", Job{Run: Run{Kind: "agent"}}.ResolvedRunner())
	assert.Equal(t, "claude", Job{Run: Run{Kind: "agent", Runner: "claude"}}.ResolvedRunner())
}
