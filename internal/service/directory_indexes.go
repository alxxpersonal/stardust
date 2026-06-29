package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/vault"
)

const (
	directoryIndexStartMarker = "<!-- stardust-directory-index:start -->"
	directoryIndexEndMarker   = "<!-- stardust-directory-index:end -->"
)

var directoryIndexDateRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)

// DirectoryIndexResult reports directory-index sync or check work.
type DirectoryIndexResult struct {
	Enabled  bool                  `json:"enabled"`
	Files    []DirectoryIndexFile  `json:"files"`
	Issues   []DirectoryIndexIssue `json:"issues,omitempty"`
	Markdown string                `json:"markdown"`
}

// DirectoryIndexFile describes one configured directory index.
type DirectoryIndexFile struct {
	Path    string `json:"path"`
	Entries int    `json:"entries"`
	Updated bool   `json:"updated"`
}

// DirectoryIndexIssue describes a missing or stale directory index.
type DirectoryIndexIssue struct {
	Severity string `json:"severity"`
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Detail   string `json:"detail"`
}

type directoryIndexEntry struct {
	Date    string
	File    string
	Link    string
	Purpose string
}

// SyncDirectoryIndexes writes configured per-directory indexes and reports what
// changed. Disabled or rootless config is a clean no-op.
func (s *Service) SyncDirectoryIndexes(ctx context.Context) (DirectoryIndexResult, error) {
	cfg := s.Config.Conventions.DirectoryIndexes.WithDefaults()
	res := DirectoryIndexResult{Enabled: cfg.Enabled}
	if !cfg.Enabled || len(cfg.Roots) == 0 {
		res.Markdown = renderDirectoryIndexResult("sync", res)
		return res, nil
	}
	dirs, err := s.directoryIndexDirs(cfg)
	if err != nil {
		return DirectoryIndexResult{}, err
	}
	sortDirectoryIndexDirsForWrite(dirs)
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return DirectoryIndexResult{}, err
		}
		file, updated, entries, err := s.syncDirectoryIndex(dir, cfg)
		if err != nil {
			return DirectoryIndexResult{}, err
		}
		res.Files = append(res.Files, DirectoryIndexFile{Path: file, Entries: entries, Updated: updated})
	}
	res.Markdown = renderDirectoryIndexResult("sync", res)
	return res, nil
}

// CheckDirectoryIndexes reports missing or stale configured directory indexes
// without writing files.
func (s *Service) CheckDirectoryIndexes(ctx context.Context) (DirectoryIndexResult, error) {
	cfg := s.Config.Conventions.DirectoryIndexes.WithDefaults()
	res := DirectoryIndexResult{Enabled: cfg.Enabled}
	if !cfg.Enabled || len(cfg.Roots) == 0 {
		res.Markdown = renderDirectoryIndexResult("check", res)
		return res, nil
	}
	dirs, err := s.directoryIndexDirs(cfg)
	if err != nil {
		return DirectoryIndexResult{}, err
	}
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return DirectoryIndexResult{}, err
		}
		file, stale, entries, err := s.checkDirectoryIndex(dir, cfg)
		if err != nil {
			return DirectoryIndexResult{}, err
		}
		res.Files = append(res.Files, DirectoryIndexFile{Path: file, Entries: entries})
		if stale {
			kind := "directory-index-stale"
			detail := "generated directory index block is stale"
			if _, err := os.Stat(filepath.Join(s.Layout.Root, filepath.FromSlash(file))); os.IsNotExist(err) {
				kind = "directory-index-missing"
				detail = "configured directory is missing its index file"
			}
			res.Issues = append(res.Issues, DirectoryIndexIssue{
				Severity: "warn",
				Kind:     kind,
				Path:     file,
				Detail:   detail,
			})
		}
	}
	res.Markdown = renderDirectoryIndexResult("check", res)
	return res, nil
}

func (s *Service) syncDirectoryIndex(dir string, cfg config.DirectoryIndexesConfig) (string, bool, int, error) {
	content, entries, err := s.expectedDirectoryIndexContent(dir, cfg)
	if err != nil {
		return "", false, 0, err
	}
	path := filepath.Join(s.Layout.Root, filepath.FromSlash(dir), cfg.Filename)
	rel := filepath.ToSlash(filepath.Join(dir, cfg.Filename))
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", false, 0, fmt.Errorf("read directory index %s: %w", rel, err)
	}
	if string(current) == content {
		return rel, false, entries, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false, 0, fmt.Errorf("create directory index dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", false, 0, fmt.Errorf("write directory index %s: %w", rel, err)
	}
	return rel, true, entries, nil
}

