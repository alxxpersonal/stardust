// Package vault reads the markdown source of truth: scanning the tree,
// parsing frontmatter, extracting wikilinks, content hashing, and header-aware
// chunking. It never writes to the vault; files stay the source of truth.
package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
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
	frontmatterRe  = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n?`)
	wikilinkRe     = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	markdownLinkRe = regexp.MustCompile(`!?\[[^\]\n]*\]\(([^)\n]+)\)`)
	hashtagRe      = regexp.MustCompile(`(?m)(?:^|\s)#([a-zA-Z][\w/-]+)`)
	h1Re           = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	repoPathRe     = regexp.MustCompile(`^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+$`)
	repoPathFindRe = regexp.MustCompile(`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+`)
)

// Edge kinds enumerate the channels through which a note references another
// note or a code path.
const (
	EdgeWikilink     = "wikilink"
	EdgeMarkdownLink = "markdown-link"
	EdgeRelated      = "related"
	EdgeInlinePath   = "inline-path"
)

// Edge is one typed outbound reference from a note. Target is a normalized note
// name for a wikilink or markdown link, or a repo-relative path for a related
// or inline-path reference.
type Edge struct {
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

// LinkResolutionCandidates carries the legacy primary target for a prose link
// plus the preferred candidates graph resolution should try for that link.
type LinkResolutionCandidates struct {
	Primary    string
	Candidates []string
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
	for _, candidates := range ExtractWikilinkCandidates(body) {
		if len(candidates) == 0 {
			continue
		}
		key := candidates[0]
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

// ExtractWikilinkCandidates returns normalized target candidates for each
// wikilink in body. Candidate zero preserves the existing Obsidian-style
// target-before-pipe behavior. When a pipe is present, candidate one is the
// Gollum/GitHub target-after-pipe form so graph resolution can fall back to it.
func ExtractWikilinkCandidates(body string) [][]string {
	body = maskMarkdownCode(body)
	var out [][]string
	for _, m := range wikilinkRe.FindAllStringSubmatch(body, -1) {
		candidates := wikilinkCandidates(m[1])
		if len(candidates) > 0 {
			out = append(out, candidates)
		}
	}
	return out
}

// ExtractLinkResolutionCandidates returns source-aware candidates for prose
// links that can point at markdown pages. The Primary field matches the target
// emitted by ExtractEdges; Candidates is ordered for resolution, so path-shaped
// wiki links can resolve before falling back to Stardust's legacy flat basename.
func ExtractLinkResolutionCandidates(sourcePath, body string) []LinkResolutionCandidates {
	visibleBody := maskMarkdownCode(body)
	var out []LinkResolutionCandidates
	out = append(out, extractWikilinkResolutionCandidates(sourcePath, visibleBody)...)
	out = append(out, extractMarkdownLinkResolutionCandidates(sourcePath, visibleBody)...)
	return out
}

// ExtractWikilinkResolutionCandidates returns source-aware candidates for only
// wikilinks, preserving GetNote's wikilink-only link target contract.
func ExtractWikilinkResolutionCandidates(sourcePath, body string) []LinkResolutionCandidates {
	return extractWikilinkResolutionCandidates(sourcePath, maskMarkdownCode(body))
}

func wikilinkCandidates(raw string) []string {
	parts := strings.SplitN(raw, "|", 2)
	candidates := make([]string, 0, len(parts))
	add := func(target string) {
		target = stripWikilinkAnchor(target)
		if isExternalWikilink(target) {
			return
		}
		key := normalizeWikilinkTarget(target)
		if key == "" {
			return
		}
		for _, existing := range candidates {
			if existing == key {
				return
			}
		}
		candidates = append(candidates, key)
	}
	add(parts[0])
	if len(parts) == 2 {
		add(parts[1])
	}
	return candidates
}

func extractWikilinkResolutionCandidates(sourcePath, body string) []LinkResolutionCandidates {
	var out []LinkResolutionCandidates
	for _, m := range wikilinkRe.FindAllStringSubmatch(body, -1) {
		primary := wikilinkCandidates(m[1])
		if len(primary) == 0 {
			continue
		}
		candidates := wikilinkResolutionCandidates(sourcePath, m[1])
		out = append(out, LinkResolutionCandidates{Primary: primary[0], Candidates: candidates})
	}
	return out
}

func wikilinkResolutionCandidates(sourcePath, raw string) []string {
	parts := strings.SplitN(raw, "|", 2)
	var candidates []string
	for _, part := range parts {
		for _, candidate := range pageTargetCandidates(sourcePath, part) {
			candidates = appendUniqueString(candidates, candidate)
		}
	}
	return candidates
}

func pageTargetCandidates(sourcePath, raw string) []string {
	target := stripWikilinkAnchor(raw)
	if target == "" || isExternalWikilink(target) {
		return nil
	}
	var candidates []string
	if hasPathSyntax(target) {
		candidates = appendUniqueString(candidates, normalizeRelativePagePath(sourcePath, target))
	}
	candidates = appendUniqueString(candidates, normalizeWikilinkTarget(target))
	return candidates
}

func stripWikilinkAnchor(target string) string {
	target = strings.TrimSpace(target)
	if i := strings.Index(target, "#"); i >= 0 {
		target = target[:i]
	}
	return strings.TrimSpace(target)
}

func isExternalWikilink(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	return strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://")
}

// ExtractEdges returns the typed outbound references in a note: wikilinks from
// prose, related targets from frontmatter, and repo-path references from prose.
// Wikilink targets are normalized; related targets are kept verbatim for the
// graph to resolve and classify; an inline-path candidate is emitted only when
// it matches a dir/dir/file.ext shape and resolves to an existing file under
// root, which suppresses false positives. Edges are unique by (kind, target).
func ExtractEdges(root string, note Note) []Edge {
	// Non-markdown wiki pages are link sinks: resolvable targets that emit no out
	// edges. Their prose is not markdown, so wikilink, markdown-link, related, and
	// inline-path extraction would only manufacture false edges from arbitrary
	// tokens. They also carry no code bindings, so drift never runs on them.
	if !IsMarkdownPath(note.Path) {
		return nil
	}
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

	visibleBody := maskMarkdownCode(note.Body)
	for _, m := range wikilinkRe.FindAllStringSubmatch(visibleBody, -1) {
		candidates := wikilinkCandidates(m[1])
		if len(candidates) > 0 {
			add(candidates[0], EdgeWikilink)
		}
	}
	for _, group := range extractMarkdownLinkResolutionCandidates(note.Path, visibleBody) {
		if markdownLinkTargetsExistingNonMarkdown(root, group.Candidates) {
			continue
		}
		add(group.Primary, EdgeMarkdownLink)
	}
	for _, target := range fmStringList(note.Frontmatter, "related") {
		add(strings.TrimSpace(target), EdgeRelated)
	}
	for _, m := range repoPathFindRe.FindAllStringSubmatch(visibleBody, -1) {
		token := cleanRepoPathToken(m[0])
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

func extractMarkdownLinkResolutionCandidates(sourcePath, body string) []LinkResolutionCandidates {
	var out []LinkResolutionCandidates
	for _, m := range markdownLinkRe.FindAllStringSubmatch(body, -1) {
		if strings.HasPrefix(m[0], "!") {
			continue
		}
		target, ok := markdownPageTarget(m[1])
		if !ok {
			continue
		}
		primaryCandidates := pageTargetCandidates("", target)
		if len(primaryCandidates) == 0 {
			continue
		}
		candidates := pageTargetCandidates(sourcePath, target)
		out = append(out, LinkResolutionCandidates{Primary: primaryCandidates[len(primaryCandidates)-1], Candidates: candidates})
	}
	return out
}

func markdownPageTarget(raw string) (string, bool) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", false
	}
	if strings.HasPrefix(target, "<") {
		end := strings.Index(target, ">")
		if end < 0 {
			return "", false
		}
		target = target[1:end]
	} else if fields := strings.Fields(target); len(fields) > 0 {
		target = fields[0]
	}
	target = strings.TrimSpace(target)
	if target == "" || strings.HasPrefix(target, "#") || isExternalMarkdownTarget(target) {
		return "", false
	}
	if unescaped, err := url.PathUnescape(target); err == nil {
		target = unescaped
	}
	if i := strings.IndexAny(target, "?#"); i >= 0 {
		target = target[:i]
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(strings.TrimRight(target, "/")))
	if ext != "" && ext != ".md" && ext != ".markdown" {
		return "", false
	}
	return target, true
}

func isExternalMarkdownTarget(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	return strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") || strings.HasPrefix(t, "mailto:")
}

func markdownLinkTargetsExistingNonMarkdown(root string, candidates []string) bool {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if markdownPageExists(root, candidate) {
			return false
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate)))
		if err != nil {
			continue
		}
		if info.IsDir() {
			return true
		}
		if !strings.EqualFold(filepath.Ext(candidate), ".md") {
			return true
		}
	}
	return false
}

