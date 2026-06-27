// Package gitx wraps the git CLI for the two things Stardust needs from git:
// change detection (the index spine) and archival of history.
package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// --- Command runner ---

// run executes git -C repoRoot with args and returns trimmed stdout.
func run(ctx context.Context, repoRoot string, args ...string) (string, error) {
	full := append([]string{"-C", repoRoot}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// --- Queries ---

// IsRepo reports whether repoRoot is inside a git work tree.
func IsRepo(ctx context.Context, repoRoot string) bool {
	_, err := run(ctx, repoRoot, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// HeadSHA returns the current HEAD commit SHA.
func HeadSHA(ctx context.Context, repoRoot string) (string, error) {
	return run(ctx, repoRoot, "rev-parse", "HEAD")
}

// Branch returns the current branch name, or a detached HEAD marker when needed.
func Branch(ctx context.Context, repoRoot string) (string, error) {
	if !IsRepo(ctx, repoRoot) {
		return "", nil
	}
	branch, err := run(ctx, repoRoot, "branch", "--show-current")
	if err == nil && branch != "" {
		return branch, nil
	}
	if !hasHead(ctx, repoRoot) {
		return "", nil
	}
	short, err := run(ctx, repoRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve branch: %w", err)
	}
	return "detached " + short, nil
}

// LastCommit returns the latest commit touching paths, or an empty string when
// repoRoot is not a git repository or no commit matches.
func LastCommit(ctx context.Context, repoRoot string, paths ...string) (string, error) {
	if !IsRepo(ctx, repoRoot) {
		return "", nil
	}
	args := []string{"log", "-1", "--format=%H", "--"}
	args = append(args, paths...)
	sha, err := run(ctx, repoRoot, args...)
	if err != nil {
		return "", fmt.Errorf("last commit: %w", err)
	}
	return sha, nil
}

// FirstCommitDate returns the YYYY-MM-DD date of the commit that first added
// path, following renames. It returns an empty string when path is untracked or
// repoRoot is not a git repository.
func FirstCommitDate(ctx context.Context, repoRoot, path string) (string, error) {
	if !hasHead(ctx, repoRoot) {
		return "", nil
	}
	out, err := run(ctx, repoRoot, "log", "--diff-filter=A", "--follow", "--format=%ad", "--date=short", "--", path)
	if err != nil {
		return "", fmt.Errorf("first commit date %s: %w", path, err)
	}
	return lastLine(out), nil
}

// LastCommitDate returns the YYYY-MM-DD date of the most recent commit touching
// paths. It returns an empty string when paths are untracked or repoRoot is not
// a git repository.
func LastCommitDate(ctx context.Context, repoRoot string, paths ...string) (string, error) {
	if !hasHead(ctx, repoRoot) {
		return "", nil
	}
	args := []string{"log", "-1", "--format=%ad", "--date=short", "--"}
	args = append(args, paths...)
	out, err := run(ctx, repoRoot, args...)
	if err != nil {
		return "", fmt.Errorf("last commit date: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// Move renames a tracked file from oldPath to newPath via git mv, preserving
// history. Both paths are relative to repoRoot.
func Move(ctx context.Context, repoRoot, oldPath, newPath string) error {
	if _, err := run(ctx, repoRoot, "mv", oldPath, newPath); err != nil {
		return fmt.Errorf("git mv %s to %s: %w", oldPath, newPath, err)
	}
	return nil
}

// IsTracked reports whether path is tracked in repoRoot's git index.
func IsTracked(ctx context.Context, repoRoot, path string) bool {
	_, err := run(ctx, repoRoot, "ls-files", "--error-unmatch", "--", path)
	return err == nil
}

// hasHead reports whether repoRoot is a git repository with at least one commit,
// so history queries do not fail on a non-repo or an unborn branch.
func hasHead(ctx context.Context, repoRoot string) bool {
	_, err := run(ctx, repoRoot, "rev-parse", "--verify", "HEAD")
	return err == nil
}

// CommitCountSince counts commits since a commit that touched paths. Non-git
// repos and empty since commits return zero.
func CommitCountSince(ctx context.Context, repoRoot string, since string, paths ...string) (int, error) {
	if since == "" || !IsRepo(ctx, repoRoot) {
		return 0, nil
	}
	args := []string{"rev-list", "--count", since + "..HEAD", "--"}
	args = append(args, paths...)
	raw, err := run(ctx, repoRoot, args...)
	if err != nil {
		return 0, fmt.Errorf("commit count since %s: %w", since, err)
	}
	if raw == "" {
		return 0, nil
	}
	var count int
	if _, err := fmt.Sscanf(raw, "%d", &count); err != nil {
		return 0, fmt.Errorf("parse commit count %q: %w", raw, err)
	}
	return count, nil
}

// Init initialises a git repository at repoRoot (which must already exist).
func Init(ctx context.Context, repoRoot string) error {
	_, err := run(ctx, repoRoot, "init")
	return err
}

// CommitAll stages everything and commits it with message, using the caller's
// configured git identity.
func CommitAll(ctx context.Context, repoRoot, message string) error {
	if _, err := run(ctx, repoRoot, "add", "-A"); err != nil {
		return err
	}
	args := []string{"commit", "-m", message}
	if !hasIdentity(ctx, repoRoot) {
		// fall back to a default identity so the first commit succeeds even when
		// the user has no global git user.name and user.email configured.
		args = append([]string{"-c", "user.name=stardust", "-c", "user.email=stardust@localhost"}, args...)
	}
	_, err := run(ctx, repoRoot, args...)
	return err
}

// hasIdentity reports whether git has a usable commit identity configured.
func hasIdentity(ctx context.Context, repoRoot string) bool {
	name, _ := run(ctx, repoRoot, "config", "user.name")
	email, _ := run(ctx, repoRoot, "config", "user.email")
	return name != "" && email != ""
}

// DiffNames returns the markdown paths changed between sinceSHA and HEAD
// (added, copied, modified, renamed, or deleted). With an empty sinceSHA it
// returns every tracked markdown file, which drives a full index. Callers stat
// each returned path to tell a delete (prune) from a change (reindex).
func DiffNames(ctx context.Context, repoRoot, sinceSHA string) ([]string, error) {
	var raw string
	var err error
	if sinceSHA == "" {
		raw, err = run(ctx, repoRoot, "ls-files", "*.md")
	} else {
		raw, err = run(ctx, repoRoot, "diff", "--name-only", "--diff-filter=ACMRD", sinceSHA, "HEAD", "--", "*.md")
	}
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(raw), nil
}

// --- Archival ---

// Archive creates a timestamped bare mirror of repoRoot's full git history
// under dest and returns the created path.
func Archive(ctx context.Context, repoRoot, dest string) (string, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", fmt.Errorf("create archive dir %s: %w", dest, err)
	}
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}
	name := fmt.Sprintf("%s-%s.git", filepath.Base(abs), time.Now().Format("20060102-150405"))
	target := filepath.Join(dest, name)
	cmd := exec.CommandContext(ctx, "git", "clone", "--mirror", abs, target)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone --mirror: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return target, nil
}

// --- Helpers ---

// lastLine returns the trimmed final non-empty line of s, or an empty string.
func lastLine(s string) string {
	lines := nonEmptyLines(s)
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func nonEmptyLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			out = append(out, t)
		}
	}
	return out
}
