package rpc

import (
	"encoding/json"
	"reflect"
	"testing"
)

// roundTrip marshals v to JSON, unmarshals into a fresh value of the same type,
// and fails if the result differs. It pins that a type round-trips through the
// wire without losing or renaming fields.
func roundTrip[T any](t *testing.T, v T) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var back T
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal %T: %v", v, err)
	}
	if !reflect.DeepEqual(v, back) {
		t.Fatalf("round-trip mismatch for %T:\n in:  %#v\n out: %#v", v, v, back)
	}
}

// assertFields fails if the marshaled JSON object keys do not exactly match want.
// It pins the wire field names so a struct-tag rename breaks the test.
func assertFields(t *testing.T, v any, want []string) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal %T into object: %v", v, err)
	}
	got := make(map[string]bool, len(obj))
	for k := range obj {
		got[k] = true
	}
	if len(got) != len(want) {
		t.Fatalf("%T field count: got %d (%v), want %d (%v)", v, len(got), keys(obj), len(want), want)
	}
	for _, k := range want {
		if !got[k] {
			t.Fatalf("%T missing wire field %q (got %v)", v, k, keys(obj))
		}
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestContractVersion(t *testing.T) {
	if ContractVersion == "" {
		t.Fatal("ContractVersion must be a non-empty string")
	}
}

func TestRecordWireShape(t *testing.T) {
	rec := Record{
		Path:        "notes/a.md",
		Title:       "A",
		Frontmatter: map[string]any{"status": "active"},
		Body:        "hello",
	}
	roundTrip(t, rec)
	assertFields(t, rec, []string{"path", "title", "frontmatter", "body"})
}

func TestStatusResultWireShape(t *testing.T) {
	res := StatusResult{
		Root:        "/vault",
		Notes:       12,
		Chunks:      34,
		LastIndexed: "abc123",
		EmbedModel:  "model-x",
		Vectors:     true,
		Reranker:    false,
	}
	roundTrip(t, res)
	assertFields(t, res, []string{
		"root", "notes", "chunks", "last_indexed_sha", "embed_model", "vectors", "reranker",
	})
}

func TestCreateRecordParamsWireShape(t *testing.T) {
	p := CreateRecordParams{
		Collection: "tasks",
		Fields:     map[string]any{"title": "ship it"},
		Body:       "do the thing",
	}
	roundTrip(t, p)
	assertFields(t, p, []string{"collection", "fields", "body"})
}

func TestRecordParamsWireShape(t *testing.T) {
	p := RecordParams{Path: "notes/a.md"}
	roundTrip(t, p)
	assertFields(t, p, []string{"path"})
}

func TestListRecordsParamsWireShape(t *testing.T) {
	p := ListRecordsParams{
		Collection: "tasks",
		Filter:     []Predicate{{Field: "status", Op: "eq", Value: "active"}},
		Sort:       "-updated_at",
		Limit:      10,
		Offset:     5,
	}
	roundTrip(t, p)
	assertFields(t, p, []string{"collection", "filter", "sort", "limit", "offset"})
}

func TestPredicateWireShape(t *testing.T) {
	pr := Predicate{Field: "status", Op: "eq", Value: "active"}
	roundTrip(t, pr)
	assertFields(t, pr, []string{"field", "op", "value"})
}

func TestRecordListWireShape(t *testing.T) {
	rl := RecordList{
		Collection: "tasks",
		Folder:     "20-Active/Tasks",
		Records:    []Record{{Path: "p", Title: "t", Frontmatter: map[string]any{}, Body: "b"}},
	}
	roundTrip(t, rl)
	assertFields(t, rl, []string{"collection", "folder", "records"})
}

func TestPatchRecordParamsWireShape(t *testing.T) {
	body := "new body"
	p := PatchRecordParams{
		Path:   "notes/a.md",
		Fields: map[string]any{"status": "done"},
		Body:   &body,
	}
	roundTrip(t, p)
	assertFields(t, p, []string{"path", "fields", "body"})
}

func TestDeleteResultWireShape(t *testing.T) {
	res := DeleteResult{Path: "notes/a.md", Status: "deleted"}
	roundTrip(t, res)
	assertFields(t, res, []string{"path", "status"})
}

// --- full operation set ---

func TestQueryParamsWireShape(t *testing.T) {
	p := QueryParams{Query: "json-rpc transport", Limit: 10}
	roundTrip(t, p)
	assertFields(t, p, []string{"query", "limit"})
}

func TestHitWireShape(t *testing.T) {
	h := Hit{Path: "notes/a.md", Title: "A", Heading: "Intro", Snippet: "...", Score: 1.5}
	roundTrip(t, h)
	assertFields(t, h, []string{"path", "title", "heading", "snippet", "score"})
}

func TestQueryResultWireShape(t *testing.T) {
	res := QueryResult{
		Query: "q",
		Mode:  "hybrid + rerank",
		Hits:  []Hit{{Path: "p", Title: "t", Heading: "h", Snippet: "s", Score: 0.9}},
	}
	roundTrip(t, res)
	assertFields(t, res, []string{"query", "mode", "hits"})
}

func TestBundleParamsWireShape(t *testing.T) {
	p := BundleParams{Task: "ship the contract", Budget: 4000}
	roundTrip(t, p)
	assertFields(t, p, []string{"task", "budget"})
}

func TestBundleItemWireShape(t *testing.T) {
	it := BundleItem{Path: "notes/a.md", Title: "A", Snippet: "...", Score: 2.1}
	roundTrip(t, it)
	assertFields(t, it, []string{"path", "title", "snippet", "score"})
}

func TestBundleResultWireShape(t *testing.T) {
	res := BundleResult{
		Task:     "t",
		Items:    []BundleItem{{Path: "p", Title: "ti", Snippet: "s", Score: 1.0}},
		Markdown: "# bundle",
		Tokens:   42,
	}
	roundTrip(t, res)
	assertFields(t, res, []string{"task", "items", "markdown", "tokens_estimate"})
}

func TestBrokenLinkWireShape(t *testing.T) {
	bl := BrokenLink{From: "notes/a.md", Target: "missing"}
	roundTrip(t, bl)
	assertFields(t, bl, []string{"from", "target"})
}

func TestPageRankEntryWireShape(t *testing.T) {
	pr := PageRankEntry{Path: "notes/a.md", Title: "A", Score: 0.25}
	roundTrip(t, pr)
	assertFields(t, pr, []string{"path", "title", "score"})
}

func TestGraphResultWireShape(t *testing.T) {
	res := GraphResult{
		Notes:    3,
		Links:    4,
		Orphans:  []string{"notes/x.md"},
		Broken:   []BrokenLink{{From: "a", Target: "b"}},
		PageRank: []PageRankEntry{{Path: "p", Title: "t", Score: 0.5}},
	}
	roundTrip(t, res)
	assertFields(t, res, []string{"notes", "links", "orphans", "broken", "pagerank"})
}

func TestDigestParamsWireShape(t *testing.T) {
	p := DigestParams{Since: "abc123", Advance: true}
	roundTrip(t, p)
	assertFields(t, p, []string{"since", "advance"})
}

func TestDigestResultWireShape(t *testing.T) {
	res := DigestResult{Since: "abc", Head: "def", Changed: 5, Markdown: "# digest"}
	roundTrip(t, res)
	assertFields(t, res, []string{"since", "head", "changed", "markdown"})
}

func TestIssueWireShape(t *testing.T) {
	is := Issue{Severity: "error", Kind: "broken-link", Path: "notes/a.md", Detail: "[[x]] resolves to no note"}
	roundTrip(t, is)
	assertFields(t, is, []string{"severity", "kind", "path", "detail"})
}

func TestCheckResultWireShape(t *testing.T) {
	res := CheckResult{
		Issues:   []Issue{{Severity: "warn", Kind: "orphan", Path: "p", Detail: "no links"}},
		Errors:   1,
		Warnings: 2,
		Markdown: "# check",
	}
	roundTrip(t, res)
	assertFields(t, res, []string{"issues", "errors", "warnings", "markdown"})
}

func TestNoteParamsWireShape(t *testing.T) {
	p := NoteParams{Path: "notes/a.md"}
	roundTrip(t, p)
	assertFields(t, p, []string{"path"})
}

func TestLinkTargetWireShape(t *testing.T) {
	lt := LinkTarget{Link: "some note", Path: "notes/some-note.md"}
	roundTrip(t, lt)
	assertFields(t, lt, []string{"link", "path"})
}

func TestNoteWireShape(t *testing.T) {
	n := Note{
		Path:        "notes/a.md",
		Title:       "A",
		Tags:        []string{"x"},
		Links:       []string{"b"},
		LinkTargets: []LinkTarget{{Link: "b", Path: "notes/b.md"}},
		Frontmatter: map[string]any{"status": "active"},
		Body:        "hello",
	}
	roundTrip(t, n)
	assertFields(t, n, []string{"path", "title", "tags", "links", "link_targets", "frontmatter", "body"})
}

func TestFieldWireShape(t *testing.T) {
	f := Field{Name: "status", Type: "enum", Required: true, Enum: []string{"active", "done"}, Default: "active"}
	roundTrip(t, f)
	assertFields(t, f, []string{"name", "type", "required", "enum", "default"})
}

func TestCollectionWireShape(t *testing.T) {
	c := Collection{
		Name:        "tasks",
		Path:        "20-Active/Tasks",
		Description: "open work",
		Fields:      []Field{{Name: "status", Type: "string"}},
		Records:     7,
	}
	roundTrip(t, c)
	assertFields(t, c, []string{"name", "path", "description", "fields", "records"})
}

func TestCollectionParamsWireShape(t *testing.T) {
	p := CollectionParams{Name: "tasks"}
	roundTrip(t, p)
	assertFields(t, p, []string{"name"})
}

func TestMountWireShape(t *testing.T) {
	m := Mount{Name: "obsidian", Kind: "mcp", Target: "/usr/bin/foo", Args: []string{"--stdio"}, Tool: "search"}
	roundTrip(t, m)
	assertFields(t, m, []string{"name", "kind", "target", "args", "tool"})
}

func TestIndexParamsWireShape(t *testing.T) {
	p := IndexParams{Since: "abc123"}
	roundTrip(t, p)
	assertFields(t, p, []string{"since"})
}

func TestIndexStatsWireShape(t *testing.T) {
	res := IndexStats{Indexed: 3, Skipped: 1, Deleted: 0, Vectors: true}
	roundTrip(t, res)
	assertFields(t, res, []string{"indexed", "skipped", "deleted", "vectors"})
}

func TestArchiveParamsWireShape(t *testing.T) {
	p := ArchiveParams{Dest: ".stardust/archives"}
	roundTrip(t, p)
	assertFields(t, p, []string{"dest"})
}

func TestArchiveResultWireShape(t *testing.T) {
	res := ArchiveResult{Path: ".stardust/archives/2026-06-25.bundle"}
	roundTrip(t, res)
	assertFields(t, res, []string{"path"})
}

func TestCronJobWireShape(t *testing.T) {
	job := CronJob{
		Name:     "nightly-index",
		Schedule: "0 3 * * *",
		On:       "commit",
		Kind:     "command",
		Command:  "index",
	}
	roundTrip(t, job)
	assertFields(t, job, []string{"name", "schedule", "on", "kind", "command"})
}

func TestCronRunParamsWireShape(t *testing.T) {
	p := CronRunParams{Name: "nightly-index"}
	roundTrip(t, p)
	assertFields(t, p, []string{"name"})
}

func TestCronRunResultWireShape(t *testing.T) {
	res := CronRunResult{Output: "indexed 12 notes\n"}
	roundTrip(t, res)
	assertFields(t, res, []string{"output"})
}
