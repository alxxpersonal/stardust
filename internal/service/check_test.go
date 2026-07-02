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

func TestCheckFindsIssues(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	write("a.md", "---\ntitle: A\n---\n# A\nlinks to [[nonexistent]]") // broken link (error)
	write("b.md", "---\ntitle: [bad\n---\n# B\n")                      // bad frontmatter (error)
	write("orphan.md", "---\ntitle: Orphan\n---\n# Orphan\nno links")  // orphan (warn)

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Errors, 2) // broken link + bad frontmatter
	require.Greater(t, res.Warnings, 0)      // orphans

	var foundBroken, foundBadFM bool
	for _, is := range res.Issues {
		switch is.Kind {
		case "broken-link":
			foundBroken = true
		case "bad-frontmatter":
			foundBadFM = true
		}
	}
	require.True(t, foundBroken)
	require.True(t, foundBadFM)
}

func TestCheckCleanVault(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.WriteFile(filepath.Join(root, "x.md"), []byte("---\ntitle: X\n---\n# X\nsee [[y]]"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "y.md"), []byte("---\ntitle: Y\n---\n# Y\nsee [[x]]"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, res.Errors)
	require.Equal(t, 0, res.Warnings) // both linked, both titled, no dupes
}

func TestCheckPlainWikiVault(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write("Home.md", "see [[Page Name]] and [[Plain Wiki Doc]]")
	write("Page-Name.md", "plain wiki page with filename title")
	write("_Sidebar.md", "[Home](Home)")
	write("docs/specs/Plain-Wiki-Doc.md", "plain nested wiki page")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.False(t, hasCheckIssue(res.Issues, "broken-link"))
	require.False(t, hasCheckIssue(res.Issues, "missing-title"))
	require.False(t, hasCheckIssue(res.Issues, "stray-doc"))
	require.False(t, hasCheckIssue(res.Issues, "missing-doc-field"))
	require.False(t, hasCheckIssue(res.Issues, "orphan"))
}

func TestCheckPlainWikiReportsBrokenMarkdownLinks(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Home.md"), []byte("# Home\n\n[Missing](No-Such-Page)\n"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.True(t, hasCheckIssue(res.Issues, "broken-link"))
	require.Contains(t, res.Markdown, "markdown link")
}

func TestCheckIncludesConventionIssues(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/probe\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "specs", "bad-name.md"), []byte("---\ntitle: Bad\ntype: spec\nstatus: Weird\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Bad\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "skills", "foo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "skills", "foo", "SKILL.md"), []byte("---\nname: foo\ntargets: [wat]\n---\n# Foo\n"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Errors, 2)
	require.True(t, hasCheckIssue(res.Issues, "bad-doc-status"))
	require.True(t, hasCheckIssue(res.Issues, "bad-target"))
}

