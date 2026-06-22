package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/vault"
)

func TestNewDocCreatesSpecWithConventionFrontmatter(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	writeGovernCollection(t, root, "specs", "docs/specs")

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.NewDoc(ctx, service.NewDocOptions{
		Kind:    "spec",
		Title:   "Agent Infra",
		Status:  "Draft",
		Governs: []string{"internal/service/*.go"},
		Now:     time.Date(2026, 6, 22, 22, 38, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, "docs/specs/2026-06-22-2238-agent-infra.md", res.Path)

	note, err := vault.Parse(root, res.Path)
	require.NoError(t, err)
	require.Equal(t, "Agent Infra", note.Frontmatter["title"])
	require.Equal(t, "spec", note.Frontmatter["type"])
	require.Equal(t, "Draft", note.Frontmatter["status"])
	require.Equal(t, "2026-06-22", note.Frontmatter["created"])
	require.Equal(t, "2026-06-22", note.Frontmatter["updated"])
	require.Equal(t, []any{"internal/service/*.go"}, note.Frontmatter["governs"])
}

func TestNewDocCreatesNextADRNumber(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	writeGovernCollection(t, root, "adr", "docs/adr")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "adr"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "adr", "0001-existing.md"), []byte("---\ntitle: Existing\ntype: adr\nstatus: Accepted\n---\n# Existing\n"), 0o644))

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.NewDoc(ctx, service.NewDocOptions{
		Kind:   "adr",
		Title:  "Choose SQLite",
		Status: "Proposed",
		Now:    time.Date(2026, 6, 22, 22, 38, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, "docs/adr/0002-choose-sqlite.md", res.Path)
}

func TestNewDocRejectsInvalidStatus(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	writeGovernCollection(t, root, "specs", "docs/specs")

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	_, err = svc.NewDoc(ctx, service.NewDocOptions{Kind: "spec", Title: "Bad", Status: "Weird"})
	require.Error(t, err)
}
