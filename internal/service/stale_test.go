package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/service"
)

func TestStaleDocsListsOnlyStaleImplementedDocs(t *testing.T) {
	ctx := context.Background()
	root := governsVault(t)
	writeGovernedCode(t, root, "internal/foo.go")
	writeGovernedDoc(t, root, "docs/specs/2026-06-22-1000-implemented-spec.md", "Implemented Spec", "spec", "Implemented", "internal/*.go")
	writeGovernedDoc(t, root, "docs/specs/2026-06-22-1100-draft-spec.md", "Draft Spec", "spec", "Draft", "internal/*.go")
	writeGovernedDoc(t, root, "docs/plans/2026-06-22-1200-no-governs-plan.md", "No Governs Plan", "plan", "Implemented", "")
	gitInit(t, root)

	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "foo.go"), []byte("package internal\n\nconst X = 1\n"), 0o644))
	gitCommitAll(t, root, "change foo")

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.StaleDocs(ctx)
	require.NoError(t, err)

	require.Len(t, res.Docs, 1)
	stale := res.Docs[0]
	require.Equal(t, "Implemented Spec", stale.Title)
	require.Equal(t, "spec", stale.Type)
	require.True(t, stale.Stale)
	require.Greater(t, stale.ChangedCommits, 0)
	require.Contains(t, stale.Matched, "internal/foo.go")
	require.NotEmpty(t, stale.DocCommit)
	require.NotEmpty(t, stale.LastCodeCommit)

	require.Contains(t, res.Markdown, "Implemented Spec")
	require.NotContains(t, res.Markdown, "Draft Spec")
	require.NotContains(t, res.Markdown, "No Governs Plan")
}

func TestStaleDocsEmptyWhenNothingStale(t *testing.T) {
	ctx := context.Background()
	root := governsVault(t)
	writeGovernedCode(t, root, "internal/foo.go")
	writeGovernedDoc(t, root, "docs/specs/2026-06-22-1000-fresh-spec.md", "Fresh Spec", "spec", "Implemented", "internal/*.go")
	gitInit(t, root)

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.StaleDocs(ctx)
	require.NoError(t, err)
	require.Empty(t, res.Docs)
	require.Contains(t, res.Markdown, "No stale docs")
}