func (s *Service) checkDirectoryIndex(dir string, cfg config.DirectoryIndexesConfig) (string, bool, int, error) {
	expected, entries, err := s.expectedDirectoryIndexContent(dir, cfg)
	if err != nil {
		return "", false, 0, err
	}
	rel := filepath.ToSlash(filepath.Join(dir, cfg.Filename))
	current, err := os.ReadFile(filepath.Join(s.Layout.Root, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return rel, true, entries, nil
	}
	if err != nil {
		return "", false, 0, fmt.Errorf("read directory index %s: %w", rel, err)
	}
	return rel, string(current) != expected, entries, nil
}

func (s *Service) expectedDirectoryIndexContent(dir string, cfg config.DirectoryIndexesConfig) (string, int, error) {
	entries, err := s.directoryIndexEntries(dir, cfg)
	if err != nil {
		return "", 0, err
	}
	block := renderDirectoryIndexBlock(entries)
	path := filepath.Join(s.Layout.Root, filepath.FromSlash(dir), cfg.Filename)
	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultDirectoryIndexContent(dir, block), len(entries), nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("read directory index %s: %w", filepath.ToSlash(filepath.Join(dir, cfg.Filename)), err)
	}
	return replaceDirectoryIndexBlock(string(current), block), len(entries), nil
}

func (s *Service) directoryIndexEntries(dir string, cfg config.DirectoryIndexesConfig) ([]directoryIndexEntry, error) {
	abs := filepath.Join(s.Layout.Root, filepath.FromSlash(dir))
	children, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}
	ignore := newDirectoryIndexIgnore(s.Config.Ignore, cfg.Ignore)
	entries := make([]directoryIndexEntry, 0, len(children))
	for _, child := range children {
		name := child.Name()
		rel := filepath.ToSlash(filepath.Join(dir, name))
		if name == cfg.Filename || ignore.skip(rel, name) {
			continue
		}
		entry := directoryIndexEntry{
			Date:    directoryIndexDate(name),
			File:    name,
			Link:    name,
			Purpose: "File.",
		}
		if child.IsDir() {
			entry.Link = name + "/"
			entry.Purpose = s.directoryIndexPurpose(filepath.ToSlash(filepath.Join(rel, cfg.Filename)), "Directory.")
			entries = append(entries, entry)
			continue
		}
		if strings.EqualFold(filepath.Ext(name), ".md") {
			entry.Purpose = s.directoryIndexPurpose(rel, "Markdown document.")
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			if entries[i].Date == "-" {
				return false
			}
			if entries[j].Date == "-" {
				return true
			}
			return entries[i].Date > entries[j].Date
		}
		return strings.ToLower(entries[i].File) < strings.ToLower(entries[j].File)
	})
	return entries, nil
}

func (s *Service) directoryIndexPurpose(rel, fallback string) string {
	note, err := vault.Parse(s.Layout.Root, rel)
	if err != nil {
		return fallback
	}
	title := strings.TrimSpace(note.Title)
	if title == "" {
		return fallback
	}
	return ensureSentence(title)
}

