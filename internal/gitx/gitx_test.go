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

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(out))
	}
}
