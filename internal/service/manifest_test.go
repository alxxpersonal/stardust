package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

func TestRefreshManifestWritesAgentManifest(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	writeGovernCollection(t, root, "plans", "docs/plans")
	plansDir := filepath.Join(root, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, "2026-06-22-1000-agent-infra.md"), []byte("---\ntitle: Agent Infra Plan\ntype: plan\nstatus: Active\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Agent Infra Plan\n"), 0o644))

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	require.NoError(t, svc.RefreshManifest(ctx))
	data, err := os.ReadFile(config.Layout{Root: root}.Manifest())
	require.NoError(t, err)
	require.Contains(t, string(data), "Agent Infra Plan")
	require.Contains(t, string(data), "docs/INDEX.md")
}

func TestRefreshManifestRendersRichStaleDrift(t *testing.T) {
	ctx := context.Background()
	root := governsVault(t)
	writeGovernedCode(t, root, "internal/foo.go")
	writeGovernedDoc(t, root, "docs/specs/2026-06-22-1000-implemented-spec.md", "Implemented Spec", "spec", "Implemented", "internal/*.go")
	gitInit(t, root)

	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "foo.go"), []byte("package internal\n\nconst X = 1\n"), 0o644))
	gitCommitAll(t, root, "change foo")

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	require.NoError(t, svc.RefreshManifest(ctx))
	data, err := os.ReadFile(config.Layout{Root: root}.Manifest())
	require.NoError(t, err)
	got := string(data)

	require.Contains(t, got, "Implemented Spec")
	require.Contains(t, got, "internal/foo.go")
	require.Contains(t, got, "commit")
}

// TestRefreshManifestRendersReferenceDrift asserts the manifest carries a drift
// line for a doc that references moved code through a reference binding, ungated
// by an Implemented status.
func TestRefreshManifestRendersReferenceDrift(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	writeGovernedCode(t, root, "internal/store/daemon.go")
	writeReferencingDoc(t, root, "docs/adr/0001-daemon.md", "Daemon ADR", "adr", "Proposed", "internal/store/daemon.go")
	gitInit(t, root)
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "store", "daemon.go"), []byte("package store\n\nconst X = 1\n"), 0o644))
	gitCommitAll(t, root, "edit daemon")

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	require.NoError(t, svc.RefreshManifest(ctx))
	data, err := os.ReadFile(config.Layout{Root: root}.Manifest())
	require.NoError(t, err)
	got := string(data)

	require.Contains(t, got, "Daemon ADR")
	require.Contains(t, got, "internal/store/daemon.go")
	require.Contains(t, got, "review")
}
