package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/manifest"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// adrCollection is the collection name whose records are ordered by number
// ascending and rendered with a numbered table instead of a dated one.
const adrCollection = "adr"

// defaultRegistryOrder is the fixed collection order for the docs registry,
// mirroring the CLI registry command.
var defaultRegistryOrder = []string{"specs", "plans", "adr", "research"}

// RegenerateRegistry regenerates docs/INDEX.md from the docs collections and
// refreshes the pinned agent manifest, mirroring `stardust registry`. It writes
// to docs/INDEX.md under the vault root.
func (s *Service) RegenerateRegistry(ctx context.Context) error {
	groups, err := s.Registry(defaultRegistryOrder)
	if err != nil {
		return err
	}
	out := filepath.Join(s.Layout.Root, "docs", "INDEX.md")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}
	if err := manifest.WriteRegistry(out, groups); err != nil {
		return err
	}
	return s.RefreshManifest(ctx)
}

// filenameDateRe matches a leading YYYY-MM-DD date in a filename.
var filenameDateRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)

// adrNumberRe matches a leading numeric prefix (an ADR number) in a filename.
var adrNumberRe = regexp.MustCompile(`^(\d+)`)

// ErrStaleIndex reports that a derived index is missing current disk files.
var ErrStaleIndex = errors.New("index looks empty or stale")

// Registry assembles the docs registry groups for the collections named in
// order, preserving that order. For each collection it loads the schema and
// queries its records through the existing ListRecords (newest filename first
// for dated collections, number ascending for adr), mapping each record's
// title/status frontmatter, path, derived date, and (for adr) number into a
// manifest.RegistryRecord. A collection with no config renders an empty group
// rather than an error, matching the no-collections behavior elsewhere.
func (s *Service) Registry(order []string) ([]manifest.RegistryGroup, error) {
	diskPaths, err := vault.Scan(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return nil, err
	}
	diskPaths = filterIgnored(diskPaths, s.Config.Ignore)

	groups := make([]manifest.RegistryGroup, 0, len(order))
	for _, name := range order {
		group := manifest.RegistryGroup{Name: name}

		c, err := collections.LoadOne(s.Layout.Collections(), name)
		if err != nil {
			// No config for this collection: empty section, not an error.
			groups = append(groups, group)
			continue
		}

		sort := "-path"
		if name == adrCollection {
			sort = "path"
		}
		list, err := s.ListRecords(context.Background(), name, nil, sort, 0, 0)
		if err != nil {
			return nil, err
		}
		if registryIndexStale(normalizeRel(c.Cfg.Path), diskPaths, list.Records) {
			return nil, fmt.Errorf("registry: %w, run stardust index", ErrStaleIndex)
		}

		group.Records = make([]manifest.RegistryRecord, 0, len(list.Records))
		for _, r := range list.Records {
			group.Records = append(group.Records, registryRecord(name, r))
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// registryIndexStale reports whether indexed records for folder disagree with
// the markdown files currently on disk.
func registryIndexStale(folder string, diskPaths []string, records []Record) bool {
	disk := map[string]bool{}
	for _, rel := range diskPaths {
		if pathInFolder(rel, folder) {
			disk[rel] = true
		}
	}
	if len(disk) == 0 && len(records) == 0 {
		return false
	}
	if len(disk) > 0 && len(records) == 0 {
		return true
	}
	indexed := map[string]bool{}
	for _, r := range records {
		indexed[r.Path] = true
		if !disk[r.Path] {
			return true
		}
	}
	for rel := range disk {
		if !indexed[rel] {
			return true
		}
	}
	return false
}

// pathInFolder reports whether rel is a descendant of folder.
func pathInFolder(rel, folder string) bool {
	rel = filepath.ToSlash(rel)
	folder = normalizeRel(folder)
	if folder == "" {
		return true
	}
	return rel == folder || filepath.Dir(rel) == folder || len(rel) > len(folder) && rel[:len(folder)+1] == folder+"/"
}

// registryRecord maps a queried record into a RegistryRecord, pulling status
// from frontmatter, deriving the date from a "date" field or the filename
// prefix, and (for adr) the number from the filename prefix.
func registryRecord(collection string, r Record) manifest.RegistryRecord {
	base := filepath.Base(r.Path)
	rec := manifest.RegistryRecord{
		Title:  r.Title,
		Status: frontmatterString(r.Frontmatter, "status"),
		Path:   r.Path,
		Date:   recordDate(r.Frontmatter, base),
	}
	if collection == adrCollection {
		if m := adrNumberRe.FindStringSubmatch(base); m != nil {
			rec.Number = m[1]
		}
	}
	return rec
}

// recordDate prefers an explicit "date" frontmatter field, falling back to a
// leading YYYY-MM-DD prefix in the filename. The date field may decode as a
// full RFC3339 timestamp (YAML parses unquoted dates into times), so only its
// leading YYYY-MM-DD is kept. Returns an empty string when neither is present.
func recordDate(frontmatter map[string]any, filename string) string {
	if d := frontmatterString(frontmatter, "date"); d != "" {
		if m := filenameDateRe.FindStringSubmatch(d); m != nil {
			return m[1]
		}
		return d
	}
	if m := filenameDateRe.FindStringSubmatch(filename); m != nil {
		return m[1]
	}
	return ""
}

// frontmatterString returns the string value of key in frontmatter, or "" when
// the key is absent or not a string.
func frontmatterString(frontmatter map[string]any, key string) string {
	if v, ok := frontmatter[key].(string); ok {
		return v
	}
	return ""
}
