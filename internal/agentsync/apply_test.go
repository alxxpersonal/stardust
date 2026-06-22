package agentsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyCreatesRelativeSymlink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "foo")
	target := filepath.Join(root, "target", "foo")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}

	plan := Plan{Actions: []Action{{
		Kind:     KindSkill,
		ItemName: "foo",
		Source:   source,
		Target:   target,
		Mode:     "symlink",
		Status:   "create",
	}}}
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	link, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("read symlink: %v", err)
	}
	if filepath.IsAbs(link) {
		t.Fatalf("symlink target = %q, want relative", link)
	}
	if !sameTarget(target, link, source) {
		t.Fatalf("symlink target = %q, want %q", link, source)
	}
}

func TestApplyCopiesDirectories(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "foo")
	target := filepath.Join(root, "target", "foo")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("# Foo\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	plan := Plan{Actions: []Action{{
		Kind:     KindSkill,
		ItemName: "foo",
		Source:   source,
		Target:   target,
		Mode:     "copy",
		Status:   "create",
	}}}
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(data) != "# Foo\n" {
		t.Fatalf("copied file = %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(target, managedMarkerName)); err != nil {
		t.Fatalf("managed marker missing: %v", err)
	}
}

func TestApplyRefusesUnmanagedConflict(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "foo")
	target := filepath.Join(root, "target", "foo")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("create target: %v", err)
	}

	plan := Plan{Repair: true, Actions: []Action{{
		Kind:     KindSkill,
		ItemName: "foo",
		Source:   source,
		Target:   target,
		Mode:     "symlink",
		Status:   "conflict",
	}}}
	if _, err := Apply(plan); err == nil {
		t.Fatal("Apply() error = nil, want unmanaged conflict error")
	}
}

func TestApplyRepairsManagedConflict(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "foo")
	target := filepath.Join(root, "target", "foo")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("create target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, managedMarkerName), []byte("stardust\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	plan := Plan{Repair: true, Actions: []Action{{
		Kind:     KindSkill,
		ItemName: "foo",
		Source:   source,
		Target:   target,
		Mode:     "symlink",
		Status:   "conflict",
	}}}
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target was not repaired to symlink: info=%v err=%v", info, err)
	}
}
