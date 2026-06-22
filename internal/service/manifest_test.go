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
