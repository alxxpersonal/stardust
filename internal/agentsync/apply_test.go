package agentsync

import (
	"os"
	"path/filepath"
	"strings"
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

func TestApplyComposesRulesIntoFreshFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".stardust", "rules.md")
	target := filepath.Join(root, "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(source, []byte("---\nname: rules\n---\n# Rules\n\nbe kind\n"), 0o644); err != nil {
		t.Fatalf("write rules source: %v", err)
	}

	plan := Plan{Actions: []Action{{
		Kind:     KindRules,
		ItemName: "rules",
		Tool:     ToolClaude,
		Source:   source,
		Target:   target,
		Mode:     "compose",
		Status:   "create",
	}}}
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read composed target: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, rulesBlockStart) || !strings.Contains(text, rulesBlockEnd) {
		t.Fatalf("composed file missing markers: %q", text)
	}
	if !strings.Contains(text, "be kind") {
		t.Fatalf("composed file missing rendered body: %q", text)
	}
	if strings.Contains(text, "name: rules") {
		t.Fatalf("composed file leaked frontmatter: %q", text)
	}
}

func TestApplySelfHealsComposeDriftWithoutRepair(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".stardust", "rules.md")
	target := filepath.Join(root, "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(source, []byte("# Rules\n\nfresh body\n"), 0o644); err != nil {
		t.Fatalf("write rules source: %v", err)
	}
	user := "# Repo owned\n\nkeep me\n"
	if err := os.WriteFile(target, []byte(user), 0o644); err != nil {
		t.Fatalf("write user target: %v", err)
	}
	if err := injectRulesBlock(target, "stale body"); err != nil {
		t.Fatalf("seed stale block: %v", err)
	}

	// No Repair set: compose drift must self-heal anyway.
	plan := Plan{Actions: []Action{{
		Kind:     KindRules,
		ItemName: "rules",
		Tool:     ToolCodex,
		Source:   source,
		Target:   target,
		Mode:     "compose",
		Status:   "drift",
	}}}
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read healed target: %v", err)
	}
	text := string(body)
	if strings.Contains(text, "stale body") {
		t.Fatalf("healed file kept stale body: %q", text)
	}
	if !strings.Contains(text, "fresh body") {
		t.Fatalf("healed file missing fresh body: %q", text)
	}
	if !strings.Contains(text, "keep me") {
		t.Fatalf("healed file dropped user line: %q", text)
	}
	if starts := strings.Count(text, rulesBlockStart); starts != 1 {
		t.Fatalf("healed file has %d start markers, want 1", starts)
	}
}

func TestApplySymlinkDriftStillNeedsRepair(t *testing.T) {
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
		Status:   "drift",
	}}}
	if _, err := Apply(plan); err == nil {
		t.Fatal("Apply() error = nil, want symlink drift to require --repair")
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
