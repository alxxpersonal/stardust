package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alxxpersonal/stardust/internal/agentsync"
	"github.com/alxxpersonal/stardust/internal/config"
)

func TestSyncDryRunPlansWithoutApplying(t *testing.T) {
	root := t.TempDir()
	requireVaultConfig(t, root)
	source := filepath.Join(root, "skills")
	writeSyncSkill(t, source, "foo")
	writeSyncConfig(t, root, source, filepath.Join(root, ".claude", "skills"))

	svc, err := Open(t.Context(), root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = svc.Close() }()

	res, err := svc.Sync(t.Context(), agentsync.Options{Scope: agentsync.ScopeRepo, Tools: []agentsync.Tool{agentsync.ToolClaude}, DryRun: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if got, want := len(res.Plan.Actions), 1; got != want {
		t.Fatalf("len(Actions) = %d, want %d", got, want)
	}
	if got, want := res.Plan.Actions[0].Status, "create"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "skills", "foo")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created target or unexpected stat error: %v", err)
	}
}

func requireVaultConfig(t *testing.T, root string) {
	t.Helper()
	layout := config.Layout{Root: root}
	if err := os.MkdirAll(layout.Cache(), 0o755); err != nil {
		t.Fatalf("create cache: %v", err)
	}
	if err := config.Save(layout.Config(), config.Default()); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func writeSyncSkill(t *testing.T, source, name string) {
	t.Helper()
	dir := filepath.Join(source, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func writeSyncConfig(t *testing.T, root, source, target string) {
	t.Helper()
	layout := config.Layout{Root: root}
	body := "default_targets = [\"claude\"]\n\n" +
		"[[sources]]\n" +
		"name = \"skills\"\n" +
		"path = \"" + filepath.ToSlash(source) + "\"\n" +
		"kind = \"skill\"\n" +
		"priority = 10\n\n" +
		"[[targets]]\n" +
		"tool = \"claude\"\n" +
		"scope = \"repo\"\n" +
		"skills_path = \"" + filepath.ToSlash(target) + "\"\n" +
		"mode = \"symlink\"\n"
	if err := os.MkdirAll(layout.Dir(), 0o755); err != nil {
		t.Fatalf("create stardust dir: %v", err)
	}
	if err := os.WriteFile(layout.SyncConfig(), []byte(body), 0o644); err != nil {
		t.Fatalf("write sync config: %v", err)
	}
}
