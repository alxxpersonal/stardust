package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/memory"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// Predicate is a single frontmatter filter for ListRecords (see index.Predicate
// for the field, op, and value semantics). It is re-exported here so surfaces
// build filters without importing the index package directly.
type Predicate = index.Predicate

// CollectionInfo describes one collection: its name, schema, vault folder, and
// the number of records currently indexed under that folder.
type CollectionInfo struct {
	Name        string              `json:"name"`
	Path        string              `json:"path"`
	Description string              `json:"description"`
	Fields      []collections.Field `json:"fields"`
	Records     int                 `json:"records"`
}

// Record is a single note in a collection: its vault path, title, decoded
// frontmatter columns, and (when read whole) its markdown body.
type Record struct {
	Path        string         `json:"path"`
	Title       string         `json:"title"`
	Frontmatter map[string]any `json:"frontmatter"`
	Body        string         `json:"body"`
}

// RecordList is a page of records for a collection, echoing the resolved folder
// the records were scoped to.
type RecordList struct {
	Collection string   `json:"collection"`
	Folder     string   `json:"folder"`
	Records    []Record `json:"records"`
}

// ListCollections returns every configured collection with a live record count,
// sorted by name. A missing collections directory yields an empty list.
func (s *Service) ListCollections(ctx context.Context) ([]CollectionInfo, error) {
	cols, err := collections.Load(s.Layout.Collections())
	if err != nil {
		return nil, err
	}
	out := make([]CollectionInfo, 0, len(cols))
	for _, c := range cols {
		info, err := s.collectionInfo(ctx, c)
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

// GetCollection returns a single collection with its live record count.
func (s *Service) GetCollection(ctx context.Context, name string) (CollectionInfo, error) {
	c, err := collections.LoadOne(s.Layout.Collections(), name)
	if err != nil {
		return CollectionInfo{}, err
	}
	return s.collectionInfo(ctx, c)
}

// ListRecords resolves the named collection to its folder and returns the
// records under it, filtered by frontmatter predicates and ordered by sort (a
// frontmatter field, or "path" / "updated_at", with an optional leading "-" for
// descending). A non-positive limit means no limit; offset paginates.
func (s *Service) ListRecords(ctx context.Context, name string, filter []Predicate, sort string, limit, offset int) (RecordList, error) {
	c, err := collections.LoadOne(s.Layout.Collections(), name)
	if err != nil {
		return RecordList{}, err
	}
	folder := normalizeRel(c.Cfg.Path)
	rows, err := s.store.ListRecords(ctx, folder, filter, sort, limit, offset)
	if err != nil {
		return RecordList{}, err
	}
	records := make([]Record, 0, len(rows))
	for _, r := range rows {
		records = append(records, Record{Path: r.Path, Title: r.Title, Frontmatter: r.Frontmatter})
	}
	return RecordList{Collection: name, Folder: folder, Records: records}, nil
}

// GetRecord parses the note at a vault-relative path and returns it as a Record,
// including both its frontmatter and its markdown body.
func (s *Service) GetRecord(_ context.Context, path string) (Record, error) {
	clean, err := cleanRel(path)
	if err != nil {
		return Record{}, err
	}
	n, err := vault.Parse(s.Layout.Root, clean)
	if err != nil {
		return Record{}, err
	}
	fm := n.Frontmatter
	if fm == nil {
		fm = map[string]any{}
	}
	return Record{Path: n.Path, Title: n.Title, Frontmatter: fm, Body: n.Body}, nil
}

// CreateRecord validates fields against the named collection's schema, composes
// a markdown note (YAML frontmatter then body) under the collection's folder,
// writes it via the path-confined memory store, reindexes it, and returns the
// new record. The filename is a unique slug of the record's title-ish field.
// Like the other write paths, it does not git-commit; it writes to disk and
// reindexes only.
func (s *Service) CreateRecord(ctx context.Context, name string, fields map[string]any, body string) (Record, error) {
	c, err := collections.LoadOne(s.Layout.Collections(), name)
	if err != nil {
		return Record{}, err
	}
	if fields == nil {
		fields = map[string]any{}
	}
	if err := collections.Validate(fields, c.Cfg.Fields); err != nil {
		return Record{}, fmt.Errorf("create record in %s: %w", name, err)
	}

	folder := normalizeRel(c.Cfg.Path)
	rel, err := s.uniqueRecordPath(folder, recordSlug(fields, c.Cfg.Fields))
	if err != nil {
		return Record{}, err
	}

	content, err := composeNote(fields, body)
	if err != nil {
		return Record{}, err
	}
	mem := memory.New(s.Layout.Root)
	if err := mem.Create(rel, content); err != nil {
		return Record{}, err
	}
	if err := s.reindexPath(ctx, rel); err != nil {
		return Record{}, err
	}
	return s.GetRecord(ctx, rel)
}

// PatchRecord reads the note at path, merges fields into its frontmatter (a nil
// value deletes a key), optionally replaces the body, validates the merged
// frontmatter against the owning collection's schema when one matches, rewrites
// the note, reindexes it, and returns the updated record.
func (s *Service) PatchRecord(ctx context.Context, path string, fields map[string]any, body *string) (Record, error) {
	clean, err := cleanRel(path)
	if err != nil {
		return Record{}, err
	}
	n, err := vault.Parse(s.Layout.Root, clean)
	if err != nil {
		return Record{}, err
	}

	merged := map[string]any{}
	for k, v := range n.Frontmatter {
		merged[k] = v
	}
	for k, v := range fields {
		if v == nil {
			delete(merged, k)
			continue
		}
		merged[k] = v
	}

	if schema, ok, err := s.schemaForPath(clean); err != nil {
		return Record{}, err
	} else if ok {
		if err := collections.Validate(merged, schema); err != nil {
			return Record{}, fmt.Errorf("patch record %s: %w", clean, err)
		}
	}

	newBody := n.Body
	if body != nil {
		newBody = *body
	}
	content, err := composeNote(merged, newBody)
	if err != nil {
		return Record{}, err
	}
	mem := memory.New(s.Layout.Root)
	if err := mem.Delete(clean); err != nil {
		return Record{}, err
	}
	if err := mem.Create(clean, content); err != nil {
		return Record{}, err
	}
	if err := s.reindexPath(ctx, clean); err != nil {
		return Record{}, err
	}
	return s.GetRecord(ctx, clean)
}

// ArchiveRecord removes a record from the vault and prunes it from the index. It
// reuses the memory delete verb, so the operation is path-confined to the vault
// root.
func (s *Service) ArchiveRecord(ctx context.Context, path string) error {
	clean, err := cleanRel(path)
	if err != nil {
		return err
	}
	mem := memory.New(s.Layout.Root)
	if err := mem.Delete(clean); err != nil {
		return err
	}
	return s.reindexPath(ctx, clean)
}

// --- Helpers ---

// collectionInfo builds a CollectionInfo for a loaded collection, counting the
// records currently indexed under its folder.
func (s *Service) collectionInfo(ctx context.Context, c collections.Collection) (CollectionInfo, error) {
	folder := normalizeRel(c.Cfg.Path)
	rows, err := s.store.ListRecords(ctx, folder, nil, "", 0, 0)
	if err != nil {
		return CollectionInfo{}, err
	}
	return CollectionInfo{
		Name:        c.Name,
		Path:        folder,
		Description: c.Cfg.Description,
		Fields:      c.Cfg.Fields,
		Records:     len(rows),
	}, nil
}

// schemaForPath finds the collection whose folder contains path and returns its
// fields. ok is false when no collection owns the path.
func (s *Service) schemaForPath(rel string) (fields []collections.Field, ok bool, err error) {
	cols, err := collections.Load(s.Layout.Collections())
	if err != nil {
		return nil, false, err
	}
	rel = filepath.ToSlash(rel)
	for _, c := range cols {
		folder := normalizeRel(c.Cfg.Path)
		if folder == "" {
			continue
		}
		if strings.HasPrefix(rel, folder+"/") {
			return c.Cfg.Fields, true, nil
		}
	}
	return nil, false, nil
}

// uniqueRecordPath returns a vault-relative ".md" path under folder built from
// slug, suffixing "-2", "-3", ... until no file exists at that path.
func (s *Service) uniqueRecordPath(folder, slug string) (string, error) {
	mem := memory.New(s.Layout.Root)
	base := slug
	for i := 1; ; i++ {
		candidate := slug
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		rel := candidate + ".md"
		if folder != "" {
			rel = folder + "/" + rel
		}
		if _, err := mem.View(rel); err != nil {
			return rel, nil
		}
	}
}

// recordSlug derives a filename slug from a record. It prefers the first present
// string-valued required field, then a "title"/"name" field, then any string
// field, falling back to "record".
func recordSlug(fields map[string]any, schema []collections.Field) string {
	for _, f := range schema {
		if !f.Required {
			continue
		}
		if s, ok := fields[f.Name].(string); ok && strings.TrimSpace(s) != "" {
			return slugify(s)
		}
	}
	for _, key := range []string{"title", "name"} {
		if s, ok := fields[key].(string); ok && strings.TrimSpace(s) != "" {
			return slugify(s)
		}
	}
	for _, f := range schema {
		if s, ok := fields[f.Name].(string); ok && strings.TrimSpace(s) != "" {
			return slugify(s)
		}
	}
	return "record"
}

// composeNote renders frontmatter and body into a markdown note: a YAML
// frontmatter block delimited by --- lines, a blank line, then the body. An
// empty frontmatter map still emits a (minimal) block so the note round-trips.
func composeNote(frontmatter map[string]any, body string) (string, error) {
	var fm strings.Builder
	enc := yaml.NewEncoder(&fm)
	enc.SetIndent(2)
	if err := enc.Encode(frontmatter); err != nil {
		return "", fmt.Errorf("encode frontmatter: %w", err)
	}
	if err := enc.Close(); err != nil {
		return "", fmt.Errorf("encode frontmatter: %w", err)
	}
	body = strings.TrimLeft(body, "\n")
	return "---\n" + fm.String() + "---\n\n" + body, nil
}

// normalizeRel reduces a schema folder to a clean slash-separated vault-relative
// path with no leading or trailing slashes.
func normalizeRel(p string) string {
	clean := filepath.ToSlash(filepath.Clean("/" + filepath.FromSlash(p)))
	return strings.Trim(clean, "/")
}

// cleanRel cleans and confines a vault-relative path, rejecting empties.
func cleanRel(path string) (string, error) {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+filepath.FromSlash(path))), "/")
	if clean == "" {
		return "", fmt.Errorf("invalid record path: %q", path)
	}
	return clean, nil
}
