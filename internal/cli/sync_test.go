package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncDryRunCmdPrintsPlanAndCreatesNothing(t *testing.T) {
	root := syncTestVault(t)
	t.Setenv("STARDUST_VAULT", root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"sync", "--dry-run", "--scope", "repo", "--tool", "claude", "--output", "plain"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), "Agent Sync Plan")
	require.Contains(t, out.String(), "| skill | foo | claude | repo | create |")
	_, err := os.Lstat(filepath.Join(root, ".claude", "skills", "foo"))
	require.True(t, os.IsNotExist(err), "dry-run must not create sync target")
}

func TestSyncCheckCmdFailsOnDrift(t *testing.T) {
	root := syncTestVault(t)
	t.Setenv("STARDUST_VAULT", root)
	var out bytes.Buffer
	target := filepath.Join(root, ".claude", "skills", "foo")
	other := filepath.Join(root, "other")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.MkdirAll(other, 0o755))
	require.NoError(t, os.Symlink(other, target))

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"sync", "--check", "--scope", "repo", "--tool", "claude", "--output", "plain"})
	require.Error(t, cmd.Execute())
}

func TestSyncInitMigrationDryRunPrintsConfig(t *testing.T) {
	root := t.TempDir()
	_, err := scaffoldVault(t.Context(), root, "off", false)
	require.NoError(t, err)
	t.Setenv("STARDUST_VAULT", root)
	t.Setenv("HOME", "/home/user")
	t.Chdir(root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"sync", "init", "--profile", "migration", "--canonical", "~/agent-assets", "--dry-run"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), "/home/user/agent-assets/skills")
	require.Contains(t, out.String(), "import_only = true")
	_, err = os.Stat(filepath.Join(root, ".stardust", "sync.toml"))
	require.True(t, os.IsNotExist(err), "dry-run must not write sync config")
}

func syncTestVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	_, err := scaffoldVault(t.Context(), root, "off", false)
	require.NoError(t, err)

	source := filepath.Join(root, "skills")
	require.NoError(t, os.MkdirAll(filepath.Join(source, "foo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "foo", "SKILL.md"), []byte("---\nname: foo\n---\n# Foo\n"), 0o644))

	body := "default_targets = [\"claude\"]\n\n" +
		"[[sources]]\n" +
		"name = \"skills\"\n" +
		"path = \"" + filepath.ToSlash(source) + "\"\n" +
		"kind = \"skill\"\n" +
		"priority = 10\n\n" +
		"[[targets]]\n" +
		"tool = \"claude\"\n" +
		"scope = \"repo\"\n" +
		"skills_path = \".claude/skills\"\n" +
		"agents_path = \".claude/agents\"\n" +
		"mode = \"symlink\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, ".stardust", "sync.toml"), []byte(body), 0o644))
	return root
}