func markdownPageExists(root, candidate string) bool {
	for _, ext := range []string{".md", ".markdown"} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate+ext)))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// CodeRefs returns the unique repo-relative paths a note binds to code through a
// related: frontmatter entry or an inline path ref: targets that resolve to
// an existing, non-directory, non-markdown file under root. It is the doc-to-code
// binding set drift detection watches; wikilinks and markdown targets are
// excluded, since those are doc-to-doc graph edges. Paths are slash-separated.
func CodeRefs(root string, note Note) []string {
	if !IsMarkdownPath(note.Path) {
		return nil
	}
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

// maskMarkdownCode replaces inline code spans and fenced code blocks with
// spaces while preserving newlines and byte length.
func maskMarkdownCode(body string) string {
	if body == "" {
		return body
	}
	b := []byte(body)
	masked := make([]bool, len(b))
	maskFencedCode(b, masked)
	maskInlineCode(b, masked)

	out := make([]byte, len(b))
	copy(out, b)
	for i := range out {
		if masked[i] && out[i] != '\n' && out[i] != '\r' {
			out[i] = ' '
		}
	}
	return string(out)
}

// maskFencedCode marks fenced code blocks opened by backticks or tildes.
func maskFencedCode(b []byte, masked []bool) {
	for lineStart := 0; lineStart < len(b); {
		lineEnd := lineStart
		for lineEnd < len(b) && b[lineEnd] != '\n' {
			lineEnd++
		}
		lineNext := lineEnd
		if lineNext < len(b) {
			lineNext++
		}
		marker, count, ok := fenceMarker(b[lineStart:lineEnd])
		if !ok {
			lineStart = lineNext
			continue
		}

		closeEnd := len(b)
		search := lineNext
		for search < len(b) {
			end := search
			for end < len(b) && b[end] != '\n' {
				end++
			}
			next := end
			if next < len(b) {
				next++
			}
			closeMarker, closeCount, closeOK := fenceMarker(b[search:end])
			if closeOK && closeMarker == marker && closeCount >= count {
				closeEnd = next
				break
			}
			search = next
		}
		for i := lineStart; i < closeEnd; i++ {
			masked[i] = true
		}
		lineStart = closeEnd
	}
}

// maskInlineCode marks inline backtick code spans outside fenced blocks.
func maskInlineCode(b []byte, masked []bool) {
	for i := 0; i < len(b); i++ {
		if masked[i] || b[i] != '`' || escapedAt(b, i) {
			continue
		}
		run := countRun(b, i, '`')
		close := findInlineCodeClose(b, masked, i+run, run)
		if close < 0 {
			i += run - 1
			continue
		}
		for j := i; j < close+run; j++ {
			masked[j] = true
		}
		i = close + run - 1
	}
}

// fenceMarker returns the marker and marker length for a markdown fence line.
func fenceMarker(line []byte) (byte, int, bool) {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	if i >= len(line) || line[i] != '`' && line[i] != '~' {
		return 0, 0, false
	}
	marker := line[i]
	count := countRun(line, i, marker)
	if count < 3 {
		return 0, 0, false
	}
	return marker, count, true
}

// findInlineCodeClose finds the next unescaped matching backtick run.
func findInlineCodeClose(b []byte, masked []bool, start, run int) int {
	for i := start; i < len(b); i++ {
		if b[i] == '\n' || b[i] == '\r' {
			return -1
		}
		if masked[i] || b[i] != '`' || escapedAt(b, i) {
			continue
		}
		n := countRun(b, i, '`')
		if n == run {
			return i
		}
		i += n - 1
	}
	return -1
}

// countRun counts consecutive target bytes starting at start.
func countRun(b []byte, start int, target byte) int {
	i := start
	for i < len(b) && b[i] == target {
		i++
	}
	return i - start
}

// escapedAt reports whether b[pos] is escaped by an odd number of backslashes.
func escapedAt(b []byte, pos int) bool {
	count := 0
	for i := pos - 1; i >= 0 && b[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

// cleanRepoPathToken removes punctuation that commonly surrounds prose paths.
func cleanRepoPathToken(token string) string {
	return strings.Trim(token, "<>()[]{}\"'`")
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

// GraphKey returns the path-aware graph node key for a vault-relative markdown
// path. Docs convention pages keep their collection-scoped key; ordinary notes
// preserve folders so foldered GitHub wiki pages can be resolved precisely.
func GraphKey(rel string) string {
	if coll := collectionFolderOf(rel); coll != "" {
		return coll + "/" + NormalizeLink(rel)
	}
	return normalizePagePath(rel)
}

// GitHubWikiDisplayAlias returns the display-title key for a normalized wiki
// page key whose filename slug uses hyphens for spaces. The collection prefix,
// when present, is preserved.
func GitHubWikiDisplayAlias(key string) string {
	prefix := ""
	base := key
	if i := strings.Index(key, "/"); i >= 0 {
		prefix = key[:i+1]
		base = key[i+1:]
	}
	alias := strings.ReplaceAll(base, "-", " ")
	if alias == base {
		return ""
	}
	return prefix + alias
}

func hasPathSyntax(target string) bool {
	target = filepath.ToSlash(strings.TrimSpace(target))
	return strings.Contains(target, "/") || strings.HasPrefix(target, ".")
}

func normalizeRelativePagePath(sourcePath, target string) string {
	target = filepath.ToSlash(strings.TrimSpace(target))
	if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") {
		base := filepath.ToSlash(filepath.Dir(filepath.ToSlash(sourcePath)))
		if base == "." {
			base = ""
		}
		target = filepath.ToSlash(filepath.Join(base, target))
	}
	return normalizePagePath(target)
}

func normalizePagePath(target string) string {
	target = strings.TrimSpace(filepath.ToSlash(target))
	if target == "" {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean("/" + filepath.FromSlash(target)))
	clean = strings.Trim(clean, "/")
	if clean == "" || clean == "." {
		return ""
	}
	parts := strings.Split(clean, "/")
	last := len(parts) - 1
	parts[last] = trimMarkdownExtension(parts[last])
	for i, part := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(part))
	}
	return strings.Join(parts, "/")
}

// trimMarkdownExtension strips a page extension from a path segment so a
// wikilink resolves to the file that carries it. Markdown extensions are
// stripped exactly as before; the supported non-markdown wiki extensions are
// stripped too, so a markdown [[Page]] resolves to Page.rst. Any other
// extension (or none) is returned unchanged, keeping markdown keying identical.
func trimMarkdownExtension(name string) string {
	if isSupportedPageExt(strings.ToLower(filepath.Ext(name))) {
		return strings.TrimSuffix(name, filepath.Ext(name))
	}
	return name
}

func appendUniqueString(items []string, item string) []string {
	if item == "" {
		return items
	}
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

// IsWikiStructuralPage reports whether rel is a GitHub wiki structural page
// that should not be treated as an orphan note.
func IsWikiStructuralPage(rel string) bool {
	switch strings.ToLower(filepath.Base(filepath.ToSlash(rel))) {
	case "_sidebar.md", "_footer.md", "home.md":
		return true
	default:
		return false
	}
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
	if isNonMarkdownPageExt(extOf(rel)) {
		return parseNonMarkdown(rel, raw), nil
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

// registryMarker is the first-content marker stamped into the generated docs
// registry (docs/INDEX.md). The walk skips any file carrying it so the derived
// table of contents never pollutes the index, search, the graph, or PageRank.
const registryMarker = "Generated by `stardust registry`"

// Scan walks root and returns the slash-separated relative paths of every
// markdown file, skipping any directory whose name is in ignore and the
// generated docs registry (detected by its filename and marker).
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
		if isSupportedPageExt(strings.ToLower(filepath.Ext(p))) {
			if isGeneratedRegistry(p) {
				return nil
			}
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

// isGeneratedRegistry reports whether the file at path is the generated docs
// registry: a file named INDEX.md whose head carries the registry marker. The
// filename pre-check keeps the content read off the hot path for ordinary notes,
// so only the registry leaf (the output path's base) is ever opened to confirm.
func isGeneratedRegistry(path string) bool {
	if !strings.EqualFold(filepath.Base(path), "INDEX.md") {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 256)
	n, _ := f.Read(buf)
	return strings.Contains(string(buf[:n]), registryMarker)
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
