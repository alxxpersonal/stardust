// Package rpc is the typed JSON-RPC 2.0 contract shared by the stardust server
// and its clients (exo-jobs and external callers). Every method's Params and
// Result are defined here as compiler-checked structs whose wire field names
// mirror the existing HTTP/JSON surface (internal/api/api.go, sdk/client.go), so
// the seam is type-safe across the repo boundary rather than map[string]any.
package rpc

// Record is a single note in a collection: its vault-relative path, title,
// decoded frontmatter columns, and (when read whole) its markdown body. Field
// names mirror service.Record and the REST wire shape.
type Record struct {
	Path        string         `json:"path"`
	Title       string         `json:"title"`
	Frontmatter map[string]any `json:"frontmatter"`
	Body        string         `json:"body"`
}

// Predicate is a single frontmatter filter for record/list: Op is one of eq, ne,
// gt, gte, lt, lte, contains. Field names mirror index.Predicate.
type Predicate struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// RecordList is a page of records for a collection, echoing the resolved folder
// the records were scoped to. Field names mirror service.RecordList.
type RecordList struct {
	Collection string   `json:"collection"`
	Folder     string   `json:"folder"`
	Records    []Record `json:"records"`
}
