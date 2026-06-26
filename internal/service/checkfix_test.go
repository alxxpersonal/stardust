package service_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/vault"
)

func fixVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
	return root
}

// gitInitVault wraps fixVault with an initialized git repository so checkfix can
// derive dates and rename through git.
func gitInitVault(t *testing.T) string {
	t.Helper()
	root := fixVault(t)
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "t@t")
	gitRun(t, root, "config", "user.name", "t")
	gitRun(t, root, "config", "commit.gpgsign", "false")
	return root
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(out))
	}
}

func gitCommitAt(t *testing.T, root, date, msg string) {
	t.Helper()
	gitRun(t, root, "add", "-A")
	cmd := exec.Command("git", "-C", root, "commit", "-m", msg)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit: %v: %s", err, string(out))
	}
}

func gitStatus(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v: %s", err, string(out))
	}
	return string(out)
}

func TestCheckFixGitDerivedDates(t *testing.T) {
	root := gitInitVault(t)
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-dates.md")
	// Missing created and updated so the fixer fills them, valid otherwise.
	body := "---\ntitle: Dates\ntype: spec\nstatus: Draft\n---\n# Dates\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))
	gitCommitAt(t, root, "2026-01-15T10:00:00", "add dates")
	// A later content commit moves the last-touched date.
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body+"\nmore\n"), 0o644))
	gitCommitAt(t, root, "2026-03-20T12:00:00", "edit dates")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	note, err := vault.Parse(root, filepath.ToSlash(rel))
	require.NoError(t, err)
	require.Equal(t, "2026-01-15", note.Frontmatter["created"], "created is the first commit date")
	require.Equal(t, "2026-03-20", note.Frontmatter["updated"], "updated is the last commit date")
}

func TestCheckFixUntrackedDatesFallBackToMtime(t *testing.T) {
	root := gitInitVault(t)
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-untracked.md")
	// In a git repo but never committed, so git has no date for it.
	body := "---\ntitle: Untracked\ntype: spec\nstatus: Draft\n---\n# Untracked\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	note, err := vault.Parse(root, filepath.ToSlash(rel))
	require.NoError(t, err)
	today := time.Now().UTC().Format("2006-01-02")
	require.Equal(t, today, note.Frontmatter["created"], "untracked created falls back to mtime")
	require.Equal(t, today, note.Frontmatter["updated"], "untracked updated falls back to mtime")
}

func TestCheckFixReplacesForbiddenDashes(t *testing.T) {
	root := fixVault(t)
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-dashy.md")
	body := "---\ntitle: Dashy\ntype: spec\nstatus: Draft\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Dashy\n\none \u2014 two \u2013 three \u2014 four\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	fixed, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err)
	require.NotContains(t, string(fixed), "\u2014")
	require.NotContains(t, string(fixed), "\u2013")
	require.Contains(t, string(fixed), "one - two - three - four")

	// A re-check should no longer report a forbidden-dash error.
	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.False(t, hasCheckIssue(check.Issues, "forbidden-dash"))
}

func TestCheckFixRewritesBadDocType(t *testing.T) {
	root := fixVault(t)
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-wrong-type.md")
	body := "---\ntitle: Wrong\ntype: plan\nstatus: Draft\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Wrong\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	note, err := vault.Parse(root, filepath.ToSlash(rel))
	require.NoError(t, err)
	require.Equal(t, "spec", note.Frontmatter["type"])

	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.False(t, hasCheckIssue(check.Issues, "bad-doc-type"))
}

func TestCheckFixFillsMissingFields(t *testing.T) {
	root := fixVault(t)
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-missing.md")
	// Missing type, created, updated. Title and status present so they are not the gap.
	body := "---\ntitle: Missing\nstatus: Draft\n---\n# Missing\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	note, err := vault.Parse(root, filepath.ToSlash(rel))
	require.NoError(t, err)
	require.Equal(t, "spec", note.Frontmatter["type"])
	require.NotEmpty(t, note.Frontmatter["created"])
	require.NotEmpty(t, note.Frontmatter["updated"])

	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	for _, is := range check.Issues {
		if is.Kind == "missing-doc-field" && is.Path == filepath.ToSlash(rel) {
			require.Contains(t, []string{"title", "status"}, strings.Fields(is.Detail)[0])
		}
	}
}

