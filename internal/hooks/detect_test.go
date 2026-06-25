package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(out))
	}
}

// newRepo makes a temp git repo and returns its root.
func newRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "t@t")
	runGit(t, root, "config", "user.name", "t")
	return root
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, root string)
		wantMode   string
		wantTarget string // path relative to root
	}{
		{
			name: "core.hooksPath set to stardust hooks is owned",
			setup: func(t *testing.T, root string) {
				runGit(t, root, "config", "core.hooksPath", ".stardust/hooks")
			},
			wantMode:   "owned",
			wantTarget: ".stardust/hooks",
		},
		{
			name: "core.hooksPath set to husky is compose into that dir",
			setup: func(t *testing.T, root string) {
				runGit(t, root, "config", "core.hooksPath", ".husky")
			},
			wantMode:   "compose",
			wantTarget: ".husky",
		},
		{
			name: "unset with an existing post-commit hook is compose into git hooks",
			setup: func(t *testing.T, root string) {
				writeHook(t, root, "post-commit", "#!/bin/sh\necho hi\n")
			},
			wantMode:   "compose",
			wantTarget: ".git/hooks",
		},
		{
			name: "unset with an existing post-merge hook is compose into git hooks",
			setup: func(t *testing.T, root string) {
				writeHook(t, root, "post-merge", "#!/bin/sh\necho hi\n")
			},
			wantMode:   "compose",
			wantTarget: ".git/hooks",
		},
		{
			name:       "unset with no existing hooks is owned",
			setup:      func(t *testing.T, root string) {},
			wantMode:   "owned",
			wantTarget: ".stardust/hooks",
		},
		{
			name: "unset with an empty post-commit hook is owned",
			setup: func(t *testing.T, root string) {
				writeHook(t, root, "post-commit", "")
			},
			wantMode:   "owned",
			wantTarget: ".stardust/hooks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newRepo(t)
			tt.setup(t, root)

			mode, target, err := detect(root)
			if err != nil {
				t.Fatalf("detect() error = %v", err)
			}
			if mode != tt.wantMode {
				t.Fatalf("detect() mode = %q, want %q", mode, tt.wantMode)
			}
			want := filepath.Join(root, tt.wantTarget)
			if target != want {
				t.Fatalf("detect() target = %q, want %q", target, want)
			}
		})
	}
}

// writeHook drops a hook script into .git/hooks under root.
func writeHook(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatalf("write hook %s: %v", name, err)
	}
}
