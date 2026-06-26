package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLastCommitAndCommitCountSince(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "t@t")
	runGit(t, root, "config", "user.name", "t")
	runGit(t, root, "config", "commit.gpgsign", "false")

	path := filepath.Join(root, "internal", "foo.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("package internal\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "init")

	first, err := LastCommit(ctx, root, "internal/foo.go")
	if err != nil {
		t.Fatalf("LastCommit() error = %v", err)
	}
	if first == "" {
		t.Fatal("LastCommit() = empty, want sha")
	}

	if err := os.WriteFile(path, []byte("package internal\n\nconst X = 1\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "change foo")

	count, err := CommitCountSince(ctx, root, first, "internal/foo.go")
	if err != nil {
		t.Fatalf("CommitCountSince() error = %v", err)
	}
	if got, want := count, 1; got != want {
		t.Fatalf("CommitCountSince() = %d, want %d", got, want)
	}
}

func TestLastCommitNonGitRepoReturnsEmpty(t *testing.T) {
	sha, err := LastCommit(t.Context(), t.TempDir(), "x.go")
	if err != nil {
		t.Fatalf("LastCommit() error = %v", err)
	}
	if sha != "" {
		t.Fatalf("LastCommit() = %q, want empty", sha)
	}
}

func TestFirstAndLastCommitDate(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "t@t")
	runGit(t, root, "config", "user.name", "t")
	runGit(t, root, "config", "commit.gpgsign", "false")

	rel := "docs/specs/x.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("# x\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, root, "add", "-A")
	commitAt(t, root, "2026-01-15T10:00:00", "add x")

	if err := os.WriteFile(path, []byte("# x\n\nmore\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}
	runGit(t, root, "add", "-A")
	commitAt(t, root, "2026-03-20T12:00:00", "update x")

	first, err := FirstCommitDate(ctx, root, rel)
	if err != nil {
		t.Fatalf("FirstCommitDate() error = %v", err)
	}
	if got, want := first, "2026-01-15"; got != want {
		t.Fatalf("FirstCommitDate() = %q, want %q", got, want)
	}

	last, err := LastCommitDate(ctx, root, rel)
	if err != nil {
		t.Fatalf("LastCommitDate() error = %v", err)
	}
	if got, want := last, "2026-03-20"; got != want {
		t.Fatalf("LastCommitDate() = %q, want %q", got, want)
	}
}

func TestCommitDatesUntrackedReturnEmpty(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "t@t")
	runGit(t, root, "config", "user.name", "t")
	runGit(t, root, "config", "commit.gpgsign", "false")

	// Give the repo a HEAD so the untracked query runs against real history.
	if err := os.WriteFile(filepath.Join(root, "seed.md"), []byte("# seed\n"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	runGit(t, root, "add", "-A")
	commitAt(t, root, "2026-01-01T00:00:00", "seed")

	first, err := FirstCommitDate(ctx, root, "docs/specs/missing.md")
	if err != nil {
		t.Fatalf("FirstCommitDate() error = %v", err)
	}
	if first != "" {
		t.Fatalf("FirstCommitDate() = %q, want empty", first)
	}

	last, err := LastCommitDate(ctx, root, "docs/specs/missing.md")
	if err != nil {
		t.Fatalf("LastCommitDate() error = %v", err)
	}
	if last != "" {
		t.Fatalf("LastCommitDate() = %q, want empty", last)
	}

	// A non-git directory is also empty, no error.
	nonRepo := t.TempDir()
	d, err := FirstCommitDate(ctx, nonRepo, "x.md")
	if err != nil || d != "" {
		t.Fatalf("FirstCommitDate(non-repo) = %q, %v; want empty, nil", d, err)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(out))
	}
}

// commitAt commits with a fixed author and committer date so date-derived
// assertions are deterministic.
func commitAt(t *testing.T, root, date, msg string) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "commit", "-m", msg)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit: %v: %s", err, string(out))
	}
}
