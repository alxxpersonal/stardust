package rpc

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// updateGolden, when set via `go test ./rpc/ -run Golden -update`, rewrites the
// checked-in golden files from the representative values below instead of
// asserting against them. Leave it off in CI: the goldens pin the wire shape, so
// any field rename, retag, or struct change diverges from the committed bytes and
// fails the test.
var updateGolden = flag.Bool("update", false, "rewrite rpc/testdata golden files")

// goldenCase is one pinned wire artifact: a method name plus a representative
// value (a Params or a Result) whose marshaled JSON is frozen on disk. The file
// name is "<method>.<kind>.json" with the method slash replaced by a dash.
type goldenCase struct {
	method string
	kind   string // "params" or "result"
	value  any
}

// goldenCases enumerates every method's Params and Result with a representative,
// fully populated value. The set mirrors the canonical method table in the spec
// for the full operation set (the record seam plus status, then the remaining
// read and write operations). A new method or a changed shape must be reflected
// here and in a regenerated golden file, which makes the wire change explicit in
// review.
func goldenCases() []goldenCase {
	body := "new body"
	return []goldenCase{
		{"status", "result", StatusResult{
			Root:        "/vault",
			Notes:       12,
			Chunks:      34,
			LastIndexed: "abc123",
			EmbedModel:  "model-x",
			Vectors:     true,
			Reranker:    false,
		}},
		{"record/create", "params", CreateRecordParams{
			Collection: "tasks",
			Fields:     map[string]any{"title": "ship it"},
			Body:       "do the thing",
		}},
		{"record/create", "result", Record{
			Path:        "20-Active/Tasks/ship-it.md",
			Title:       "ship it",
			Frontmatter: map[string]any{"status": "active"},
			Body:        "do the thing",
		}},
		{"record/get", "params", RecordParams{Path: "notes/a.md"}},
		{"record/get", "result", Record{
			Path:        "notes/a.md",
			Title:       "A",
			Frontmatter: map[string]any{"status": "active"},
			Body:        "hello",
		}},
		{"record/list", "params", ListRecordsParams{
			Collection: "tasks",
			Filter:     []Predicate{{Field: "status", Op: "eq", Value: "active"}},
			Sort:       "-updated_at",
			Limit:      10,
			Offset:     5,
		}},
		{"record/list", "result", RecordList{
			Collection: "tasks",
			Folder:     "20-Active/Tasks",
			Records: []Record{{
				Path:        "20-Active/Tasks/a.md",
				Title:       "A",
				Frontmatter: map[string]any{"status": "active"},
				Body:        "hello",
			}},
		}},
		{"record/patch", "params", PatchRecordParams{
			Path:   "notes/a.md",
			Fields: map[string]any{"status": "done"},
			Body:   &body,
		}},
		{"record/patch", "result", Record{
			Path:        "notes/a.md",
			Title:       "A",
			Frontmatter: map[string]any{"status": "done"},
			Body:        "new body",
		}},
		{"record/delete", "params", RecordParams{Path: "notes/a.md"}},
		{"record/delete", "result", DeleteResult{Path: "notes/a.md", Status: "deleted"}},
		{"query", "params", QueryParams{Query: "ship it", Limit: 10}},
		{"query", "result", QueryResult{
			Query:         "ship it",
			Mode:          "hybrid + rerank",
			RetrievalMode: "hybrid-semantic",
			Reranked:      true,
			Hits: []Hit{{
				Path:    "20-Active/Tasks/a.md",
				Title:   "A",
				Heading: "Plan",
				Snippet: "ship it tonight",
				Score:   0.91,
			}},
		}},
		{"bundle", "params", BundleParams{Task: "ship the release", Budget: 4000}},
		{"bundle", "result", BundleResult{
			Task: "ship the release",
			Items: []BundleItem{{
				Path:       "20-Active/Tasks/a.md",
				Title:      "A",
				Snippet:    "ship it tonight",
				Score:      0.91,
				Provenance: []string{"keyword", "semantic"},
			}},
			Markdown:      "# Context bundle\n\nship it tonight\n",
			Tokens:        42,
			RetrievalMode: "hybrid-semantic",
			CommitsBehind: 0,
		}},
		{"graph", "result", GraphResult{
			Notes:   3,
			Links:   2,
			Orphans: []string{"notes/orphan.md"},
			Broken:  []BrokenLink{{From: "notes/a.md", Target: "missing", Kind: "wikilink"}},
			PageRank: []PageRankEntry{{
				Path:  "notes/a.md",
				Title: "A",
				Score: 0.42,
			}},
		}},
		{"digest", "params", DigestParams{Since: "abc123", Advance: true}},
		{"digest", "result", DigestResult{
			Since:    "abc123",
			Head:     "def456",
			Changed:  2,
			Markdown: "# Digest\n\n2 notes changed.\n",
		}},
		{"check", "result", CheckResult{
			Issues: []Issue{{
				Severity: "warn",
				Kind:     "broken_link",
				Path:     "notes/a.md",
				Detail:   "link target missing",
			}},
			Errors:   0,
			Warnings: 1,
			Markdown: "# Vault check\n\n0 errors, 1 warning.\n",
		}},
		{"note/get", "params", NoteParams{Path: "notes/a.md"}},
		{"note/get", "result", Note{
			Path:        "notes/a.md",
			Title:       "A",
			Tags:        []string{"project", "active"},
			Links:       []string{"B"},
			LinkTargets: []LinkTarget{{Link: "B", Path: "notes/b.md"}},
			Frontmatter: map[string]any{"status": "active"},
			Body:        "hello",
		}},
		{"collection/list", "result", []Collection{{
			Name:        "tasks",
			Path:        "20-Active/Tasks",
			Description: "task records",
			Fields: []Field{{
				Name:     "status",
				Type:     "enum",
				Required: true,
				Enum:     []string{"active", "done"},
				Default:  "active",
			}},
			Records: 7,
		}}},
		{"collection/get", "params", CollectionParams{Name: "tasks"}},
		{"collection/get", "result", Collection{
			Name:        "tasks",
			Path:        "20-Active/Tasks",
			Description: "task records",
			Fields: []Field{{
				Name:     "status",
				Type:     "enum",
				Required: true,
				Enum:     []string{"active", "done"},
				Default:  "active",
			}},
			Records: 7,
		}},
		{"mount/list", "result", []Mount{{
			Name:   "exo-jobs",
			Kind:   "mcp",
			Target: "exo-jobs",
			Args:   []string{"serve", "--mcp"},
			Tool:   "jobs",
		}}},
		{"index/run", "params", IndexParams{Since: "abc123"}},
		{"index/run", "result", IndexStats{Indexed: 5, Skipped: 2, Deleted: 1, Vectors: true}},
		{"index/rebuild", "result", IndexStats{Indexed: 12, Skipped: 0, Deleted: 0, Vectors: true}},
		{"archive", "params", ArchiveParams{Dest: ".stardust/archives"}},
		{"archive", "result", ArchiveResult{Path: ".stardust/archives/vault-20260625.bundle"}},
		{"cron/list", "result", []CronJob{{
			Name:     "nightly-index",
			Schedule: "0 3 * * *",
			Kind:     "command",
			Command:  "stardust index",
		}}},
		{"cron/run", "params", CronRunParams{Name: "nightly-index"}},
		{"cron/run", "result", CronRunResult{Output: "indexed 5 notes\n"}},
		{"memory/remember", "params", RememberParams{Fact: "the deploy key rotates monthly"}},
		{"memory/remember", "result", RememberResult{Action: "appended", Path: "memory/2026-06-25-deploy.md"}},
		{"memory/edit", "params", MemoryParams{
			Command: "rename",
			Path:    "memory/notes.md",
			Content: "full note body",
			OldStr:  "old text",
			NewStr:  "new text",
			Line:    3,
			Text:    "inserted line",
			Dest:    "memory/renamed.md",
		}},
		{"memory/edit", "result", MemoryResult{Result: "created memory/notes.md"}},
	}
}

