// Package hooks wires git commit hooks to keep the index fresh. It versions the
// hooks under .stardust/hooks (via core.hooksPath) so they travel with clones,
// and the hooks themselves are async and never fail a commit.
package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const postCommit = `#!/bin/sh
# stardust: index changed notes after each commit, non-blocking, never fail the commit
command -v stardust >/dev/null 2>&1 && stardust index --since HEAD~1 --background >/dev/null 2>&1 || true
`

const postMerge = `#!/bin/sh
# stardust: re-index after pulling or rewriting history, non-blocking
command -v stardust >/dev/null 2>&1 && stardust index --background >/dev/null 2>&1 || true
`

const preCommitWarn = `#!/bin/sh
# stardust: warn on vault issues, never blocks the commit
command -v stardust >/dev/null 2>&1 && stardust check >&2 || true
`

const preCommitStrict = `#!/bin/sh
# stardust: block the commit if the vault has errors (broken links, bad frontmatter)
command -v stardust >/dev/null 2>&1 || exit 0
stardust check --strict >&2
`

// Install writes the index hooks to hooksDir, sets up the pre-commit check gate
// per the check mode ("off" | "warn" | "strict"), and points git's
// core.hooksPath at hooksDir (relative to root). It is idempotent.
func Install(ctx context.Context, root, hooksDir, check string) error {
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	scripts := map[string]string{
		"post-commit":  postCommit,
		"post-merge":   postMerge,
		"post-rewrite": postMerge,
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(hooksDir, name), []byte(body), 0o755); err != nil {
			return fmt.Errorf("write hook %s: %w", name, err)
		}
	}

	preCommit := filepath.Join(hooksDir, "pre-commit")
	switch check {
	case "warn":
		if err := os.WriteFile(preCommit, []byte(preCommitWarn), 0o755); err != nil {
			return fmt.Errorf("write pre-commit hook: %w", err)
		}
	case "strict":
		if err := os.WriteFile(preCommit, []byte(preCommitStrict), 0o755); err != nil {
			return fmt.Errorf("write pre-commit hook: %w", err)
		}
	default: // off
		_ = os.Remove(preCommit)
	}
	rel, err := filepath.Rel(root, hooksDir)
	if err != nil {
		rel = hooksDir
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "config", "core.hooksPath", rel)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set core.hooksPath: %w: %s", err, string(out))
	}
	return nil
}

// Uninstall unsets core.hooksPath, restoring git's default .git/hooks. It is
// best-effort: unsetting an already-absent key is not an error.
func Uninstall(ctx context.Context, root string) error {
	_ = exec.CommandContext(ctx, "git", "-C", root, "config", "--unset", "core.hooksPath").Run()
	return nil
}
