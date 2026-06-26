// Package graph derives the vault's link graph from [[wikilinks]] and from
// related: frontmatter references, classifying each target into a doc-to-doc
// edge or a doc-to-code reference by on-disk resolution. It is a rebuildable
// cache, never a database: regex over markdown plus a stat per reference,
// written to cache/graph.json, used for neighbor expansion and orphan /
// broken-link reporting.
package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alxxpersonal/stardust/internal/vault"
)

// Edge is a typed graph reference (re-exported from vault): a doc-to-doc out
// edge or a doc-to-code reference, each carrying its Kind.
type Edge = vault.Edge

// Node is one note in the graph, keyed elsewhere by its normalized name. Out
// holds the doc-to-doc edges (wikilink and related) it resolves to; CodeRefs
// holds doc-to-code references (related or inline-path targets that resolve to a
// non-markdown repo file), captured for drift binding but never graph nodes.
type Node struct {
	Path     string   `json:"path"`
	Title    string   `json:"title"`
	Out      []Edge   `json:"out"` // doc edges this note links to (normalized target + kind)
	In       []string `json:"in"`  // normalized names that link to this note
	CodeRefs []Edge   `json:"code_refs,omitempty"`
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

// Build scans the vault and derives the link graph from wikilink and related
// edges, capturing doc-to-code references separately for drift binding.
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
		key := vault.CollectionKey(rel)
		out, codeRefs := classifyEdges(root, vault.ExtractEdges(root, note))
		g.Nodes[key] = Node{Path: note.Path, Title: note.Title, Out: out, CodeRefs: codeRefs}
	}

	// resolve each out edge to an actual node key now that every node is known:
	// in-collection first, then a node keyed exactly by the target, then a unique
	// global basename match. Unqualified links keep resolving while collection-
	// qualified ones disambiguate cross-collection slug reuse.
	idx := newResolveIndex(g)
	for key, node := range g.Nodes {
		coll := collectionOfKey(key)
		resolved := make([]Edge, 0, len(node.Out))
		for _, e := range node.Out {
			target, _ := idx.resolve(coll, e.Target)
			resolved = append(resolved, Edge{Target: target, Kind: e.Kind})
		}
		node.Out = resolved
		g.Nodes[key] = node
	}

	for name, node := range g.Nodes {
		for _, e := range node.Out {
			if t, ok := g.Nodes[e.Target]; ok {
				t.In = append(t.In, name)
				g.Nodes[e.Target] = t
			}
		}
	}
	return g, nil
}

// resolveIndex resolves a link target to a node key. It holds the set of node
// keys and an index from bare basename to the keys that carry it.
type resolveIndex struct {
	keys   map[string]bool
	byBase map[string][]string
}

// newResolveIndex builds the resolution index over a graph's node keys.
func newResolveIndex(g *Graph) resolveIndex {
	idx := resolveIndex{keys: make(map[string]bool, len(g.Nodes)), byBase: map[string][]string{}}
	for key := range g.Nodes {
		idx.keys[key] = true
		base := baseOfKey(key)
		idx.byBase[base] = append(idx.byBase[base], key)
	}
	return idx
}

// resolve maps a link target to a node key. A collection-qualified target (one
// carrying a "/" separator) resolves directly to the matching key. An unqualified
// target prefers an in-collection node, then a node keyed exactly by the target,
// then a unique global basename match; an ambiguous or missing target is returned
// unchanged with ok false, so BrokenLinks surfaces it.
func (idx resolveIndex) resolve(sourceColl, target string) (string, bool) {
	if strings.Contains(target, "/") {
		if idx.keys[target] {
			return target, true
		}
		return target, false
	}
	if sourceColl != "" {
		if k := sourceColl + "/" + target; idx.keys[k] {
			return k, true
		}
	}
	if idx.keys[target] {
		return target, true
	}
	if ks := idx.byBase[target]; len(ks) == 1 {
		return ks[0], true
	}
	return target, false
}

// collectionOfKey returns the collection prefix of a node key, or "" when the key
// is a bare basename.
func collectionOfKey(key string) string {
	if i := strings.Index(key, "/"); i >= 0 {
		return key[:i]
	}
	return ""
}

