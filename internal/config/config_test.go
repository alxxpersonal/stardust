package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSourceRootEmpty(t *testing.T) {
	got, err := Default().ResolveSourceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveSourceRoot() error = %v", err)
	}
	if got != "" {
		t.Fatalf("ResolveSourceRoot() = %q, want empty", got)
	}
}

func TestResolveSourceRootRelativeToVault(t *testing.T) {
	root := t.TempDir()
	cfg := Default()
	cfg.SourceRoot = "../source"

	got, err := cfg.ResolveSourceRoot(root)
	if err != nil {
		t.Fatalf("ResolveSourceRoot() error = %v", err)
	}
	want, err := filepath.Abs(filepath.Join(root, "..", "source"))
	if err != nil {
		t.Fatalf("abs source root: %v", err)
	}
	if got != want {
		t.Fatalf("ResolveSourceRoot() = %q, want %q", got, want)
	}
}

func TestResolveSourceRootAbsolute(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "source")
	cfg := Default()
	cfg.SourceRoot = abs

	got, err := cfg.ResolveSourceRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveSourceRoot() error = %v", err)
	}
	if got != abs {
		t.Fatalf("ResolveSourceRoot() = %q, want %q", got, abs)
	}
}

func TestLoadDirectoryIndexesConfig(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")
	body := `embed_model = ""
ignore = [".git"]

[conventions.directory_indexes]
enabled = true
roots = ["20-Profile", "90-Archive"]
ignore = [".claude", "10-Code/Worktrees"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	dir := cfg.Conventions.DirectoryIndexes.WithDefaults()
	if !dir.Enabled {
		t.Fatalf("directory indexes should be enabled")
	}
	if dir.Filename != "INDEX.md" {
		t.Fatalf("Filename = %q, want INDEX.md", dir.Filename)
	}
	if dir.Mode != "managed-block" {
		t.Fatalf("Mode = %q, want managed-block", dir.Mode)
	}
	if len(dir.Roots) != 2 || dir.Roots[0] != "20-Profile" || dir.Roots[1] != "90-Archive" {
		t.Fatalf("Roots = %#v", dir.Roots)
	}
	if len(dir.Ignore) != 2 || dir.Ignore[1] != "10-Code/Worktrees" {
		t.Fatalf("Ignore = %#v", dir.Ignore)
	}
}
