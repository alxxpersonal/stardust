package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

// docsRepo builds a temp vault with a "specs" collection mapped at docs/specs
// and one timestamped sample doc carrying title/status frontmatter, indexes it,
// and returns the vault root. It mirrors the service package test harness.
func docsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))

	dir := filepath.Join(root, ".stardust", "collections", "specs")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	cfg := "path = \"docs/specs\"\n" +
		"description = \"docs specs\"\n\n" +
		"[[fields]]\nname = \"title\"\ntype = \"string\"\nrequired = true\n\n" +
		"[[fields]]\nname = \"status\"\ntype = \"enum\"\nenum = [\"Draft\", \"Approved\"]\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0o644))

	specsDir := filepath.Join(root, "docs", "specs")
	require.NoError(t, os.MkdirAll(specsDir, 0o755))
	note := "---\ntitle: First Spec\nstatus: Approved\n---\n\n# First Spec\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "2026-06-22-2238-first-spec.md"), []byte(note), 0o644))

	ctx := t.Context()
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	return root
}

func TestRegistryCmd(t *testing.T) {
	root := docsRepo(t)
	t.Setenv("STARDUST_VAULT", root)

	cmd := newRegistryCmd()
	cmd.SetArgs([]string{"--output", "docs/INDEX.md"})
	require.NoError(t, cmd.Execute())

	out := filepath.Join(root, "docs", "INDEX.md")
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	got := string(data)

	require.Contains(t, got, "# Docs Index")
	require.Contains(t, got, "## Specs")
	require.Contains(t, got, "| Title | Status | Doc | Date |")
	require.Contains(t, got, "| First Spec | Approved | docs/specs/2026-06-22-2238-first-spec.md | 2026-06-22 |")
	manifestData, err := os.ReadFile(config.Layout{Root: root}.Manifest())
	require.NoError(t, err)
	require.Contains(t, string(manifestData), "docs/INDEX.md")

	// Idempotent: a second run produces byte-identical output.
	cmd2 := newRegistryCmd()
	cmd2.SetArgs([]string{"--output", "docs/INDEX.md"})
	require.NoError(t, cmd2.Execute())
	data2, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Equal(t, data, data2)
}

func TestRegistryGovernsCmd(t *testing.T) {
	root := governsDocsRepo(t)
	t.Setenv("STARDUST_VAULT", root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"registry", "governs", "internal/service/check.go", "--output", "plain"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), "Check Spec")
	require.Contains(t, out.String(), "docs/specs/2026-06-22-1000-check-spec.md")
	require.Contains(t, out.String(), "internal/service/check.go")
}

func TestRegistryGovernsCmdJSON(t *testing.T) {
	root := governsDocsRepo(t)
	t.Setenv("STARDUST_VAULT", root)
	var out bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"registry", "governs", "internal/service/check.go", "--output", "json"})
	require.NoError(t, cmd.Execute())

	var got struct {
		Docs []struct {
			DocPath string `json:"doc_path"`
		} `json:"docs"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Equal(t, "docs/specs/2026-06-22-1000-check-spec.md", got.Docs[0].DocPath)
}

func governsDocsRepo(t *testing.T) string {
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
	note := "---\ntitle: Check Spec\nstatus: Approved\ngoverns: [\"internal/service/*.go\"]\n---\n\n# Check Spec\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "2026-06-22-1000-check-spec.md"), []byte(note), 0o644))

	ctx := t.Context()
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	return root
}
