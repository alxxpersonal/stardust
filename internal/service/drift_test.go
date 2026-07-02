package service_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

// TestDriftDocsListsReferenceBoundDrift asserts the service reports a doc whose
// related: and inline-path code bindings moved since the doc's last commit,
// ungated by status.
func TestDriftDocsListsReferenceBoundDrift(t *testing.T) {
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

	res, err := svc.DriftDocs(ctx)
	require.NoError(t, err)
	require.Len(t, res.Docs, 1)
	d := res.Docs[0]
	require.Equal(t, "Daemon ADR", d.Title)
	require.Equal(t, "adr", d.Type)
	require.Len(t, d.Bindings, 1)
	require.Equal(t, "internal/store/daemon.go", d.Bindings[0].File)
	require.Greater(t, d.Bindings[0].ChangedCommits, 0)
	require.Contains(t, res.Markdown, "internal/store/daemon.go")
}

// TestDriftDocsEmptyWhenUnmoved asserts no drift when the referenced code has not
// moved since the doc.
func TestDriftDocsEmptyWhenUnmoved(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	writeGovernedCode(t, root, "internal/store/daemon.go")
	writeReferencingDoc(t, root, "docs/adr/0001-daemon.md", "Daemon ADR", "adr", "Proposed", "internal/store/daemon.go")
	gitInit(t, root)

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.DriftDocs(ctx)
	require.NoError(t, err)
	require.Empty(t, res.Docs)
	require.Contains(t, res.Markdown, "No drifted docs")
}

func TestDriftDocsIncludesWikiGovernsFrontmatter(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	writeGovernedCode(t, root, "internal/wiki.go")
	writeWikiGovernedDoc(t, root, "Home.md", "Wiki Home", "internal/wiki.go")
	gitInit(t, root)
	gitRemoteAdd(t, root, "https://github.com/acme/project.wiki.git")
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "wiki.go"), []byte("package internal\n\nconst X = 1\n"), 0o644))
	gitCommitAll(t, root, "edit wiki code")

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.DriftDocs(ctx)
	require.NoError(t, err)
	require.Len(t, res.Docs, 1)
	require.Equal(t, "Home.md", res.Docs[0].DocPath)
	require.Equal(t, "Wiki Home", res.Docs[0].Title)
	require.Equal(t, "wiki", res.Docs[0].Type)
	require.Len(t, res.Docs[0].Bindings, 1)
	require.Equal(t, "internal/wiki.go", res.Docs[0].Bindings[0].File)
}

func TestDriftDocsUsesSourceRootForWikiGoverns(t *testing.T) {
	ctx := context.Background()
	wikiRoot := emptyVault(t)
	sourceRoot := gitSourceRepo(t)
	writeSourceFile(t, sourceRoot, "backend/src/auth.py", "def login():\n    return True\n")
	gitCommitAt(t, sourceRoot, "2026-01-01T10:00:00", "add auth")

	cfg := config.Default()
	cfg.SourceRoot = sourceRoot
	require.NoError(t, config.Save(config.Layout{Root: wikiRoot}.Config(), cfg))
	writeWikiGovernedDoc(t, wikiRoot, "Home.md", "Auth Wiki", "backend/src/auth.py")
	gitInitNoCommit(t, wikiRoot)
	gitRemoteAdd(t, wikiRoot, "https://github.com/acme/project.wiki.git")
	gitCommitAt(t, wikiRoot, "2026-01-02T10:00:00", "add wiki")

	writeSourceFile(t, sourceRoot, "backend/src/auth.py", "def login():\n    return False\n")
	gitCommitAt(t, sourceRoot, "2026-01-03T10:00:00", "edit auth")

	svc, err := service.Open(ctx, wikiRoot)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.DriftDocs(ctx)
	require.NoError(t, err)
	require.Len(t, res.Docs, 1)
	require.Equal(t, "Home.md", res.Docs[0].DocPath)
	require.Len(t, res.Docs[0].Bindings, 1)
	require.Equal(t, "backend/src/auth.py", res.Docs[0].Bindings[0].File)
	require.Equal(t, "source_repo", res.Docs[0].Bindings[0].Source)
	require.Greater(t, res.Docs[0].Bindings[0].ChangedCommits, 0)
	require.Contains(t, res.Markdown, "source repo")

	check, err := svc.Check(ctx)
	require.NoError(t, err)
	require.True(t, hasCheckIssue(check.Issues, "drift"))
	var driftDetail string
	for _, issue := range check.Issues {
		if issue.Kind == "drift" {
			driftDetail = issue.Detail
			break
		}
	}
	require.Contains(t, driftDetail, "backend/src/auth.py")
	require.Contains(t, driftDetail, "source repo")
}

