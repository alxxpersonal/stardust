package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// initRepo makes the cwd a git repo so the hooks command has somewhere to wire.
func initRepo(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, string(out))
		}
	}
}

// runHooksInstall scaffolds .stardust, runs `hooks install`, and returns stderr.
func runHooksInstall(t *testing.T, root string) string {
	t.Helper()
	t.Chdir(root)

	initCmd := newInitCmd()
	require.NoError(t, initCmd.Execute())

	cmd := newHooksCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install"})
	require.NoError(t, cmd.Execute())
	return stderr.String()
}

// TestHooksInstallReportsOwnedMode asserts the install message names owned mode
// and the .stardust/hooks target in a clean repo.
func TestHooksInstallReportsOwnedMode(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)

	out := runHooksInstall(t, root)

	require.Contains(t, out, "owned")
	require.Contains(t, out, ".stardust/hooks")
	require.NotContains(t, out, "composed")
}

// TestHooksInstallReportsComposeMode asserts the install message names compose
// mode and the existing chain's target when a husky-style chain owns the path.
func TestHooksInstallReportsComposeMode(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	cmd := exec.Command("git", "-C", root, "config", "core.hooksPath", ".husky")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("set core.hooksPath: %v: %s", err, string(out))
	}
	huskyDir := filepath.Join(root, ".husky")
	require.NoError(t, os.MkdirAll(huskyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(huskyDir, "post-commit"), []byte("#!/bin/sh\nnpx lint-staged\n"), 0o755))

	out := runHooksInstall(t, root)

	require.Contains(t, out, "composed into")
	require.Contains(t, out, ".husky")
	require.NotContains(t, out, "owned")
}
