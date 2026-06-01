package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// statusLoadedMsg carries index stats back to the status tab.
type statusLoadedMsg struct {
	notes  int
	chunks int
	sha    string
	model  string
}

// statusTab shows index health: counts, last indexed commit, embedding state.
type statusTab struct {
	be     *backend
	loaded bool
	notes  int
	chunks int
	sha    string
	model  string
}

func newStatusTab(be *backend) statusTab { return statusTab{be: be} }

// Init loads the stats.
func (t statusTab) Init() tea.Cmd { return t.load() }

func (t statusTab) load() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		ctx := context.Background()
		notes, chunks, _ := be.store.Count(ctx)
		sha, _ := be.store.GetMeta(ctx, "last_indexed_sha")
		model, _ := be.store.GetMeta(ctx, "embed_model")
		return statusLoadedMsg{notes: notes, chunks: chunks, sha: sha, model: model}
	}
}

// Update applies loaded stats and handles reload.
func (t statusTab) Update(msg tea.Msg) (statusTab, tea.Cmd) {
	switch msg := msg.(type) {
	case statusLoadedMsg:
		t.loaded = true
		t.notes = msg.notes
		t.chunks = msg.chunks
		t.sha = msg.sha
		t.model = msg.model
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			return t, t.load()
		}
	}
	return t, nil
}

// View renders the status rows.
func (t statusTab) View(width, _ int) string {
	if !t.loaded {
		return mutedStyle.Render("loading...")
	}
	sha := t.sha
	if len(sha) > 12 {
		sha = sha[:12]
	}
	if sha == "" {
		sha = "(not a git repo)"
	}
	vectors := "off (FTS-only)"
	if t.be.hasVec {
		vectors = "on (" + t.model + ")"
	}
	rows := [][2]string{
		{"Vault", t.be.layout.Root},
		{"Notes", fmt.Sprintf("%d", t.notes)},
		{"Chunks", fmt.Sprintf("%d", t.chunks)},
		{"Last indexed", sha},
		{"Embeddings", vectors},
		{"Ollama", t.be.cfg.OllamaURL},
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Index status") + "\n\n")
	for _, row := range rows {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("%-14s", row[0])) + textStyle.Render(row[1]) + "\n")
	}
	b.WriteString("\n" + mutedStyle.Render("press r to refresh"))
	return b.String()
}
