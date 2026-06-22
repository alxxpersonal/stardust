package agentsync

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// MigrationReport summarizes import-only sources that can be adopted into the
// canonical agent infrastructure source.
type MigrationReport struct {
	Canonical string `json:"canonical"`
	Adoptable []Item `json:"adoptable"`
	Duplicate []Item `json:"duplicate"`
	Loose     []Item `json:"loose"`
}

// BuildMigrationReport classifies discovered items against canonical sources.
func BuildMigrationReport(cfg Config, items []Item) MigrationReport {
	report := MigrationReport{Canonical: canonicalRoot(cfg)}
	canonical := map[string]bool{}
	for _, item := range items {
		key := itemKey(item)
		if !item.Source.ImportOnly {
			canonical[key] = true
		}
	}
	for _, item := range items {
		if !item.Source.ImportOnly {
			continue
		}
		if isLooseSource(item.Source) {
			report.Loose = append(report.Loose, item)
		}
		if canonical[itemKey(item)] {
			report.Duplicate = append(report.Duplicate, item)
			continue
		}
		report.Adoptable = append(report.Adoptable, item)
	}
	sortItems(report.Adoptable)
	sortItems(report.Duplicate)
	sortItems(report.Loose)
	return report
}

// Markdown renders a migration report for humans.
func (r MigrationReport) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Agent Sync Migration Report\n\n")
	if r.Canonical != "" {
		fmt.Fprintf(&b, "canonical: `%s`\n\n", r.Canonical)
	}
	writeReportSection(&b, "Adoptable", r.Adoptable)
	writeReportSection(&b, "Duplicates", r.Duplicate)
	writeReportSection(&b, "Loose Claude Assets", r.Loose)
	return b.String()
}

func writeReportSection(b *strings.Builder, title string, items []Item) {
	fmt.Fprintf(b, "## %s\n\n", title)
	if len(items) == 0 {
		b.WriteString("None.\n\n")
		return
	}
	fmt.Fprintln(b, "| kind | name | source |")
	fmt.Fprintln(b, "|---|---|---|")
	for _, item := range items {
		fmt.Fprintf(b, "| %s | %s | `%s` |\n", item.Kind, item.Name, item.SourcePath)
	}
	fmt.Fprintln(b)
}

func canonicalRoot(cfg Config) string {
	for _, src := range cfg.Sources {
		if src.ImportOnly {
			continue
		}
		base := filepath.Base(src.Path)
		if base == "skills" || base == "agents" {
			return filepath.Dir(src.Path)
		}
		return src.Path
	}
	return ""
}

func itemKey(item Item) string {
	return string(item.Kind) + "\x00" + item.Name
}

func isLooseSource(src Source) bool {
	name := strings.ToLower(src.Name)
	path := filepath.ToSlash(strings.ToLower(src.Path))
	return strings.Contains(name, "claude") || strings.Contains(path, "/.claude/")
}

func sortItems(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Name < items[j].Name
	})
}
