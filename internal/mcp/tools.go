package mcp

import (
	"fmt"
	"strings"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/rpc"
)

type queryArgs struct {
	Query string `json:"query" jsonschema:"the search query"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results, default 10"`
}

type getNoteArgs struct {
	Path string `json:"path" jsonschema:"vault-relative path to the note, e.g. notes/foo.md"`
}

type bundleArgs struct {
	Task   string `json:"task" jsonschema:"the task description to assemble context for"`
	Budget int    `json:"budget,omitempty" jsonschema:"approximate token budget, default 4000"`
}

type rememberArgs struct {
	Fact string `json:"fact" jsonschema:"the fact to store in the vault"`
}

type digestArgs struct {
	Since   string `json:"since,omitempty" jsonschema:"git SHA to diff from"`
	Advance bool   `json:"advance,omitempty" jsonschema:"advance the digest cursor to HEAD"`
}

type memoryArgs struct {
	Command string `json:"command" jsonschema:"one of: view, create, str_replace, insert, delete, rename"`
	Path    string `json:"path" jsonschema:"vault-relative file path"`
	Content string `json:"content,omitempty" jsonschema:"file content (create)"`
	OldStr  string `json:"old_str,omitempty" jsonschema:"text to replace (str_replace)"`
	NewStr  string `json:"new_str,omitempty" jsonschema:"replacement text (str_replace)"`
	Line    int    `json:"line,omitempty" jsonschema:"0-based line index (insert)"`
	Text    string `json:"text,omitempty" jsonschema:"text to insert (insert)"`
	Dest    string `json:"dest,omitempty" jsonschema:"destination path (rename)"`
}

type memoryResult struct {
	Result string `json:"result"`
}

// mountsResult wraps the mounts list so the MCP tool output schema is an object.
type mountsResult struct {
	Mounts []service.MountInfo `json:"mounts"`
}

// collectionsResult wraps the collections list so the MCP tool output schema is an object.
type collectionsResult struct {
	Collections []service.CollectionInfo `json:"collections"`
}

type listRecordsArgs struct {
	Collection string   `json:"collection" jsonschema:"the collection name to list records from"`
	Where      []string `json:"where,omitempty" jsonschema:"frontmatter filters as field:op:value, op is one of eq, ne, gt, gte, lt, lte, contains"`
	Sort       string   `json:"sort,omitempty" jsonschema:"sort field (a frontmatter key, or path / updated_at), prefix with - for descending"`
	Limit      int      `json:"limit,omitempty" jsonschema:"maximum number of records, 0 means no limit"`
	Offset     int      `json:"offset,omitempty" jsonschema:"number of records to skip for pagination"`
}

type getRecordArgs struct {
	Path string `json:"path" jsonschema:"vault-relative path to the record note, e.g. jobs/acme.md"`
}

type createRecordArgs struct {
	Collection string         `json:"collection" jsonschema:"the collection to create the record in"`
	Fields     map[string]any `json:"fields" jsonschema:"frontmatter columns for the record, validated against the collection schema"`
	Body       string         `json:"body,omitempty" jsonschema:"markdown body of the record note"`
}

type patchRecordArgs struct {
	Path   string         `json:"path" jsonschema:"vault-relative path to the record to patch"`
	Fields map[string]any `json:"fields,omitempty" jsonschema:"frontmatter columns to merge, a null value deletes that key"`
	Body   *string        `json:"body,omitempty" jsonschema:"new markdown body, omit to leave the body unchanged"`
}

// parsePredicates turns "field:op:value" strings into rpc.Predicate values for
// the record/list method. Only the first two colons separate the parts, so a
// value may itself contain colons. An empty field or op is rejected.
func parsePredicates(where []string) ([]rpc.Predicate, error) {
	if len(where) == 0 {
		return nil, nil
	}
	preds := make([]rpc.Predicate, 0, len(where))
	for _, w := range where {
		parts := strings.SplitN(w, ":", 3)
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid where clause %q: want field:op:value", w)
		}
		preds = append(preds, rpc.Predicate{Field: parts[0], Op: parts[1], Value: parts[2]})
	}
	return preds, nil
}
