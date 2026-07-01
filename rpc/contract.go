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

// --- query ---

// QueryParams runs hybrid retrieval for Query, capped at Limit hits (a
// non-positive Limit lets the server pick its default). It mirrors the GET /query
// q + limit params. Result is a QueryResult. Field names mirror service.Query's
// arguments.
type QueryParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// Hit is a single retrieval result, collapsed to one row per note (the best
// matching chunk stands in for its parent note). Field names mirror index.Hit.
type Hit struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Heading string  `json:"heading"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// QueryResult is the outcome of a search: the echoed query, the Mode recording
// which retrieval stages ran (keyword, hybrid, with optional rerank), and the
// ranked Hits. Field names mirror service.QueryResult.
type QueryResult struct {
	Query string `json:"query"`
	Mode  string `json:"mode"`
	Hits  []Hit  `json:"hits"`
}

// --- bundle ---

// BundleParams assembles a token-budgeted context bundle for Task. A non-positive
// Budget lets the server pick its default. It mirrors the GET /bundle task +
// budget params. Result is a BundleResult. Field names mirror service.Bundle's
// arguments.
type BundleParams struct {
	Task   string `json:"task"`
	Budget int    `json:"budget"`
}

// BundleItem is a note chosen for a context bundle. Field names mirror
// service.BundleItem.
type BundleItem struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// BundleResult is an assembled, token-budgeted context bundle for a task: the
// chosen Items, the packed Markdown, and an estimated token count. Field names
// mirror service.BundleResult.
type BundleResult struct {
	Task     string       `json:"task"`
	Items    []BundleItem `json:"items"`
	Markdown string       `json:"markdown"`
	Tokens   int          `json:"tokens_estimate"`
}

// --- graph ---

// BrokenLink is a wikilink whose target resolves to no note. Field names mirror
// graph.BrokenLink.
type BrokenLink struct {
	From   string `json:"from"`
	Target string `json:"target"`
}

// PageRankEntry is one note's centrality score in the link graph. Field names
// mirror graph.PageRankEntry.
type PageRankEntry struct {
	Path  string  `json:"path"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

// GraphResult summarizes the derived link graph: note and link counts, orphans,
// broken links, and the top notes by centrality. The graph method takes no
// params. Field names mirror service.GraphReport.
type GraphResult struct {
	Notes    int             `json:"notes"`
	Links    int             `json:"links"`
	Orphans  []string        `json:"orphans"`
	Broken   []BrokenLink    `json:"broken"`
	PageRank []PageRankEntry `json:"pagerank"`
}

// --- digest ---

// DigestParams summarizes vault activity since the Since commit cursor (empty
// uses the stored cursor); with Advance set the server moves the cursor to HEAD.
// It mirrors the GET /digest since + advance params. Result is a DigestResult.
// Field names mirror service.Digest's arguments.
type DigestParams struct {
	Since   string `json:"since"`
	Advance bool   `json:"advance"`
}

// DigestResult is a summary of recent vault activity: the resolved Since cursor,
// the current Head, the count of Changed notes, and the rendered Markdown. Field
// names mirror service.DigestResult.
type DigestResult struct {
	Since    string `json:"since"`
	Head     string `json:"head"`
	Changed  int    `json:"changed"`
	Markdown string `json:"markdown"`
}

// --- check ---

// Issue is one vault-integrity problem. Severity is "error" or "warn". Field
// names mirror service.Issue.
type Issue struct {
	Severity string `json:"severity"`
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Detail   string `json:"detail"`
}

// CheckResult is the outcome of a vault integrity check: the Issues found, their
// error and warning counts, and the rendered Markdown. The check method takes no
// params. Field names mirror service.CheckResult.
type CheckResult struct {
	Issues   []Issue `json:"issues"`
	Errors   int     `json:"errors"`
	Warnings int     `json:"warnings"`
	Markdown string  `json:"markdown"`
}

// --- note/get ---

// NoteParams reads the parsed note at a vault-relative Path. It mirrors the GET
// /note path param. Result is a Note. Field name mirrors service.GetNote's
// argument.
type NoteParams struct {
	Path string `json:"path"`
}

// LinkTarget pairs a note's normalized wikilink with the vault-relative path it
// resolves to. Path is empty when the link points at no existing note (broken).
// Field names mirror service.LinkTarget.
type LinkTarget struct {
	Link string `json:"link"`
	Path string `json:"path"`
}

// Note is a parsed note returned by note/get: its path, title, tags, raw links,
// resolved link targets, decoded frontmatter, and markdown body. Field names
// mirror service.Note.
type Note struct {
	Path        string         `json:"path"`
	Title       string         `json:"title"`
	Tags        []string       `json:"tags"`
	Links       []string       `json:"links"`
	LinkTargets []LinkTarget   `json:"link_targets"`
	Frontmatter map[string]any `json:"frontmatter"`
	Body        string         `json:"body"`
}

// --- collection/list and collection/get ---

