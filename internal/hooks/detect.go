package hooks

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Install modes. owned means stardust writes .stardust/hooks and sets
// core.hooksPath; compose means stardust appends a sentinel block to an existing
// hook chain and leaves core.hooksPath untouched.
const (
	modeOwned   = "owned"
	modeCompose = "compose"
)

// ownedHooksRel is the path stardust owns, relative to the repo root. It matches
// config.Layout.Hooks() and is the value stardust writes into core.hooksPath in
// owned mode.
const ownedHooksRel = ".stardust/hooks"

// composeEvents are the git hook files detect probes when core.hooksPath is
// unset: an existing, non-empty one means another chain already owns .git/hooks.
var composeEvents = []string{"post-commit", "post-merge"}

// detect resolves how stardust should install hooks in root. It returns the
// install mode (owned or compose) and the absolute target directory the hooks go
// into:
//
//   - core.hooksPath == .stardust/hooks: owned (idempotent re-run).
//   - core.hooksPath set to anything else (husky .husky, custom): compose into
//     that dir.
//   - core.hooksPath unset but an existing, non-empty .git/hooks/post-commit or
//     post-merge: compose into .git/hooks.
//   - core.hooksPath unset, no existing hooks: owned (current behavior).
func detect(root string) (mode, targetDir string, err error) {
	hooksPath, err := gitHooksPath(root)
	if err != nil {
		return "", "", err
	}

	if hooksPath != "" {
		if filepath.Clean(hooksPath) == filepath.Clean(ownedHooksRel) {
			return modeOwned, filepath.Join(root, ownedHooksRel), nil
		}
		return modeCompose, resolvePath(root, hooksPath), nil
	}

	gitHooks := filepath.Join(root, ".git", "hooks")
	for _, event := range composeEvents {
		if nonEmptyFile(filepath.Join(gitHooks, event)) {
			return modeCompose, gitHooks, nil
		}
	}

	return modeOwned, filepath.Join(root, ownedHooksRel), nil
}

// gitHooksPath returns the configured core.hooksPath for root, or an empty
// string when it is unset. An unset key makes git exit non-zero, which is not an
// error here.
func gitHooksPath(root string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", root, "config", "--get", "core.hooksPath")
	out, err := cmd.Output()
	if err != nil {
		// `git config --get` exits 1 when the key is unset. Treat that as unset,
		// not a failure.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// resolvePath turns a core.hooksPath value into an absolute directory. Relative
// values resolve against root; absolute values pass through.
func resolvePath(root, hooksPath string) string {
	if filepath.IsAbs(hooksPath) {
		return filepath.Clean(hooksPath)
	}
	return filepath.Join(root, hooksPath)
}

// nonEmptyFile reports whether path exists, is a regular file, and has content.
func nonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Size() > 0
}