func TestDriftDocsSourceRootCleanWhenSourceUnmoved(t *testing.T) {
	ctx := context.Background()
	wikiRoot := emptyVault(t)
	sourceRoot := gitSourceRepo(t)
	writeSourceFile(t, sourceRoot, "backend/src/auth.py", "def login():\n    return True\n")
	gitCommitAt(t, sourceRoot, "2026-01-01T10:00:00", "add auth")

	cfg := config.Default()
	cfg.SourceRoot = sourceRoot
	require.NoError(t, config.Save(config.Layout{Root: wikiRoot}.Config(), cfg))
	writeWikiGovernedDoc(t, wikiRoot, "Home.md", "Auth Wiki", "backend/src/auth.py")
	gitInitNoCommit(t, wikiRoot)
	gitRemoteAdd(t, wikiRoot, "https://github.com/acme/project.wiki.git")
	gitCommitAt(t, wikiRoot, "2026-01-02T10:00:00", "add wiki")

	svc, err := service.Open(ctx, wikiRoot)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.DriftDocs(ctx)
	require.NoError(t, err)
	require.Empty(t, res.Docs)

	check, err := svc.Check(ctx)
	require.NoError(t, err)
	require.False(t, hasCheckIssue(check.Issues, "drift"))
}

func TestDriftDocsEmptySourceRootKeepsSameRepoResolution(t *testing.T) {
	ctx := context.Background()
	wikiRoot := emptyVault(t)
	writeWikiGovernedDoc(t, wikiRoot, "Home.md", "Auth Wiki", "backend/src/auth.py")
	gitInitNoCommit(t, wikiRoot)
	gitRemoteAdd(t, wikiRoot, "https://github.com/acme/project.wiki.git")
	gitCommitAt(t, wikiRoot, "2026-01-02T10:00:00", "add wiki")

	svc, err := service.Open(ctx, wikiRoot)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.DriftDocs(ctx)
	require.NoError(t, err)
	require.Empty(t, res.Docs)

	check, err := svc.Check(ctx)
	require.NoError(t, err)
	require.False(t, hasCheckIssue(check.Issues, "drift"))
}

// TestDriftDocsDetectsSiblingSourceRoot asserts a <name>.wiki workspace with no
// source_root binds cross-repo drift against the sibling ../<name> checkout when
// their git remotes name the same repository, identical to the explicit
// source_root result.
func TestDriftDocsDetectsSiblingSourceRoot(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	wikiRoot := filepath.Join(parent, "project.wiki")
	wikiStardustVault(t, wikiRoot)
	sourceRoot := filepath.Join(parent, "project")
	require.NoError(t, os.MkdirAll(sourceRoot, 0o755))
	gitInitNoCommit(t, sourceRoot)
	gitRemoteAdd(t, sourceRoot, "https://github.com/acme/project.git")
	writeSourceFile(t, sourceRoot, "backend/src/auth.py", "def login():\n    return True\n")
	gitCommitAt(t, sourceRoot, "2026-01-01T10:00:00", "add auth")

	writeWikiGovernedDoc(t, wikiRoot, "Home.md", "Auth Wiki", "backend/src/auth.py")
	gitInitNoCommit(t, wikiRoot)
	gitRemoteAdd(t, wikiRoot, "https://github.com/acme/project.wiki.git")
	gitCommitAt(t, wikiRoot, "2026-01-02T10:00:00", "add wiki")

	writeSourceFile(t, sourceRoot, "backend/src/auth.py", "def login():\n    return False\n")
	gitCommitAt(t, sourceRoot, "2026-01-03T10:00:00", "edit auth")

	svc, err := service.Open(ctx, wikiRoot)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.DriftDocs(ctx)
	require.NoError(t, err)
	require.Len(t, res.Docs, 1)
	require.Equal(t, "Home.md", res.Docs[0].DocPath)
	require.Len(t, res.Docs[0].Bindings, 1)
	require.Equal(t, "backend/src/auth.py", res.Docs[0].Bindings[0].File)
	require.Equal(t, "source_repo", res.Docs[0].Bindings[0].Source)
	require.Greater(t, res.Docs[0].Bindings[0].ChangedCommits, 0)

	check, err := svc.Check(ctx)
	require.NoError(t, err)
	require.True(t, hasCheckIssue(check.Issues, "drift"))
}