// goldenPath maps a case to its checked-in file under rpc/testdata.
func goldenPath(c goldenCase) string {
	name := ""
	for _, r := range c.method {
		if r == '/' {
			name += "-"
			continue
		}
		name += string(r)
	}
	return filepath.Join("testdata", name+"."+c.kind+".json")
}

// marshalGolden produces the canonical bytes for a case: indented JSON with a
// trailing newline, so the files are diff-friendly and stable across runs.
func marshalGolden(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return append(raw, '\n')
}

// TestGoldenWireShapes pins each seam method's Params and Result against a
// checked-in JSON file. A field rename, a struct-tag change, or an added or
// removed field shifts the marshaled bytes and fails here, forcing the wire
// change to be acknowledged by regenerating the golden with -update.
func TestGoldenWireShapes(t *testing.T) {
	for _, c := range goldenCases() {
		c := c
		t.Run(c.method+"/"+c.kind, func(t *testing.T) {
			path := goldenPath(c)
			got := marshalGolden(t, c.value)

			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir testdata: %v", err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write golden %s: %v", path, err)
				}
				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s (run `go test ./rpc/ -run Golden -update` to create): %v", path, err)
			}
			if string(got) != string(want) {
				t.Fatalf("wire shape for %s drifted from %s:\n got:\n%s\nwant:\n%s", c.method+" "+c.kind, path, got, want)
			}
		})
	}
}