// baseOfKey returns the basename portion of a node key, dropping any collection
// prefix.
func baseOfKey(key string) string {
	if i := strings.Index(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}

// classifyEdges splits a note's extracted edges into doc-to-doc out edges and
// doc-to-code references. Wikilinks are always doc edges, resolved later against
// the node set. A related or inline-path target is resolved on disk: a markdown
// file becomes a doc edge keyed by its normalized name; a non-markdown repo file
// becomes a code reference; an unresolved target is dropped (the convention
// checker reports broken related refs separately). Out edges are unique by target.
func classifyEdges(root string, edges []Edge) (out []Edge, codeRefs []Edge) {
	seen := map[string]bool{}
	addOut := func(target, kind string) {
		if target == "" || seen[target] {
			return
		}
		seen[target] = true
		out = append(out, Edge{Target: target, Kind: kind})
	}
	for _, e := range edges {
		if e.Kind == vault.EdgeWikilink {
			addOut(e.Target, e.Kind)
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(e.Target)))
		if err != nil || info.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Target), ".md") {
			addOut(vault.CollectionKey(e.Target), e.Kind)
			continue
		}
		codeRefs = append(codeRefs, Edge{Target: filepath.ToSlash(e.Target), Kind: e.Kind})
	}
	return out, codeRefs
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
		for _, e := range node.Out {
			if _, ok := g.Nodes[e.Target]; !ok {
				out = append(out, BrokenLink{From: node.Path, Target: e.Target})
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
	start := vault.CollectionKey(name)
	if _, ok := g.Nodes[start]; !ok || hops <= 0 {
		return nil
	}
	seen := map[string]bool{start: true}
	frontier := []string{start}
	for h := 0; h < hops; h++ {
		var next []string
		for _, n := range frontier {
			node := g.Nodes[n]
			adjacent := make([]string, 0, len(node.Out)+len(node.In))
			for _, e := range node.Out {
				adjacent = append(adjacent, e.Target)
			}
			adjacent = append(adjacent, node.In...)
			for _, adj := range adjacent {
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

// PersonalizedPageRank runs personalized PageRank over the undirected link graph
// with the restart distribution concentrated on the seed nodes (paths or names).
// It returns a score per note name; higher means more central to the seeds. This
// is the link-aware expansion used by context bundles (the Aider repomap move).
func (g *Graph) PersonalizedPageRank(seeds []string, iterations int, damping float64) map[string]float64 {
	n := len(g.Nodes)
	if n == 0 {
		return nil
	}
	if iterations <= 0 {
		iterations = 30
	}
	if damping <= 0 || damping >= 1 {
		damping = 0.85
	}

	// undirected adjacency restricted to in-graph nodes
	adj := make(map[string][]string, n)
	for name, node := range g.Nodes {
		seen := map[string]bool{}
		for _, e := range node.Out {
			if _, ok := g.Nodes[e.Target]; ok {
				seen[e.Target] = true
			}
		}
		for _, e := range node.In {
			if _, ok := g.Nodes[e]; ok {
				seen[e] = true
			}
		}
		for nb := range seen {
			adj[name] = append(adj[name], nb)
		}
	}

	// restart vector, concentrated on valid seeds (uniform if none resolve)
	restart := make(map[string]float64, n)
	valid := 0
	for _, s := range seeds {
		k := vault.CollectionKey(s)
		if _, ok := g.Nodes[k]; ok {
			restart[k]++
			valid++
		}
	}
	if valid == 0 {
		for k := range g.Nodes {
			restart[k] = 1.0 / float64(n)
		}
	} else {
		for k := range restart {
			restart[k] /= float64(valid)
		}
	}

	pr := make(map[string]float64, n)
	for k := range g.Nodes {
		pr[k] = restart[k]
	}
	for i := 0; i < iterations; i++ {
		next := make(map[string]float64, n)
		var dangling float64
		for node, p := range pr {
			d := len(adj[node])
			if d == 0 {
				dangling += p
				continue
			}
			contrib := damping * p / float64(d)
			for _, nb := range adj[node] {
				next[nb] += contrib
			}
		}
		for node := range g.Nodes {
			next[node] += (1-damping)*restart[node] + damping*dangling*restart[node]
		}
		pr = next
	}
	return pr
}

// PageRankEntry is one note's global centrality score in the link graph.
type PageRankEntry struct {
	Path  string  `json:"path"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

// TopPageRank runs global (non-personalized) PageRank over the undirected link
// graph and returns the top n notes by centrality, sorted by descending score
// then path for stability. It reuses PersonalizedPageRank with no seeds, which
// falls back to the uniform restart distribution that defines standard PageRank.
// A non-positive n returns all notes.
func (g *Graph) TopPageRank(n int) []PageRankEntry {
	pr := g.PersonalizedPageRank(nil, 0, 0)
	if len(pr) == 0 {
		return nil
	}
	out := make([]PageRankEntry, 0, len(pr))
	for name, score := range pr {
		node := g.Nodes[name]
		out = append(out, PageRankEntry{Path: node.Path, Title: node.Title, Score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Path < out[j].Path
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
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
