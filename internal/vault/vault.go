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
)

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