func TestCheckFixRenamesOffConventionFiles(t *testing.T) {
	root := gitInitVault(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "adr"), 0o755))

	// Off-convention spec with no status, so status proves report-only.
	specRel := filepath.Join("docs", "specs", "badspec.md")
	specBody := "---\ntitle: Spec One\ntype: spec\ncreated: 2026-02-10\nupdated: 2026-02-10\n---\n# Spec One\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, specRel), []byte(specBody), 0o644))

	// Off-convention adr, otherwise valid.
	adrRel := filepath.Join("docs", "adr", "badadr.md")
	adrBody := "---\ntitle: Some Decision\ntype: adr\nstatus: Proposed\ncreated: 2026-02-10\nupdated: 2026-02-10\n---\n# Some Decision\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, adrRel), []byte(adrBody), 0o644))
	gitCommitAt(t, root, "2026-02-10T09:00:00", "add off-convention docs")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	// The spec is renamed to <first-commit-date>-<time>-<slug>.md.
	newSpec := "docs/specs/2026-02-10-0000-spec-one.md"
	require.FileExists(t, filepath.Join(root, filepath.FromSlash(newSpec)))
	require.NoFileExists(t, filepath.Join(root, filepath.FromSlash(specRel)))

	// The adr is renamed to <next-number>-<slug>.md.
	newADR := "docs/adr/0001-some-decision.md"
	require.FileExists(t, filepath.Join(root, filepath.FromSlash(newADR)))
	require.NoFileExists(t, filepath.Join(root, filepath.FromSlash(adrRel)))

	// git mv preserved history: both renames are staged as renames, not delete+add.
	status := gitStatus(t, root)
	require.Contains(t, status, "R  docs/specs/badspec.md -> "+newSpec)
	require.Contains(t, status, "R  docs/adr/badadr.md -> "+newADR)

	// title and status are report-only: the spec keeps its title and is still
	// missing status after the fix.
	note, err := vault.Parse(root, newSpec)
	require.NoError(t, err)
	require.Equal(t, "Spec One", note.Frontmatter["title"])
	require.NotContains(t, note.Frontmatter, "status")

	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.False(t, hasCheckIssue(check.Issues, "bad-doc-name"))
	require.True(t, hasCheckIssue(check.Issues, "missing-doc-field"))
}

func TestCheckFixCodemodRemediatesEverythingButTitleStatus(t *testing.T) {
	root := gitInitVault(t)
	// One file with: forbidden dash, missing type, missing dates, off-convention
	// name, and a missing status. Title present so the rename slug derives.
	rel := filepath.Join("docs", "specs", "rawnote.md")
	body := "---\ntitle: Raw Note\n---\n# Raw Note\n\none \\u2014 two\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))
	gitCommitAt(t, root, "2026-02-10T09:00:00", "add raw note")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	newRel := "docs/specs/2026-02-10-0000-raw-note.md"
	require.FileExists(t, filepath.Join(root, filepath.FromSlash(newRel)))
	require.NoFileExists(t, filepath.Join(root, filepath.FromSlash(rel)))

	note, err := vault.Parse(root, newRel)
	require.NoError(t, err)
	require.Equal(t, "spec", note.Frontmatter["type"])
	require.Equal(t, "2026-02-10", note.Frontmatter["created"])
	require.Equal(t, "2026-02-10", note.Frontmatter["updated"])

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(newRel)))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "\u2014")

	// Re-running check: dash, type, dates, and name are all clean; only the
	// report-only missing status remains.
	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.False(t, hasCheckIssue(check.Issues, "forbidden-dash"))
	require.False(t, hasCheckIssue(check.Issues, "bad-doc-name"))
	require.False(t, hasCheckIssue(check.Issues, "bad-doc-type"))
	for _, is := range check.Issues {
		if is.Kind == "missing-doc-field" {
			require.Equal(t, "status", strings.Fields(is.Detail)[0])
		}
	}
}

func TestCheckFixLeavesAmbiguousIssuesAlone(t *testing.T) {
	root := fixVault(t)
	// bad-doc-status is a judgment call: must not be touched.
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-weird-status.md")
	body := "---\ntitle: Weird\ntype: spec\nstatus: Weird\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Weird\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, res.Fixed)

	note, err := vault.Parse(root, filepath.ToSlash(rel))
	require.NoError(t, err)
	require.Equal(t, "Weird", note.Frontmatter["status"])

	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.True(t, hasCheckIssue(check.Issues, "bad-doc-status"))
}
