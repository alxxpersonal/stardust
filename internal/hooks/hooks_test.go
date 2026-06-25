package hooks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostCommitRegeneratesRegistry(t *testing.T) {
	indexIdx := strings.Index(postCommit, "stardust index")
	require.GreaterOrEqual(t, indexIdx, 0, "post-commit hook must still index changed notes")

	registryIdx := strings.Index(postCommit, "stardust registry")
	require.GreaterOrEqual(t, registryIdx, 0, "post-commit hook must regenerate the docs registry")
	require.Greater(t, registryIdx, indexIdx, "registry line must come after the index line")

	// The registry line stays best-effort so it never fails the commit.
	registryLine := postCommit[registryIdx:]
	if nl := strings.IndexByte(registryLine, '\n'); nl >= 0 {
		registryLine = registryLine[:nl]
	}
	require.Contains(t, registryLine, "|| true", "registry line must not fail the commit")
}

// configValue returns the configured git value for key in root, or "" when unset.
func configValue(t *testing.T, root, key string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "config", "--get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestInstallOwnedModeWritesStardustHooks(t *testing.T) {
	root := newRepo(t)
	hooksDir := filepath.Join(root, ".stardust", "hooks")

	if err := Install(context.Background(), root, hooksDir, "off"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Owned mode writes the .stardust/hooks scripts byte-for-byte as today.
	for name, want := range map[string]string{
		"post-commit":  postCommit,
		"post-merge":   postMerge,
		"post-rewrite": postMerge,
	} {
		if got := readFile(t, filepath.Join(hooksDir, name)); got != want {
			t.Fatalf("owned %s = %q, want %q", name, got, want)
		}
	}

	// Owned mode points core.hooksPath at .stardust/hooks.
	if got := configValue(t, root, "core.hooksPath"); got != ".stardust/hooks" {
		t.Fatalf("core.hooksPath = %q, want .stardust/hooks", got)
	}
}

func TestInstallOwnedModeWritesPreCommitGate(t *testing.T) {
	root := newRepo(t)
	hooksDir := filepath.Join(root, ".stardust", "hooks")

	if err := Install(context.Background(), root, hooksDir, "strict"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if got := readFile(t, filepath.Join(hooksDir, "pre-commit")); got != preCommitStrict {
		t.Fatalf("owned pre-commit = %q, want %q", got, preCommitStrict)
	}
}

func TestInstallComposeModeAppendsBlockAndKeepsHooksPath(t *testing.T) {
	root := newRepo(t)
	// A husky-style chain already owns core.hooksPath.
	runGit(t, root, "config", "core.hooksPath", ".husky")
	huskyDir := filepath.Join(root, ".husky")
	if err := os.MkdirAll(huskyDir, 0o755); err != nil {
		t.Fatalf("create husky dir: %v", err)
	}
	userHook := "#!/bin/sh\nnpx lint-staged\n"
	if err := os.WriteFile(filepath.Join(huskyDir, "post-commit"), []byte(userHook), 0o755); err != nil {
		t.Fatalf("write user post-commit: %v", err)
	}

	hooksDir := filepath.Join(root, ".stardust", "hooks")
	if err := Install(context.Background(), root, hooksDir, "off"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Compose mode must not touch core.hooksPath.
	if got := configValue(t, root, "core.hooksPath"); got != ".husky" {
		t.Fatalf("core.hooksPath = %q, want .husky (untouched)", got)
	}

	// The user's post-commit keeps its line and gains exactly one stardust block.
	postCommitBody := readFile(t, filepath.Join(huskyDir, "post-commit"))
	if !strings.Contains(postCommitBody, "npx lint-staged") {
		t.Fatalf("post-commit = %q, want the user line preserved", postCommitBody)
	}
	if starts, ends := countMarkers(postCommitBody); starts != 1 || ends != 1 {
		t.Fatalf("post-commit markers = (%d start, %d end), want exactly one of each", starts, ends)
	}
	for _, want := range []string{"stardust index", "stardust registry"} {
		if !strings.Contains(postCommitBody, want) {
			t.Fatalf("post-commit = %q, want the %q line composed in", postCommitBody, want)
		}
	}

	// post-merge gets its own composed block too.
	postMergeBody := readFile(t, filepath.Join(huskyDir, "post-merge"))
	if starts, ends := countMarkers(postMergeBody); starts != 1 || ends != 1 {
		t.Fatalf("post-merge markers = (%d start, %d end), want exactly one of each", starts, ends)
	}
	if !strings.Contains(postMergeBody, "stardust index") {
		t.Fatalf("post-merge = %q, want the index line composed in", postMergeBody)
	}

	// Compose mode must NOT create the owned .stardust/hooks scripts.
	if _, err := os.Stat(filepath.Join(hooksDir, "post-commit")); !os.IsNotExist(err) {
		t.Fatalf("compose mode wrote .stardust/hooks/post-commit, want it left alone")
	}
}

func TestInstallComposeModeIsIdempotent(t *testing.T) {
	root := newRepo(t)
	runGit(t, root, "config", "core.hooksPath", ".husky")
	huskyDir := filepath.Join(root, ".husky")
	if err := os.MkdirAll(huskyDir, 0o755); err != nil {
		t.Fatalf("create husky dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(huskyDir, "post-commit"), []byte("#!/bin/sh\nnpx lint-staged\n"), 0o755); err != nil {
		t.Fatalf("write user post-commit: %v", err)
	}

	hooksDir := filepath.Join(root, ".stardust", "hooks")
	if err := Install(context.Background(), root, hooksDir, "off"); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}
	first := readFile(t, filepath.Join(huskyDir, "post-commit"))
	if err := Install(context.Background(), root, hooksDir, "off"); err != nil {
		t.Fatalf("second Install() error = %v", err)
	}
	second := readFile(t, filepath.Join(huskyDir, "post-commit"))

	if first != second {
		t.Fatalf("second compose install changed the file:\nfirst:\n%q\nsecond:\n%q", first, second)
	}
	if starts, ends := countMarkers(second); starts != 1 || ends != 1 {
		t.Fatalf("after two installs markers = (%d start, %d end), want exactly one of each", starts, ends)
	}
}

func TestInstallComposeModeWritesPreCommitGate(t *testing.T) {
	root := newRepo(t)
	runGit(t, root, "config", "core.hooksPath", ".husky")
	huskyDir := filepath.Join(root, ".husky")
	if err := os.MkdirAll(huskyDir, 0o755); err != nil {
		t.Fatalf("create husky dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(huskyDir, "post-commit"), []byte("#!/bin/sh\nnpx lint-staged\n"), 0o755); err != nil {
		t.Fatalf("write user post-commit: %v", err)
	}

	hooksDir := filepath.Join(root, ".stardust", "hooks")
	if err := Install(context.Background(), root, hooksDir, "warn"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	preCommitBody := readFile(t, filepath.Join(huskyDir, "pre-commit"))
	if starts, ends := countMarkers(preCommitBody); starts != 1 || ends != 1 {
		t.Fatalf("pre-commit markers = (%d start, %d end), want exactly one of each", starts, ends)
	}
	if !strings.Contains(preCommitBody, "stardust check") {
		t.Fatalf("pre-commit = %q, want the check line composed in", preCommitBody)
	}
}
