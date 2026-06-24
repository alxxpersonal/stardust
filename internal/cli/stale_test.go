package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

func TestRegistryStaleCmd(t *testing.T) {
	root := staleDocsRepo(t)
	t.Setenv("STARDUST_VAULT", root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"registry", "stale", "--output", "plain"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), "Stale Spec")
	require.Contains(t, out.String(), "docs/specs/2026-06-22-1000-stale-spec.md")
	require.Contains(t, out.String(), "internal/service/check.go")
}

func TestRegistryStaleCmdJSON(t *testing.T) {
	root := staleDocsRepo(t)
	t.Setenv("STARDUST_VAULT", root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"registry", "stale", "--output", "json"})
	require.NoError(t, cmd.Execute())

	var got struct {
		Docs []struct {
			DocPath        string `json:"doc_path"`
			ChangedCommits int    `json:"changed_commits"`
		} `json:"docs"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Docs, 1)
	require.Equal(t, "docs/specs/2026-06-22-1000-stale-spec.md", got.Docs[0].DocPath)
	require.Greater(t, got.Docs[0].ChangedCommits, 0)
}

func TestRegistryStaleCmdExitCode(t *testing.T) {
	root := staleDocsRepo(t)
	t.Setenv("STARDUST_VAULT", root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"registry", "stale", "--output", "plain", "--exit-code"})
	require.Error(t, cmd.Execute())
}

// staleDocsRepo builds a temp vault with a "specs" collection, an Implemented
// spec governing internal/service/*.go, and a code change committed after the
// doc so the spec reads as stale. It mirrors governsDocsRepo.
func staleDocsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))

	dir := filepath.Join(root, ".stardust", "collections", "specs")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	cfg := "path = \"docs/specs\"\n" +
		"description = \"docs specs\"\n\n" +
		"[[fields]]\nname = \"title\"\ntype = \"string\"\nrequired = true\n\n" +
		"[[fields]]\nname = \"status\"\ntype = \"string\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0o644))

	codePath := filepath.Join(root, "internal", "service", "check.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(codePath), 0o755))
	require.NoError(t, os.WriteFile(codePath, []byte("package service\n"), 0o644))

	specsDir := filepath.Join(root, "docs", "specs")
	require.NoError(t, os.MkdirAll(specsDir, 0o755))
	note := "---\ntitle: Stale Spec\nstatus: Implemented\ngoverns: [\"internal/service/*.go\"]\n---\n\n# Stale Spec\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "2026-06-22-1000-stale-spec.md"), []byte(note), 0o644))

	gitInitRepo(t, root)
	require.NoError(t, os.WriteFile(codePath, []byte("package service\n\nconst Y = 2\n"), 0o644))
	gitCommit(t, root, "change check")

	ctx := t.Context()
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	return root
}

func gitInitRepo(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}, {"config", "commit.gpgsign", "false"},
		{"add", "-A"}, {"commit", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}
}

func gitCommit(t *testing.T, root, message string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-m", message},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}
}
