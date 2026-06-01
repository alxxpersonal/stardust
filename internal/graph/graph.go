// Package graph derives the vault's link graph from [[wikilinks]] (and could
// extend to frontmatter relations). It is a rebuildable cache, never a database:
// regex over markdown, written to cache/graph.json, used for neighbor expansion
// and orphan / broken-link reporting.
package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/alxxpersonal/stardust/internal/vault"
)

// Node is one note in the graph, keyed elsewhere by its normalized name.
type Node struct {
	Path  string   `json:"path"`
	Title string   `json:"title"`
	Out   []string `json:"out"` // normalized names this note links to
	In    []string `json:"in"`  // normalized names that link to this note
}

// Graph is the derived link graph keyed by normalized note name.
type Graph struct {
	Nodes map[string]Node `json:"nodes"`
}

// BrokenLink is a wikilink whose target resolves to no note.
type BrokenLink struct {
	From   string `json:"from"`   // source note path
	Target string `json:"target"` // unresolved normalized target
}

// Build scans the vault and derives the link graph from wikilinks.
func Build(root string, ignore []string) (*Graph, error) {
	paths, err := vault.Scan(root, ignore)
	if err != nil {
		return nil, err
	}
	g := &Graph{Nodes: make(map[string]Node, len(paths))}

	for _, rel := range paths {
		note, err := vault.Parse(root, rel)
		if err != nil {
			return nil, err
		}
		key := vault.NormalizeLink(rel)
		g.Nodes[key] = Node{Path: note.Path, Title: note.Title, Out: note.Links}
	}

	for name, node := range g.Nodes {
		for _, target := range node.Out {
			if t, ok := g.Nodes[target]; ok {
				t.In = append(t.In, name)
				g.Nodes[target] = t
			}
		}
	}
	return g, nil
}

// Orphans returns the paths of notes with no incoming and no outgoing links,
// sorted.
func (g *Graph) Orphans() []string {
	var out []string
	for _, node := range g.Nodes {
		if len(node.In) == 0 && len(node.Out) == 0 {
			out = append(out, node.Path)
		}
	}
	sort.Strings(out)
	return out
}

// BrokenLinks returns every wikilink that points at a non-existent note, sorted
// by source path.
func (g *Graph) BrokenLinks() []BrokenLink {
	var out []BrokenLink
	for _, node := range g.Nodes {
		for _, target := range node.Out {
			if _, ok := g.Nodes[target]; !ok {
				out = append(out, BrokenLink{From: node.Path, Target: target})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].Target < out[j].Target
	})
	return out
}

// Neighbors returns the note names within hops of name (excluding name itself),
// following both out and in edges. It is the retrieval-time expansion primitive.
func (g *Graph) Neighbors(name string, hops int) []string {
	start := vault.NormalizeLink(name)
	if _, ok := g.Nodes[start]; !ok || hops <= 0 {
		return nil
	}
	seen := map[string]bool{start: true}
	frontier := []string{start}
	for h := 0; h < hops; h++ {
		var next []string
		for _, n := range frontier {
			node := g.Nodes[n]
			for _, adj := range append(append([]string{}, node.Out...), node.In...) {
				if _, ok := g.Nodes[adj]; ok && !seen[adj] {
					seen[adj] = true
					next = append(next, adj)
				}
			}
		}
		frontier = next
	}
	var out []string
	for n := range seen {
		if n != start {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// EdgeCount returns the total number of outgoing links across all notes.
func (g *Graph) EdgeCount() int {
	n := 0
	for _, node := range g.Nodes {
		n += len(node.Out)
	}
	return n
}

// Save writes the graph to path as JSON, creating parent dirs as needed.
func (g *Graph) Save(path string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create graph dir: %w", err)
		}
	}
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write graph %s: %w", path, err)
	}
	return nil
}

// Load reads a graph JSON from path.
func Load(path string) (*Graph, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read graph %s: %w", path, err)
	}
	var g Graph
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("parse graph %s: %w", path, err)
	}
	return &g, nil
}
