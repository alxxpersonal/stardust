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
	_, err := run(ctx, repoRoot, "commit", "-m", message)
	return err
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
