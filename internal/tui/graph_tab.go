package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/alxxpersonal/stardust/internal/graph"
)

// graphLoadedMsg carries the derived graph back to the graph tab.
type graphLoadedMsg struct {
	g       *graph.Graph
	orphans []string
	broken  []graph.BrokenLink
	err     error
}

// graphTab reports the link graph: counts, orphans, broken links.
type graphTab struct {
	be      *backend
	loaded  bool
	err     error
	nodes   int
	edges   int
	orphans []string
	broken  []graph.BrokenLink
}

func newGraphTab(be *backend) graphTab { return graphTab{be: be} }

// Init derives the graph.
func (t graphTab) Init() tea.Cmd { return t.load() }

func (t graphTab) load() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		g, err := graph.Build(be.layout.Root, be.cfg.Ignore)
		if err != nil {
			return graphLoadedMsg{err: err}
		}
		return graphLoadedMsg{g: g, orphans: g.Orphans(), broken: g.BrokenLinks()}
	}
}

// Update applies the derived graph and handles reload.
func (t graphTab) Update(msg tea.Msg) (graphTab, tea.Cmd) {
	switch msg := msg.(type) {
	case graphLoadedMsg:
		t.loaded = true
		t.err = msg.err
		if msg.g != nil {
			t.nodes = len(msg.g.Nodes)
			t.edges = msg.g.EdgeCount()
			t.orphans = msg.orphans
			t.broken = msg.broken
		}
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			return t, t.load()
		}
	}
	return t, nil
}

// View renders the graph report.
func (t graphTab) View(_, height int) string {
	if t.err != nil {
		return mutedStyle.Render("graph error: " + t.err.Error())
	}
	if !t.loaded {
		return mutedStyle.Render("deriving graph...")
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Link graph") + "\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("%d notes, %d links", t.nodes, t.edges)) + "\n\n")

	limit := (height - 8) / 2
	if limit < 3 {
		limit = 3
	}

	b.WriteString(accentStyle.Render(fmt.Sprintf("Orphans (%d)", len(t.orphans))) + "\n")
	if len(t.orphans) == 0 {
		b.WriteString(mutedStyle.Render("  none") + "\n")
	}
	for i, p := range t.orphans {
		if i >= limit {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more", len(t.orphans)-limit)) + "\n")
			break
		}
		b.WriteString(textStyle.Render("  "+p) + "\n")
	}

	b.WriteString("\n" + accentStyle.Render(fmt.Sprintf("Broken links (%d)", len(t.broken))) + "\n")
	if len(t.broken) == 0 {
		b.WriteString(mutedStyle.Render("  none") + "\n")
	}
	for i, bl := range t.broken {
		if i >= limit {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more", len(t.broken)-limit)) + "\n")
			break
		}
		b.WriteString(textStyle.Render(fmt.Sprintf("  %s -> [[%s]]", bl.From, bl.Target)) + "\n")
	}

	b.WriteString("\n" + mutedStyle.Render("press r to rebuild"))
	return b.String()
}
