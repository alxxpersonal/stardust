package rpc

import "sort"

// OpenRPCVersion is the OpenRPC specification version the discovery document
// declares. It is the schema the rpc.discover result conforms to.
const OpenRPCVersion = "1.2.6"

// OpenRPCDoc is a deliberately light OpenRPC document: the spec header, the info
// block, and a flat method list of name plus a one-line summary. It is the result
// of rpc.discover and the on-disk docs/openrpc.json. Per-method param and result
// JSON Schema is intentionally omitted - the typed contract package (this
// package) is the schema source of truth; the document is for method discovery,
// not full schema validation.
type OpenRPCDoc struct {
	OpenRPC string          `json:"openrpc"`
	Info    OpenRPCInfo     `json:"info"`
	Methods []OpenRPCMethod `json:"methods"`
}

// OpenRPCInfo is the document's info header: the API title and the contract
// version it pins. Version mirrors ContractVersion so a discovery reader can
// detect contract drift the same way the status handshake does.
type OpenRPCInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// OpenRPCMethod is one entry in the document's method list: the canonical method
// name and a one-line summary. Params and result schema are intentionally
// omitted to keep the document light.
type OpenRPCMethod struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// methodSummaries is the canonical one-line summary per method name. It is the
// single source for both rpc.discover and docs/openrpc.json. A method present in
// the registry but missing here is summarized as an empty string, which the
// OpenRPC conformance test treats as drift, so this map MUST track the registry.
var methodSummaries = map[string]string{
	"status":          "Report index health.",
	"record/create":   "Create a record in a collection from validated fields and a markdown body.",
	"record/get":      "Read a single record by its vault-relative path.",
	"record/list":     "List records in a collection, filtered and ordered.",
	"record/patch":    "Merge fields into a record's frontmatter and optionally replace its body.",
	"record/delete":   "Archive a record by path and report the terminal status.",
	"query":           "Run hybrid retrieval and return the ranked hits.",
	"bundle":          "Assemble a token-budgeted context bundle for a task.",
	"graph":           "Derive the link graph and summarize it.",
	"digest":          "Summarize vault activity since a commit cursor.",
	"check":           "Run the vault integrity check.",
	"note/get":        "Read the parsed note at a vault-relative path.",
	"collection/list": "List every configured collection with a live record count.",
	"collection/get":  "Read a single collection by name.",
	"mount/list":      "List the configured mounts.",
	"index/run":       "Incrementally index the vault.",
	"index/rebuild":   "Clear the derived cache and reindex from scratch.",
	"archive":         "Snapshot the vault's git history into a destination.",
	"cron/list":       "List the configured cron jobs.",
	"cron/run":        "Run a cron job by name and return its buffered output.",
	"memory/remember": "Store a fact add-only, appending to the nearest note or creating a dated note.",
	"memory/edit":     "Apply a memory verb to a vault file and reindex the change.",
	"rpc.discover":    "Return this OpenRPC discovery document.",
}

// BuildOpenRPC assembles an OpenRPCDoc from the supplied method names, sorted for
// a stable wire shape. The summary per method comes from the canonical
// methodSummaries map; an unknown name yields an empty summary so doc drift
// against the registry is detectable. The same builder backs rpc.discover and the
// docs/openrpc.json generator, so the runtime document and the committed file stay
// identical.
func BuildOpenRPC(names []string) OpenRPCDoc {
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)

	methods := make([]OpenRPCMethod, 0, len(sorted))
	for _, name := range sorted {
		methods = append(methods, OpenRPCMethod{Name: name, Summary: methodSummaries[name]})
	}
	return OpenRPCDoc{
		OpenRPC: OpenRPCVersion,
		Info:    OpenRPCInfo{Title: "Stardust JSON-RPC contract", Version: ContractVersion},
		Methods: methods,
	}
}
