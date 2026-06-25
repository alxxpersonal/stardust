package rpc

// ContractVersion identifies the wire contract this package defines. A client
// and server that disagree on this value may have drifted; the status handshake
// surfaces the server's version so a stale client can detect a mismatch.
const ContractVersion = "1"

// --- status ---

// StatusResult reports index health. It mirrors service.Status and the GET
// /status wire shape. The status method takes no params.
type StatusResult struct {
	Root        string `json:"root"`
	Notes       int    `json:"notes"`
	Chunks      int    `json:"chunks"`
	LastIndexed string `json:"last_indexed_sha"`
	EmbedModel  string `json:"embed_model"`
	Vectors     bool   `json:"vectors"`
	Reranker    bool   `json:"reranker"`
}

// --- record/create ---

// CreateRecordParams creates a record in a collection. Fields are validated
// against the collection schema; Body is the markdown body of the new note. It
// mirrors the POST /records request body. Result is a Record.
type CreateRecordParams struct {
	Collection string         `json:"collection"`
	Fields     map[string]any `json:"fields"`
	Body       string         `json:"body"`
}

// --- record/get and record/delete ---

// RecordParams addresses a single record by its vault-relative path. It is the
// params for both record/get (Result Record) and record/delete (Result
// DeleteResult), mirroring the GET /record and DELETE /record path query.
type RecordParams struct {
	Path string `json:"path"`
}

// --- record/list ---

// ListRecordsParams lists records in a collection, filtered by frontmatter
// predicates and ordered by Sort (a frontmatter field, or "path" / "updated_at",
// with an optional leading "-" for descending). A non-positive Limit means no
// limit; Offset paginates. It mirrors the GET /records query. Result is a
// RecordList.
type ListRecordsParams struct {
	Collection string      `json:"collection"`
	Filter     []Predicate `json:"filter"`
	Sort       string      `json:"sort"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
}

// --- record/patch ---

// PatchRecordParams updates a record by path: Fields are merged into its
// frontmatter (a nil value deletes a key) and Body, when non-nil, replaces the
// markdown body. It mirrors the PATCH /record path query plus body. Result is a
// Record.
type PatchRecordParams struct {
	Path   string         `json:"path"`
	Fields map[string]any `json:"fields"`
	Body   *string        `json:"body"`
}

// DeleteResult is the result of record/delete: the path that was removed and a
// terminal status. It mirrors the DELETE /record response body.
type DeleteResult struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}