func (s *Service) directoryIndexDirs(cfg config.DirectoryIndexesConfig) ([]string, error) {
	ignore := newDirectoryIndexIgnore(s.Config.Ignore, cfg.Ignore)
	seen := map[string]bool{}
	var dirs []string
	for _, root := range cfg.Roots {
		root = cleanDirectoryIndexPath(root)
		if root == "" {
			continue
		}
		abs := filepath.Join(s.Layout.Root, filepath.FromSlash(root))
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			continue
		}
		err := filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(s.Layout.Root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if rel != root && ignore.skip(rel, d.Name()) {
				return filepath.SkipDir
			}
			if !seen[rel] {
				seen[rel] = true
				dirs = append(dirs, rel)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk directory index root %s: %w", root, err)
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

func sortDirectoryIndexDirsForWrite(dirs []string) {
	sort.SliceStable(dirs, func(i, j int) bool {
		iDepth := strings.Count(dirs[i], "/")
		jDepth := strings.Count(dirs[j], "/")
		if iDepth != jDepth {
			return iDepth > jDepth
		}
		return dirs[i] < dirs[j]
	})
}

func (s *Service) directoryIndexPathSet() (map[string]bool, error) {
	cfg := s.Config.Conventions.DirectoryIndexes.WithDefaults()
	out := map[string]bool{}
	if !cfg.Enabled || len(cfg.Roots) == 0 {
		return out, nil
	}
	dirs, err := s.directoryIndexDirs(cfg)
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		out[filepath.ToSlash(filepath.Join(dir, cfg.Filename))] = true
	}
	return out, nil
}

func renderDirectoryIndexBlock(entries []directoryIndexEntry) string {
	var b strings.Builder
	b.WriteString(directoryIndexStartMarker + "\n")
	b.WriteString("| Date | File | Purpose |\n")
	b.WriteString("|---|---|---|\n")
	if len(entries) == 0 {
		b.WriteString("| - | - | (empty) |\n")
	} else {
		for _, entry := range entries {
			fmt.Fprintf(&b, "| %s | [%s](%s) | %s |\n",
				escapeTableCell(entry.Date),
				escapeTableCell(entry.File),
				escapeMarkdownLink(entry.Link),
				escapeTableCell(entry.Purpose),
			)
		}
	}
	b.WriteString(directoryIndexEndMarker + "\n")
	return b.String()
}

func defaultDirectoryIndexContent(dir, block string) string {
	return fmt.Sprintf("# %s Index\n\n%s", directoryIndexTitle(dir), block)
}

func replaceDirectoryIndexBlock(current, block string) string {
	start := strings.Index(current, directoryIndexStartMarker)
	end := strings.Index(current, directoryIndexEndMarker)
	if start >= 0 && end >= start {
		end += len(directoryIndexEndMarker)
		prefix := strings.TrimRight(current[:start], "\n")
		next := strings.TrimRight(block, "\n")
		if prefix != "" {
			next = prefix + "\n\n" + next
		}
		if suffix := strings.TrimLeft(current[end:], "\n"); suffix != "" {
			next += "\n\n" + suffix
		} else {
			next += "\n"
		}
		return next
	}
	return strings.TrimRight(current, "\n") + "\n\n" + block
}

func renderDirectoryIndexResult(action string, res DirectoryIndexResult) string {
	var b strings.Builder
	b.WriteString("# Directory indexes\n\n")
	if !res.Enabled {
		b.WriteString("Directory indexes are disabled.\n")
		return b.String()
	}
	updated := 0
	for _, file := range res.Files {
		if file.Updated {
			updated++
		}
	}
	if action == "check" {
		fmt.Fprintf(&b, "%d index file(s) checked, %d issue(s).\n", len(res.Files), len(res.Issues))
	} else {
		fmt.Fprintf(&b, "%d index file(s), %d updated.\n", len(res.Files), updated)
	}
	if len(res.Files) > 0 {
		b.WriteString("\n| File | Entries | Updated |\n")
		b.WriteString("|---|---:|---|\n")
		for _, file := range res.Files {
			fmt.Fprintf(&b, "| %s | %d | %t |\n", file.Path, file.Entries, file.Updated)
		}
	}
	if len(res.Issues) > 0 {
		b.WriteString("\n## Issues\n\n")
		for _, issue := range res.Issues {
			fmt.Fprintf(&b, "- **%s** [%s] `%s` - %s\n", issue.Severity, issue.Kind, issue.Path, issue.Detail)
		}
	}
	return b.String()
}

type directoryIndexIgnore struct {
	names map[string]bool
	paths map[string]bool
}

func newDirectoryIndexIgnore(groups ...[]string) directoryIndexIgnore {
	ig := directoryIndexIgnore{names: map[string]bool{}, paths: map[string]bool{}}
	for _, group := range groups {
		for _, item := range group {
			item = cleanDirectoryIndexPath(item)
			if item == "" {
				continue
			}
			if strings.Contains(item, "/") {
				ig.paths[item] = true
				continue
			}
			ig.names[item] = true
		}
	}
	return ig
}

func (ig directoryIndexIgnore) skip(rel, name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	if ig.names[name] {
		return true
	}
	rel = cleanDirectoryIndexPath(rel)
	for path := range ig.paths {
		if rel == path || strings.HasPrefix(rel, path+"/") {
			return true
		}
	}
	for _, seg := range strings.Split(rel, "/") {
		if ig.names[seg] {
			return true
		}
	}
	return false
}

func cleanDirectoryIndexPath(path string) string {
	clean := filepath.ToSlash(filepath.Clean("/" + filepath.FromSlash(strings.TrimSpace(path))))
	return strings.Trim(clean, "/")
}

func directoryIndexTitle(dir string) string {
	dir = cleanDirectoryIndexPath(dir)
	if dir == "" {
		return "Directory"
	}
	return filepath.Base(filepath.FromSlash(dir))
}

func directoryIndexDate(name string) string {
	if m := directoryIndexDateRe.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return "-"
}

func ensureSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	switch s[len(s)-1] {
	case '.', '!', '?':
		return s
	default:
		return s + "."
	}
}

func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func escapeMarkdownLink(s string) string {
	return strings.ReplaceAll(s, ")", "%29")
}
