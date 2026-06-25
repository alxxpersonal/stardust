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

// The hook bodies are the guarded command lines stardust runs for each event.
// Owned mode writes them under a #!/bin/sh shebang into .stardust/hooks; compose
// mode injects the same lines (no shebang) into the existing chain's hook file via
// a sentinel block. Keeping the body separate from the shebang lets both modes
// share one source of truth without drifting.

const postCommitBody = `# stardust: index changed notes after each commit, non-blocking, never fail the commit
command -v stardust >/dev/null 2>&1 && stardust index --since HEAD~1 --background >/dev/null 2>&1 || true
# stardust: regenerate the grouped docs registry, non-blocking, never fail the commit
command -v stardust >/dev/null 2>&1 && stardust registry >/dev/null 2>&1 || true
`

const postMergeBody = `# stardust: re-index after pulling or rewriting history, non-blocking
command -v stardust >/dev/null 2>&1 && stardust index --background >/dev/null 2>&1 || true
`

const preCommitWarnBody = `# stardust: warn on vault issues, never blocks the commit
command -v stardust >/dev/null 2>&1 && stardust check >&2 || true
`

const preCommitStrictBody = `# stardust: block the commit if the vault has errors (broken links, bad frontmatter)
command -v stardust >/dev/null 2>&1 || exit 0
stardust check --strict >&2
`

// The full owned-mode scripts are the bodies under a #!/bin/sh shebang. They stay
// byte-identical to the original standalone hooks.
const (
	postCommit      = shebang + postCommitBody
	postMerge       = shebang + postMergeBody
	preCommitWarn   = shebang + preCommitWarnBody
	preCommitStrict = shebang + preCommitStrictBody
)

// Install wires stardust's index hooks and the optional pre-commit check gate
// (mode "off" | "warn" | "strict"). It detects an existing hook chain first:
//
//   - owned mode (no manager, no existing hooks): write the scripts into hooksDir
//     and point git's core.hooksPath at it, as before.
//   - compose mode (husky, a custom core.hooksPath, or existing .git/hooks): inject
//     a sentinel block into the existing chain's hook files and leave
//     core.hooksPath untouched.
//
// Both modes are idempotent.
func Install(ctx context.Context, root, hooksDir, check string) error {
	mode, targetDir, err := detect(root)
	if err != nil {
		return fmt.Errorf("detect hook chain: %w", err)
	}
	if mode == modeCompose {
		return composeInstall(targetDir, check)
	}
	return ownedInstall(ctx, root, hooksDir, check)
}

// ownedInstall writes stardust's standalone hook scripts into hooksDir and points
// git's core.hooksPath at it. This is the original, clobber-free behavior for repos
// with no existing hook chain.
func ownedInstall(ctx context.Context, root, hooksDir, check string) error {
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

// composeInstall injects stardust's guarded lines into the existing chain's hook
// files under targetDir, wrapped in the sentinel block so re-runs replace rather
// than duplicate. It never touches core.hooksPath: the existing manager keeps
// ownership of the chain. The pre-commit gate is composed when check is "warn" or
// "strict", and stripped when "off".
func composeInstall(targetDir, check string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	blocks := map[string]string{
		"post-commit": postCommitBody,
		"post-merge":  postMergeBody,
	}
	for name, body := range blocks {
		if err := injectBlock(filepath.Join(targetDir, name), body); err != nil {
			return err
		}
	}

	preCommit := filepath.Join(targetDir, "pre-commit")
	switch check {
	case "warn":
		if err := injectBlock(preCommit, preCommitWarnBody); err != nil {
			return err
		}
	case "strict":
		if err := injectBlock(preCommit, preCommitStrictBody); err != nil {
			return err
		}
	default: // off
		if err := stripBlock(preCommit); err != nil {
			return err
		}
	}
	return nil
}

// Uninstall removes only stardust's contribution to the repo's hooks. It detects
// how stardust was installed and acts surgically:
//
//   - compose mode (husky, a custom core.hooksPath, or existing .git/hooks): strip
//     the sentinel block from each target hook file, leaving the user's lines and
//     core.hooksPath untouched.
//   - owned mode (stardust set core.hooksPath to .stardust/hooks): unset
//     core.hooksPath, restoring git's default .git/hooks.
//
// It never unsets a core.hooksPath value stardust did not write. Unsetting an
// already-absent key is best-effort and not an error.
func Uninstall(ctx context.Context, root string) error {
	mode, targetDir, err := detect(root)
	if err != nil {
		return fmt.Errorf("detect hook chain: %w", err)
	}
	if mode == modeCompose {
		for _, name := range []string{"post-commit", "post-merge", "pre-commit"} {
			if err := stripBlock(filepath.Join(targetDir, name)); err != nil {
				return err
			}
		}
		return nil
	}
	_ = exec.CommandContext(ctx, "git", "-C", root, "config", "--unset", "core.hooksPath").Run()
	return nil
}
