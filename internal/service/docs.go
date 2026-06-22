package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/convention"
	"github.com/alxxpersonal/stardust/internal/memory"
)

// NewDocOptions describes a convention doc to create.
type NewDocOptions struct {
	Kind    string
	Title   string
	Status  string
	Related []string
	Governs []string
	Now     time.Time
}

// NewDocResult returns the created doc path and content.
type NewDocResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// NewDoc creates a spec, plan, or adr using the configured docs collection.
func (s *Service) NewDoc(ctx context.Context, opts NewDocOptions) (NewDocResult, error) {
	kind := strings.TrimSpace(opts.Kind)
	if kind != "spec" && kind != "plan" && kind != "adr" {
		return NewDocResult{}, fmt.Errorf("unsupported doc kind %q", opts.Kind)
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		return NewDocResult{}, fmt.Errorf("doc title is required")
	}
	status := opts.Status
	if status == "" {
		status = defaultDocStatus(kind)
	}
	if !convention.DocStatusAllowed(kind, status) {
		return NewDocResult{}, fmt.Errorf("status %q is not allowed for %s", status, kind)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	folder, err := s.docFolder(kind)
	if err != nil {
		return NewDocResult{}, err
	}
	rel, err := s.newDocPath(kind, folder, title, now)
	if err != nil {
		return NewDocResult{}, err
	}
	content := renderNewDoc(kind, title, status, opts.Related, opts.Governs, now)
	mem := memory.New(s.Layout.Root)
	if err := mem.Create(rel, content); err != nil {
		return NewDocResult{}, err
	}
	if err := s.reindexPath(ctx, rel); err != nil {
		return NewDocResult{}, err
	}
	return NewDocResult{Path: rel, Content: content}, nil
}

func (s *Service) docFolder(kind string) (string, error) {
	collection := collectionForDocKind(kind)
	cfg := filepath.Join(s.Layout.Collections(), collection, "config.toml")
	if _, err := os.Stat(cfg); os.IsNotExist(err) {
		return fallbackDocFolder(kind), nil
	}
	c, err := collections.LoadOne(s.Layout.Collections(), collection)
	if err != nil {
		return "", err
	}
	return normalizeRel(c.Cfg.Path), nil
}

func (s *Service) newDocPath(kind, folder, title string, now time.Time) (string, error) {
	if kind == "adr" {
		next, err := nextADRNumber(filepath.Join(s.Layout.Root, folder))
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(filepath.Join(folder, fmt.Sprintf("%04d-%s.md", next, slugify(title)))), nil
	}
	name := fmt.Sprintf("%s-%s.md", now.Format("2006-01-02-1504"), slugify(title))
	return filepath.ToSlash(filepath.Join(folder, name)), nil
}

func renderNewDoc(kind, title, status string, related, governs []string, now time.Time) string {
	date := now.Format("2006-01-02")
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", yamlString(title))
	fmt.Fprintf(&b, "type: %s\n", yamlString(kind))
	fmt.Fprintf(&b, "status: %s\n", yamlString(status))
	fmt.Fprintf(&b, "created: %s\n", yamlString(date))
	fmt.Fprintf(&b, "updated: %s\n", yamlString(date))
	if len(governs) > 0 {
		fmt.Fprintf(&b, "governs: %s\n", yamlList(governs))
	}
	if len(related) > 0 {
		fmt.Fprintf(&b, "related: %s\n", yamlList(related))
	}
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n## Summary\n\n## Details\n", title)
	return b.String()
}

func collectionForDocKind(kind string) string {
	switch kind {
	case "spec":
		return "specs"
	case "plan":
		return "plans"
	default:
		return kind
	}
}

func fallbackDocFolder(kind string) string {
	switch kind {
	case "spec":
		return "docs/specs"
	case "plan":
		return "docs/plans"
	default:
		return "docs/adr"
	}
}

func defaultDocStatus(kind string) string {
	switch kind {
	case "adr":
		return "Proposed"
	default:
		return "Draft"
	}
}

func yamlString(s string) string {
	return strconv.Quote(s)
}

func yamlList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = yamlString(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

var adrNumberPrefixRe = regexp.MustCompile(`^(\d+)`)

func nextADRNumber(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("read adr dir: %w", err)
	}
	maxNum := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := adrNumberPrefixRe.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if n > maxNum {
			maxNum = n
		}
	}
	return maxNum + 1, nil
}
