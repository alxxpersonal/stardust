package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/embed"
	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// newTestApp builds an App over a tiny FTS-only index in a temp vault.
func newTestApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	writeNote(t, root, "a.md", "---\ntitle: Alpha\ntags: [x]\n---\n# Alpha\nGoroutines and channels. See [[b]].")
	writeNote(t, root, "b.md", "---\ntitle: Beta\n---\n# Beta\nError wrapping in go.")

	layout := config.Layout{Root: root}
	cfg := config.Default()
	store, err := index.Open(context.Background(), layout.DB())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, rel := range []string{"a.md", "b.md"} {
		note, err := vault.Parse(root, rel)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		if err := store.UpsertNote(context.Background(), note.Path, note.Hash, vault.Chunks(note), nil, note.Frontmatter); err != nil {
			t.Fatalf("upsert %s: %v", rel, err)
		}
	}

	be := &backend{
		layout: layout,
		cfg:    cfg,
		store:  store,
		embed:  embed.New(cfg.OllamaURL, cfg.EmbedModel),
		hasVec: false,
	}
	return newApp(be)
}

func writeNote(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
}

func finalApp(t *testing.T, tm *teatest.TestModel) *App {
	t.Helper()
	_ = tm.Quit()
	m := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second))
	app, ok := m.(*App)
	if !ok {
		t.Fatalf("final model is not *App: %T", m)
	}
	return app
}

func TestApp_TabCycling(t *testing.T) {
	tm := teatest.NewTestModel(t, newTestApp(t), teatest.WithInitialTermSize(100, 40))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab}) // search -> status
	time.Sleep(30 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab}) // status -> graph
	time.Sleep(30 * time.Millisecond)
	if final := finalApp(t, tm); final.active != tabGraph {
		t.Errorf("expected active=%d (graph), got %d", tabGraph, final.active)
	}
}

func TestApp_DigitsTypeIntoSearch(t *testing.T) {
	tm := teatest.NewTestModel(t, newTestApp(t), teatest.WithInitialTermSize(100, 40))
	// on the search tab, "2" is query text, not a tab switch
	tm.Send(tea.KeyPressMsg{Code: '2'})
	time.Sleep(30 * time.Millisecond)
	if final := finalApp(t, tm); final.active != tabSearch {
		t.Errorf("digit switched tabs from search, active=%d", final.active)
	}
}

func TestApp_CtrlCQuits(t *testing.T) {
	tm := teatest.NewTestModel(t, newTestApp(t), teatest.WithInitialTermSize(100, 40))
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestApp_BannerRenders(t *testing.T) {
	if !strings.Contains(newTestApp(t).renderBanner(), "STARDUST") {
		t.Error("banner missing wordmark")
	}
}