// TestCheckNonMarkdownPagesSkipTitleRule pins ADR 0041: in a docs-convention
// repo the explicit-title requirement applies to markdown docs only. A
// non-markdown page titles itself from its filename and must not be flagged,
// while a titleless markdown file still is.
func TestCheckNonMarkdownPagesSkipTitleRule(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	colDir := filepath.Join(root, ".stardust", "collections", "specs")
	require.NoError(t, os.MkdirAll(colDir, 0o755))
	colCfg := "path = \"docs/specs\"\ndescription = \"docs specs\"\n\n" +
		"[[fields]]\nname = \"title\"\ntype = \"string\"\nrequired = true\n"
	require.NoError(t, os.WriteFile(filepath.Join(colDir, "config.toml"), []byte(colCfg), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Guide.textile"), []byte("some textile body without any title heading\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "untitled.md"), []byte("a markdown file with no title\n"), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	for _, is := range res.Issues {
		if is.Kind == "missing-title" && is.Path == "Guide.textile" {
			t.Fatalf("non-markdown page flagged missing-title: %+v", is)
		}
	}
	foundMD := false
	for _, is := range res.Issues {
		if is.Kind == "missing-title" && is.Path == "untitled.md" {
			foundMD = true
		}
	}
	require.True(t, foundMD, "the title rule must still fire for markdown files")
}

func TestRelatedEdgeParticipatesInGraph(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/probe\n"), 0o644))
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	// a references an existing plan and a missing spec via related: only.
	write("docs/specs/2026-06-26-0001-a.md", "---\ntitle: A\ntype: spec\nstatus: Draft\ncreated: 2026-06-26\nupdated: 2026-06-26\nrelated: [\"docs/plans/2026-06-26-0001-b.md\", \"docs/specs/missing.md\"]\n---\n# A\n")
	write("docs/plans/2026-06-26-0001-b.md", "---\ntitle: B\ntype: plan\nstatus: Draft\ncreated: 2026-06-26\nupdated: 2026-06-26\n---\n# B\n")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.True(t, hasCheckIssue(check.Issues, "broken-doc-ref")) // missing related still flagged

	gr, err := svc.Graph(context.Background())
	require.NoError(t, err)
	require.NotContains(t, gr.Orphans, "docs/plans/2026-06-26-0001-b.md") // reachable via related:
}

func TestDuplicateNameCrossCollectionScoped(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	// the same slug across specs/ and plans/ must not warn: distinct collection keys.
	write("docs/specs/2026-06-26-0001-game.md", "---\ntitle: Spec\ntype: spec\nstatus: Draft\ncreated: 2026-06-26\nupdated: 2026-06-26\n---\n# Spec\n")
	write("docs/plans/2026-06-26-0001-game.md", "---\ntitle: Plan\ntype: plan\nstatus: Draft\ncreated: 2026-06-26\nupdated: 2026-06-26\n---\n# Plan\n")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.False(t, hasCheckIssue(res.Issues, "duplicate-name"))

	// a true in-collection duplicate (same slug, two subdirs of specs/) still warns.
	write("docs/specs/archive/2026-06-26-0001-game.md", "---\ntitle: Spec2\ntype: spec\nstatus: Draft\ncreated: 2026-06-26\nupdated: 2026-06-26\n---\n# Spec2\n")
	res, err = svc.Check(context.Background())
	require.NoError(t, err)
	require.True(t, hasCheckIssue(res.Issues, "duplicate-name"))
}

// TestCheckBareDuplicateNameWarnsOnlyWhenReferenced pins the referenced-
// ambiguity rule: free-form notes sharing a basename (multiple README.md) are
// ordinary repo shape and stay silent until a wikilink actually targets the
// shared name, at which point the ambiguity is real and warns.
func TestCheckBareDuplicateNameWarnsOnlyWhenReferenced(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write("README.md", "# Root\n\nsee [[guide]]\n")
	write("plugin/README.md", "# Plugin readme\n")
	write("guide.md", "# Guide\n\nsee [[readme-like-nothing]]\n")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.False(t, hasCheckIssue(res.Issues, "duplicate-name"), "unreferenced basename twins must stay silent")

	// The moment a wikilink targets the shared name, the ambiguity is real.
	write("guide.md", "# Guide\n\nsee [[readme]]\n")
	res, err = svc.Check(context.Background())
	require.NoError(t, err)
	require.True(t, hasCheckIssue(res.Issues, "duplicate-name"), "a referenced shared name is genuinely ambiguous")
}

func TestCheckSuppressesConfiguredDirectoryIndexDuplicates(t *testing.T) {
	ctx := context.Background()
	root := directoryIndexVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	_, err = svc.SyncDirectoryIndexes(ctx)
	require.NoError(t, err)

	res, err := svc.Check(ctx)
	require.NoError(t, err)
	require.False(t, hasCheckIssue(res.Issues, "broken-link"))
	require.False(t, hasCheckIssue(res.Issues, "duplicate-name"))
	require.False(t, hasCheckIssuePath(res.Issues, "orphan", "20-Profile/portfolio.md"))
	require.False(t, hasCheckIssuePath(res.Issues, "orphan", "20-Profile/proposals/2026-06-28-sample.md"))
}

func TestCheckReportsDirectoryIndexDrift(t *testing.T) {
	ctx := context.Background()
	root := directoryIndexVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.Check(ctx)
	require.NoError(t, err)
	require.True(t, hasCheckIssue(res.Issues, "directory-index-missing"))
}

func hasCheckIssue(issues []service.Issue, kind string) bool {
	for _, issue := range issues {
		if issue.Kind == kind {
			return true
		}
	}
	return false
}

func hasCheckIssuePath(issues []service.Issue, kind, path string) bool {
	for _, issue := range issues {
		if issue.Kind == kind && issue.Path == path {
			return true
		}
	}
	return false
}
