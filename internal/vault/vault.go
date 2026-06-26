// Package vault reads the markdown source of truth: scanning the tree,
// parsing frontmatter, extracting wikilinks, content hashing, and header-aware
// chunking. It never writes to the vault; files stay the source of truth.
package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Note is a parsed markdown file.
type Note struct {
	Path        string         // path relative to the vault root, slash-separated
	Title       string         // frontmatter title, else first H1, else file name
	Tags        []string       // frontmatter tags plus inline #hashtags
	Links       []string       // normalized wikilink targets
	Frontmatter map[string]any // raw parsed frontmatter
	Body        string         // markdown body after the frontmatter block
	Hash        string         // sha256 of the raw file bytes
}

var (
	frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n?`)
	wikilinkRe    = regexp.MustCompile(`\[\[([^\]|#]+)(?:[#|][^\]]*)?\]\]`)
	hashtagRe     = regexp.MustCompile(`(?m)(?:^|\s)#([a-zA-Z][\w/-]+)`)
	h1Re          = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	inlineCodeRe  = regexp.MustCompile("`([^`\n]+)`")
	repoPathRe    = regexp.MustCompile(`^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+$`)
)

// Edge kinds enumerate the channels through which a note references another.
const (
	EdgeWikilink   = "wikilink"
	EdgeRelated    = "related"
	EdgeInlinePath = "inline-path"
)

// Edge is one typed outbound reference from a note. Target is a normalized note
// name for a wikilink, or a repo-relative path for a related or inline-path
// reference. Kind is one of wikilink, related, or inline-path.
type Edge struct {
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

// --- Hashing + links ---

// ContentHash returns the hex sha256 of b.
func ContentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ExtractLinks returns the unique normalized wikilink targets in body.
func ExtractLinks(body string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range wikilinkRe.FindAllStringSubmatch(body, -1) {
		key := NormalizeLink(m[1])
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

// ExtractEdges returns the typed outbound references in a note: wikilinks from
// the body, related targets from frontmatter, and inline repo-path references
// from backtick code spans. Wikilink targets are normalized; related targets are
// kept verbatim for the graph to resolve and classify; an inline-path candidate
// is emitted only when it matches a dir/dir/file.ext shape and resolves to an
// existing file under root, which suppresses false positives. Edges are unique
// by (kind, target).
func ExtractEdges(root string, note Note) []Edge {
	var out []Edge
	seen := map[string]bool{}
	add := func(target, kind string) {
		if target == "" {
			return
		}
		key := kind + "\x00" + target
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Edge{Target: target, Kind: kind})
	}

	for _, m := range wikilinkRe.FindAllStringSubmatch(note.Body, -1) {
		add(normalizeWikilinkTarget(m[1]), EdgeWikilink)
	}
	for _, target := range fmStringList(note.Frontmatter, "related") {
		add(strings.TrimSpace(target), EdgeRelated)
	}
	for _, m := range inlineCodeRe.FindAllStringSubmatch(note.Body, -1) {
		token := strings.TrimSpace(m[1])
		if !repoPathRe.MatchString(token) || filepath.Ext(token) == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(token))); err != nil || info.IsDir() {
			continue
		}
		add(token, EdgeInlinePath)
	}
	return out
}

// CodeRefs returns the unique repo-relative paths a note binds to code through a
// related: frontmatter entry or an inline code-path span: targets that resolve to
// an existing, non-directory, non-markdown file under root. It is the doc-to-code
// binding set drift detection watches; wikilinks and markdown targets are
// excluded, since those are doc-to-doc graph edges. Paths are slash-separated.
func CodeRefs(root string, note Note) []string {
	var out []string
	seen := map[string]bool{}
	for _, e := range ExtractEdges(root, note) {
		if e.Kind != EdgeRelated && e.Kind != EdgeInlinePath {
			continue
		}
		target := filepath.ToSlash(e.Target)
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(target)))
		if err != nil || info.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(target), ".md") {
			continue
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

// NormalizeLink reduces a wikilink target or file path to its lowercased base
// name without extension, so links and note keys resolve identically.
func NormalizeLink(t string) string {
	t = strings.TrimSpace(t)
	t = strings.TrimSuffix(t, ".md")
	if i := strings.LastIndexAny(t, `/\`); i >= 0 {
		t = t[i+1:]
	}
	return strings.ToLower(strings.TrimSpace(t))
}

// --- Collection-scoped keys ---

// docCollectionFolders maps a singular doc type to its collection folder so a
// prefix-style wikilink qualifier (spec:slug) resolves to the same key as the
// path-style folder qualifier (specs/slug).
var docCollectionFolders = map[string]string{
	"spec":     "specs",
	"plan":     "plans",
	"adr":      "adr",
	"research": "research",
}

// CollectionKey returns the collection-qualified node key for a vault-relative
// path: "<collection>/<base>" for a doc under docs/<collection>/, else the bare
// normalized basename. Cross-collection slugs (a spec and its same-slug plan)
// therefore key to distinct nodes, while non-docs notes keep their legacy
// basename key for backward compatibility.
func CollectionKey(rel string) string {
	base := NormalizeLink(rel)
	if coll := collectionFolderOf(rel); coll != "" {
		return coll + "/" + base
	}
	return base
}

// collectionFolderOf returns the lowercased collection folder for a path under
// docs/<collection>/..., or an empty string when the path is not a collection doc.
func collectionFolderOf(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) >= 3 && parts[0] == "docs" {
		return strings.ToLower(parts[1])
	}
	return ""
}

// normalizeWikilinkTarget normalizes a wikilink target while preserving a
// collection qualifier: a prefix-style spec:slug and a path-style specs/slug both
// yield "specs/slug". Any other target falls back to NormalizeLink's basename
// behavior, so unqualified links and ordinary path links are unchanged.
func normalizeWikilinkTarget(t string) string {
	t = strings.TrimSpace(t)
	// prefix-style: <type>:<slug> with no path separator.
	if i := strings.Index(t, ":"); i > 0 && !strings.ContainsAny(t, `/\`) {
		typ := strings.ToLower(strings.TrimSpace(t[:i]))
		if folder, ok := docCollectionFolders[typ]; ok {
			return folder + "/" + NormalizeLink(t[i+1:])
		}
	}
	// path-style: <collection>/<slug> where the first segment is a known folder.
	if i := strings.IndexAny(t, `/\`); i > 0 {
		first := strings.ToLower(strings.TrimSpace(t[:i]))
		if isCollectionFolder(first) {
			return first + "/" + NormalizeLink(t[i+1:])
		}
	}
	return NormalizeLink(t)
}

// isCollectionFolder reports whether seg is a known docs collection folder, used
// to tell a path-style qualifier (specs/slug) from an ordinary path link.
func isCollectionFolder(seg string) bool {
	for _, folder := range docCollectionFolders {
		if seg == folder {
			return true
		}
	}
	return false
}

// --- Parsing ---

// Parse reads the markdown file at root/rel and returns a parsed Note.
func Parse(root, rel string) (Note, error) {
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return Note{}, fmt.Errorf("read note %s: %w", rel, err)
	}
	n := Note{Path: filepath.ToSlash(rel), Hash: ContentHash(raw)}
	body := string(raw)
	if m := frontmatterRe.FindStringSubmatch(body); m != nil {
		fm := map[string]any{}
		if err := yaml.Unmarshal([]byte(m[1]), &fm); err == nil {
			n.Frontmatter = fm
			n.Title = fmString(fm, "title")
			n.Tags = fmTags(fm)
		}
		body = body[len(m[0]):]
	}
	n.Body = body
	if n.Title == "" {
		if m := h1Re.FindStringSubmatch(body); m != nil {
			n.Title = strings.TrimSpace(m[1])
		}
	}
	if n.Title == "" {
		n.Title = strings.TrimSuffix(filepath.Base(rel), ".md")
	}
	n.Tags = dedupe(append(n.Tags, inlineTags(body)...))
	n.Links = ExtractLinks(body)
	return n, nil
}

// --- Scanning ---

// Scan walks root and returns the slash-separated relative paths of every
// markdown file, skipping any directory whose name is in ignore.
func Scan(root string, ignore []string) ([]string, error) {
	ignored := map[string]bool{}
	for _, ig := range ignore {
		ignored[ig] = true
	}
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && ignored[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".md") {
			rel, relErr := filepath.Rel(root, p)
			if relErr != nil {
				return relErr
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan vault: %w", err)
	}
	return out, nil
}

// --- Frontmatter helpers ---

func fmString(fm map[string]any, key string) string {
	if v, ok := fm[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// fmStringList reads a frontmatter field as a list of strings, accepting a YAML
// list (the common form for related) or a single scalar. It lives here, rather
// than reusing convention.StringList, because convention imports vault and the
// reverse import would cycle.
func fmStringList(fm map[string]any, key string) []string {
	raw, ok := fm[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		return []string{strings.TrimSpace(v)}
	default:
		return nil
	}
}

func fmTags(fm map[string]any) []string {
	switch v := fm["tags"].(type) {
	case string:
		return splitTags(v)
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

func splitTags(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
	var out []string
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func inlineTags(body string) []string {
	var out []string
	for _, m := range hashtagRe.FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
