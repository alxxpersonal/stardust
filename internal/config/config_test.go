package config

import (
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