// Field is one typed column of a collection schema. Type is one of string,
// number, bool, date, enum, tags, or ref; Enum lists allowed values for an enum
// field; Default supplies a value when a record omits the field. Field names
// mirror collections.Field.
type Field struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Enum     []string `json:"enum,omitempty"`
	Default  any      `json:"default,omitempty"`
}

// Collection describes one collection: its name, vault folder, description,
// typed schema fields, and live indexed record count. It is the result of
// collection/get and the element type of collection/list. Field names mirror
// service.CollectionInfo.
type Collection struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
	Records     int     `json:"records"`
}

// CollectionParams addresses a single collection by Name. It mirrors the GET
// /collection name param. Result is a Collection. Field name mirrors
// service.GetCollection's argument.
type CollectionParams struct {
	Name string `json:"name"`
}

// --- mount/list ---

// Mount describes one configured mount as read from its config.toml. Every mount
// is an MCP-server connector, so Kind is "mcp" and Target is the executable
// launched to reach the source. It is the element type of mount/list. Field
// names mirror service.MountInfo.
type Mount struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Target string   `json:"target"`
	Args   []string `json:"args,omitempty"`
	Tool   string   `json:"tool"`
}

// --- index/run, index/rebuild, archive ---

// IndexParams incrementally indexes the vault; a non-empty Since uses the
// git-diff fast path, otherwise a full scan. It mirrors the POST /index since
// param. Result is IndexStats. Field name mirrors service.Index's argument.
type IndexParams struct {
	Since string `json:"since"`
}

// IndexStats summarizes an indexing run: notes indexed, skipped, and deleted, and
// whether vectors were written. It is the result of index/run and index/rebuild
// (which takes no params). Field names mirror service.IndexStats.
type IndexStats struct {
	Indexed int  `json:"indexed"`
	Skipped int  `json:"skipped"`
	Deleted int  `json:"deleted"`
	Vectors bool `json:"vectors"`
}

// ArchiveParams snapshots the vault's git history into Dest (empty uses the
// default .stardust/archives). It mirrors the POST /archive dest param. Result is
// an ArchiveResult. Field name mirrors service.Archive's argument.
type ArchiveParams struct {
	Dest string `json:"dest"`
}

// ArchiveResult reports the path the archive bundle was written to. It mirrors
// the POST /archive response body.
type ArchiveResult struct {
	Path string `json:"path"`
}

// --- cron/list and cron/run ---

// CronJob is one configured cron job flattened for the wire: its name, the
// trigger (Schedule cron expression or On event with optional Paths globs), and
// the run kind plus its kind-specific fields. Field names mirror the displayed
// cron.Job surface (cron.Trigger, cron.Run).
type CronJob struct {
	Name     string   `json:"name"`
	Schedule string   `json:"schedule,omitempty"`
	On       string   `json:"on,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Kind     string   `json:"kind"`
	Command  string   `json:"command,omitempty"`
	Exec     string   `json:"exec,omitempty"`
	Prompt   string   `json:"prompt,omitempty"`
	Model    string   `json:"model,omitempty"`
}

// CronRunParams runs a cron job by Name. The server supplies the stardust binary
// path; the streamed run output is buffered into CronRunResult.Output. It mirrors
// the POST /cron/run job param. Field name mirrors service.CronRun's name
// argument.
type CronRunParams struct {
	Name string `json:"name"`
}

// CronRunResult holds the buffered output of a cron run. service.CronRun streams
// to an io.Writer; as a JSON-RPC method the server buffers that stream into this
// string. It mirrors the POST /cron/run response body.
type CronRunResult struct {
	Output string `json:"output"`
}

// --- memory/remember ---

// RememberParams stores Fact in the vault add-only. Field name mirrors the
// remember MCP tool's input and service.Remember's fact argument. Result is a
// RememberResult.
type RememberParams struct {
	Fact string `json:"fact"`
}

// RememberResult records where a remembered fact landed: Action is "appended"
// (merged into the nearest existing note) or "created" (a fresh dated note under
// memory/), and Path is the note that was touched. Field names mirror
// service.RememberResult.
type RememberResult struct {
	Action string `json:"action"`
	Path   string `json:"path"`
}

// --- memory/edit ---

// MemoryParams applies a single memory verb to a vault file. Command is one of
// view, create, str_replace, insert, delete, rename; the remaining fields carry
// that verb's operands: Content for create, OldStr and NewStr for str_replace,
// Line and Text for insert, and Dest for rename. It mirrors service.MemoryOp and
// the memory MCP tool's input schema. Result is a MemoryResult.
type MemoryParams struct {
	Command string `json:"command"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	OldStr  string `json:"old_str,omitempty"`
	NewStr  string `json:"new_str,omitempty"`
	Line    int    `json:"line,omitempty"`
	Text    string `json:"text,omitempty"`
	Dest    string `json:"dest,omitempty"`
}

// MemoryResult is the human-readable outcome line of a memory verb, e.g.
// "created memory/a.md" or "view" content. It mirrors the memory MCP tool's
// result wrapper.
type MemoryResult struct {
	Result string `json:"result"`
}
