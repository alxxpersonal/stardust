package service

import (
	"context"
	"path/filepath"
	"regexp"

	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/manifest"
)

// adrCollection is the collection name whose records are ordered by number
// ascending and rendered with a numbered table instead of a dated one.
const adrCollection = "adr"

// filenameDateRe matches a leading YYYY-MM-DD date in a filename.
var filenameDateRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)

// adrNumberRe matches a leading numeric prefix (an ADR number) in a filename.
var adrNumberRe = regexp.MustCompile(`^(\d+)`)

// Registry assembles the docs registry groups for the collections named in
// order, preserving that order. For each collection it loads the schema and
// queries its records through the existing ListRecords (newest filename first
// for dated collections, number ascending for adr), mapping each record's
// title/status frontmatter, path, derived date, and (for adr) number into a
// manifest.RegistryRecord. A collection with no config renders an empty group
// rather than an error, matching the no-collections behavior elsewhere.
func (s *Service) Registry(order []string) ([]manifest.RegistryGroup, error) {
	groups := make([]manifest.RegistryGroup, 0, len(order))
	for _, name := range order {
		group := manifest.RegistryGroup{Name: name}

		if _, err := collections.LoadOne(s.Layout.Collections(), name); err != nil {
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

		group.Records = make([]manifest.RegistryRecord, 0, len(list.Records))
		for _, r := range list.Records {
			group.Records = append(group.Records, registryRecord(name, r))
		}
		groups = append(groups, group)
	}
	return groups, nil
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