// TestDriftDocsIgnoresUnrelatedSiblingRemote asserts a same-named sibling that is
// a different repository binds nothing: the remote-URL identity guard rejects it,
// so an unmatched governs path resolves to no source root and manufactures no
// drift.
func TestDriftDocsIgnoresUnrelatedSiblingRemote(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	wikiRoot := filepath.Join(parent, "project.wiki")
	wikiStardustVault(t, wikiRoot)
	sibling := filepath.Join(parent, "project")
	require.NoError(t, os.MkdirAll(sibling, 0o755))
	gitInitNoCommit(t, sibling)
	gitRemoteAdd(t, sibling, "https://github.com/acme/other.git")
	writeSourceFile(t, sibling, "backend/src/auth.py", "def login():\n    return True\n")
	gitCommitAt(t, sibling, "2026-01-01T10:00:00", "add auth")

	writeWikiGovernedDoc(t, wikiRoot, "Home.md", "Auth Wiki", "backend/src/auth.py")
	gitInitNoCommit(t, wikiRoot)
	gitRemoteAdd(t, wikiRoot, "https://github.com/acme/project.wiki.git")
	gitCommitAt(t, wikiRoot, "2026-01-02T10:00:00", "add wiki")

	writeSourceFile(t, sibling, "backend/src/auth.py", "def login():\n    return False\n")
	gitCommitAt(t, sibling, "2026-01-03T10:00:00", "edit auth")

	svc, err := service.Open(ctx, wikiRoot)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.DriftDocs(ctx)
	require.NoError(t, err)
	require.Empty(t, res.Docs)

	check, err := svc.Check(ctx)
	require.NoError(t, err)
	require.False(t, hasCheckIssue(check.Issues, "drift"))
}

// wikiStardustVault initializes an empty Stardust vault at dir so a specific
// <name>.wiki basename can be exercised (emptyVault uses a random temp name).
func wikiStardustVault(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: dir}.Config(), config.Default()))
}

// TestCheckSurfacesDriftAsWarning asserts drift reaches the check surface as a
// warn, never as an error, so it does not fail a --strict gate by itself.
func TestCheckSurfacesDriftAsWarning(t *testing.T) {
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

	res, err := svc.Check(ctx)
	require.NoError(t, err)
	require.True(t, hasCheckIssue(res.Issues, "drift"))
	for _, is := range res.Issues {
		if is.Kind == "drift" {
			require.Equal(t, "warn", is.Severity)
			require.Contains(t, is.Detail, "internal/store/daemon.go")
			require.Contains(t, is.Detail, "review")
		}
	}
}

// writeReferencingDoc writes a doc that binds to a code file through both a
// related: frontmatter entry and an inline code-path span in its body.
func writeReferencingDoc(t *testing.T, root, rel, title, typ, status, codePath string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	body := "---\n" +
		"title: " + title + "\n" +
		"type: " + typ + "\n" +
		"status: " + status + "\n" +
		"created: 2026-06-26\n" +
		"updated: 2026-06-26\n" +
		"related: [\"" + codePath + "\"]\n" +
		"---\n# " + title + "\n\nThe daemon store lives in `" + codePath + "` and manages the store daemon.\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func writeWikiGovernedDoc(t *testing.T, root, rel, title, governs string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	body := "---\n" +
		"title: " + title + "\n" +
		"governs: [\"" + governs + "\"]\n" +
		"---\n# " + title + "\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func gitRemoteAdd(t *testing.T, root, remote string) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "remote", "add", "origin", remote)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git remote add: %s", string(out))
}

func gitSourceRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitInitNoCommit(t, root)
	return root
}

func gitInitNoCommit(t *testing.T, root string) {
	t.Helper()
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "t@t")
	gitRun(t, root, "config", "user.name", "t")
	gitRun(t, root, "config", "commit.gpgsign", "false")
}

func writeSourceFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
