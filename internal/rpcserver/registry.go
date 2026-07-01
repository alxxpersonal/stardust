// Package rpcserver assembles the typed JSON-RPC method registry over the
// Stardust service core. It maps the canonical slash method names to typed
// handlers, each a thin caller of an internal/service method. jrpc2 owns request
// decoding, response encoding, id correlation, and the error band (ADR 0006); a
// handler returns a typed value and a plain error, nothing more.
package rpcserver

import (
	"bytes"
	"context"
	"os"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"

	"github.com/alxxpersonal/stardust/internal/cron"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/rpc"
)

// ServerOptions returns the canonical jrpc2 server options every transport mounts
// this registry with. DisableBuiltin lets the registry own the reserved rpc.*
// namespace so the rpc.discover method is dispatched to our handler rather than
// jrpc2's built-in introspection methods. A nil return would re-reserve rpc.* and
// shadow rpc.discover, so every mount (stdio, jhttp bridge, in-memory local) MUST
// pass this.
func ServerOptions() *jrpc2.ServerOptions {
	return &jrpc2.ServerOptions{DisableBuiltin: true}
}

// NewRegistry builds the handler.Map of canonical slash method names to typed
// handlers over svc. The same map backs every transport (stdio, HTTP, MCP) so
// the surfaces stay at structural parity. Alongside the twenty-two operation
// methods it registers the reserved rpc.discover method, whose result is an
// OpenRPC document built from this registry's own method set.
func NewRegistry(svc *service.Service) handler.Map {
	reg := handler.Map{
		"status":          handler.New(statusHandler(svc)),
		"record/create":   handler.New(createRecordHandler(svc)),
		"record/get":      handler.New(getRecordHandler(svc)),
		"record/list":     handler.New(listRecordsHandler(svc)),
		"record/patch":    handler.New(patchRecordHandler(svc)),
		"record/delete":   handler.New(deleteRecordHandler(svc)),
		"query":           handler.New(queryHandler(svc)),
		"bundle":          handler.New(bundleHandler(svc)),
		"graph":           handler.New(graphHandler(svc)),
		"digest":          handler.New(digestHandler(svc)),
		"check":           handler.New(checkHandler(svc)),
		"note/get":        handler.New(getNoteHandler(svc)),
		"collection/list": handler.New(listCollectionsHandler(svc)),
		"collection/get":  handler.New(getCollectionHandler(svc)),
		"mount/list":      handler.New(listMountsHandler(svc)),
		"index/run":       handler.New(indexHandler(svc)),
		"index/rebuild":   handler.New(rebuildHandler(svc)),
		"archive":         handler.New(archiveHandler(svc)),
		"cron/list":       handler.New(listCronHandler(svc)),
		"cron/run":        handler.New(runCronHandler(svc)),
		"memory/remember": handler.New(rememberHandler(svc)),
		"memory/edit":     handler.New(memoryEditHandler(svc)),
	}

	// rpc.discover is the OpenRPC discovery method (the 21st). Its document lists
	// the full method set, including rpc.discover itself, so the runtime document
	// names every callable method. handler.Map.Names is sorted; appending the
	// discover name keeps the built document stable regardless of map order.
	names := append(reg.Names(), "rpc.discover")
	reg["rpc.discover"] = handler.New(discoverHandler(names))
	return reg
}

// --- rpc.discover ---

// discoverHandler returns the OpenRPC discovery document for the registry's
// method set. The names are captured at registry-construction time, so the
// document always matches the methods this server exposes. The method takes no
// params.
func discoverHandler(names []string) func(context.Context) (rpc.OpenRPCDoc, error) {
	return func(_ context.Context) (rpc.OpenRPCDoc, error) {
		return rpc.BuildOpenRPC(names), nil
	}
}

// --- status ---

// statusHandler reports index health. The method takes no params.
func statusHandler(svc *service.Service) func(context.Context) (rpc.StatusResult, error) {
	return func(ctx context.Context) (rpc.StatusResult, error) {
		st, err := svc.Status(ctx)
		if err != nil {
			return rpc.StatusResult{}, err
		}
		return rpc.StatusResult{
			Root:        st.Root,
			Notes:       st.Notes,
			Chunks:      st.Chunks,
			LastIndexed: st.LastIndexed,
			EmbedModel:  st.EmbedModel,
			Vectors:     st.Vectors,
			Reranker:    st.Reranker,
		}, nil
	}
}

// --- record/create ---

// createRecordHandler validates fields against the collection schema and writes
// a new note, returning the created record.
func createRecordHandler(svc *service.Service) func(context.Context, rpc.CreateRecordParams) (rpc.Record, error) {
	return func(ctx context.Context, p rpc.CreateRecordParams) (rpc.Record, error) {
		rec, err := svc.CreateRecord(ctx, p.Collection, p.Fields, p.Body)
		if err != nil {
			return rpc.Record{}, domainError(err)
		}
		return toRecord(rec), nil
	}
}

// --- record/get ---

// getRecordHandler reads a single record by its vault-relative path.
func getRecordHandler(svc *service.Service) func(context.Context, rpc.RecordParams) (rpc.Record, error) {
	return func(ctx context.Context, p rpc.RecordParams) (rpc.Record, error) {
		rec, err := svc.GetRecord(ctx, p.Path)
		if err != nil {
			return rpc.Record{}, err
		}
		return toRecord(rec), nil
	}
}

// --- record/list ---

// listRecordsHandler lists records in a collection, filtered and ordered by the
// supplied params.
func listRecordsHandler(svc *service.Service) func(context.Context, rpc.ListRecordsParams) (rpc.RecordList, error) {
	return func(ctx context.Context, p rpc.ListRecordsParams) (rpc.RecordList, error) {
		list, err := svc.ListRecords(ctx, p.Collection, toPredicates(p.Filter), p.Sort, p.Limit, p.Offset)
		if err != nil {
			return rpc.RecordList{}, err
		}
		return toRecordList(list), nil
	}
}

// --- record/patch ---

// patchRecordHandler merges fields into a record's frontmatter, optionally
// replaces its body, and returns the updated record.
func patchRecordHandler(svc *service.Service) func(context.Context, rpc.PatchRecordParams) (rpc.Record, error) {
	return func(ctx context.Context, p rpc.PatchRecordParams) (rpc.Record, error) {
		rec, err := svc.PatchRecord(ctx, p.Path, p.Fields, p.Body)
		if err != nil {
			return rpc.Record{}, domainError(err)
		}
		return toRecord(rec), nil
	}
}

// --- record/delete ---

// deleteRecordHandler archives a record by path and reports the terminal status,
// mirroring the DELETE /record wire shape.
func deleteRecordHandler(svc *service.Service) func(context.Context, rpc.RecordParams) (rpc.DeleteResult, error) {
	return func(ctx context.Context, p rpc.RecordParams) (rpc.DeleteResult, error) {
		if err := svc.ArchiveRecord(ctx, p.Path); err != nil {
			return rpc.DeleteResult{}, err
		}
		return rpc.DeleteResult{Path: p.Path, Status: "deleted"}, nil
	}
}

// --- query ---

// queryHandler runs hybrid retrieval and returns the ranked hits.
func queryHandler(svc *service.Service) func(context.Context, rpc.QueryParams) (rpc.QueryResult, error) {
	return func(ctx context.Context, p rpc.QueryParams) (rpc.QueryResult, error) {
		res, err := svc.Query(ctx, p.Query, p.Limit)
		if err != nil {
			return rpc.QueryResult{}, err
		}
		return toQueryResult(res), nil
	}
}

// --- bundle ---

// bundleHandler assembles a token-budgeted context bundle for a task.
func bundleHandler(svc *service.Service) func(context.Context, rpc.BundleParams) (rpc.BundleResult, error) {
	return func(ctx context.Context, p rpc.BundleParams) (rpc.BundleResult, error) {
		res, err := svc.Bundle(ctx, p.Task, p.Budget)
		if err != nil {
			return rpc.BundleResult{}, err
		}
		return toBundleResult(res), nil
	}
}

// --- graph ---

// graphHandler derives the link graph and summarizes it. The method takes no
// params.
func graphHandler(svc *service.Service) func(context.Context) (rpc.GraphResult, error) {
	return func(ctx context.Context) (rpc.GraphResult, error) {
		rep, err := svc.Graph(ctx)
		if err != nil {
			return rpc.GraphResult{}, err
		}
		return toGraphResult(rep), nil
	}
}

// --- digest ---

// digestHandler summarizes vault activity since a commit cursor, optionally
// advancing the stored cursor to HEAD.
func digestHandler(svc *service.Service) func(context.Context, rpc.DigestParams) (rpc.DigestResult, error) {
	return func(ctx context.Context, p rpc.DigestParams) (rpc.DigestResult, error) {
		res, err := svc.Digest(ctx, p.Since, p.Advance)
		if err != nil {
			return rpc.DigestResult{}, err
		}
		return rpc.DigestResult{
			Since:    res.Since,
			Head:     res.Head,
			Changed:  res.Changed,
			Markdown: res.Markdown,
		}, nil
	}
}

// --- check ---

// checkHandler runs the vault integrity check. The method takes no params.
func checkHandler(svc *service.Service) func(context.Context) (rpc.CheckResult, error) {
	return func(ctx context.Context) (rpc.CheckResult, error) {
		res, err := svc.Check(ctx)
		if err != nil {
			return rpc.CheckResult{}, err
		}
		return toCheckResult(res), nil
	}
}

// --- note/get ---

// getNoteHandler reads the parsed note at a vault-relative path.
func getNoteHandler(svc *service.Service) func(context.Context, rpc.NoteParams) (rpc.Note, error) {
	return func(ctx context.Context, p rpc.NoteParams) (rpc.Note, error) {
		n, err := svc.GetNote(ctx, p.Path)
		if err != nil {
			return rpc.Note{}, err
		}
		return toNote(n), nil
	}
}

// --- collection/list ---

// listCollectionsHandler lists every configured collection with a live record
// count. The method takes no params.
func listCollectionsHandler(svc *service.Service) func(context.Context) ([]rpc.Collection, error) {
	return func(ctx context.Context) ([]rpc.Collection, error) {
		cols, err := svc.ListCollections(ctx)
		if err != nil {
			return nil, err
		}
		return toCollections(cols), nil
	}
}

// --- collection/get ---

// getCollectionHandler reads a single collection by name.
func getCollectionHandler(svc *service.Service) func(context.Context, rpc.CollectionParams) (rpc.Collection, error) {
	return func(ctx context.Context, p rpc.CollectionParams) (rpc.Collection, error) {
		c, err := svc.GetCollection(ctx, p.Name)
		if err != nil {
			return rpc.Collection{}, err
		}
		return toCollection(c), nil
	}
}

// --- mount/list ---

// listMountsHandler returns the configured mounts. The method takes no params.
func listMountsHandler(svc *service.Service) func(context.Context) ([]rpc.Mount, error) {
	return func(_ context.Context) ([]rpc.Mount, error) {
		ms, err := svc.Mounts()
		if err != nil {
			return nil, err
		}
		return toMounts(ms), nil
	}
}

// --- index/run ---

// indexHandler incrementally indexes the vault; a non-empty Since uses the
// git-diff fast path.
func indexHandler(svc *service.Service) func(context.Context, rpc.IndexParams) (rpc.IndexStats, error) {
	return func(ctx context.Context, p rpc.IndexParams) (rpc.IndexStats, error) {
		stats, err := svc.Index(ctx, p.Since)
		if err != nil {
			return rpc.IndexStats{}, err
		}
		return toIndexStats(stats), nil
	}
}

// --- index/rebuild ---

// rebuildHandler clears the derived cache and reindexes from scratch. The method
// takes no params.
func rebuildHandler(svc *service.Service) func(context.Context) (rpc.IndexStats, error) {
	return func(ctx context.Context) (rpc.IndexStats, error) {
		stats, err := svc.Rebuild(ctx)
		if err != nil {
			return rpc.IndexStats{}, err
		}
		return toIndexStats(stats), nil
	}
}

// --- archive ---

// archiveHandler snapshots the vault's git history into Dest (empty uses the
// default archives directory).
func archiveHandler(svc *service.Service) func(context.Context, rpc.ArchiveParams) (rpc.ArchiveResult, error) {
	return func(ctx context.Context, p rpc.ArchiveParams) (rpc.ArchiveResult, error) {
		path, err := svc.Archive(ctx, p.Dest)
		if err != nil {
			return rpc.ArchiveResult{}, err
		}
		return rpc.ArchiveResult{Path: path}, nil
	}
}

// --- cron/list ---

// listCronHandler returns the configured cron jobs flattened for the wire. The
// method takes no params.
func listCronHandler(svc *service.Service) func(context.Context) ([]rpc.CronJob, error) {
	return func(_ context.Context) ([]rpc.CronJob, error) {
		jobs, err := svc.CronList()
		if err != nil {
			return nil, err
		}
		return toCronJobs(jobs), nil
	}
}

// --- cron/run ---

// runCronHandler executes a cron job by name. service.CronRun streams to an
// io.Writer; this buffers that stream into the typed result string (the
// streaming variant is deferred per the spec amendment). The stardust binary is
// resolved from the running executable, mirroring the REST cron-run handler.
func runCronHandler(svc *service.Service) func(context.Context, rpc.CronRunParams) (rpc.CronRunResult, error) {
	return func(ctx context.Context, p rpc.CronRunParams) (rpc.CronRunResult, error) {
		exe, _ := os.Executable()
		var buf bytes.Buffer
		if err := svc.CronRun(ctx, p.Name, exe, &buf); err != nil {
			return rpc.CronRunResult{}, err
		}
		return rpc.CronRunResult{Output: buf.String()}, nil
	}
}

// --- memory/remember ---

// rememberHandler stores a fact add-only and reports where it landed (appended to
// the nearest note or created under memory/). It backs the remember MCP tool.
func rememberHandler(svc *service.Service) func(context.Context, rpc.RememberParams) (rpc.RememberResult, error) {
	return func(ctx context.Context, p rpc.RememberParams) (rpc.RememberResult, error) {
		res, err := svc.Remember(ctx, p.Fact)
		if err != nil {
			return rpc.RememberResult{}, err
		}
		return rpc.RememberResult{Action: res.Action, Path: res.Path}, nil
	}
}

// --- memory/edit ---

// memoryEditHandler applies one memory verb to a vault file and returns the
// human-readable outcome line, reindexing the changed file. It backs the memory
// MCP tool; the params mirror service.MemoryOp field for field.
func memoryEditHandler(svc *service.Service) func(context.Context, rpc.MemoryParams) (rpc.MemoryResult, error) {
	return func(ctx context.Context, p rpc.MemoryParams) (rpc.MemoryResult, error) {
		out, err := svc.Memory(ctx, service.MemoryOp{
			Command: p.Command,
			Path:    p.Path,
			Content: p.Content,
			Old:     p.OldStr,
			New:     p.NewStr,
			Line:    p.Line,
			Text:    p.Text,
			Dest:    p.Dest,
		})
		if err != nil {
			return rpc.MemoryResult{}, err
		}
		return rpc.MemoryResult{Result: out}, nil
	}
}

// --- mapping ---

// toRecord projects a service.Record onto the wire-typed rpc.Record. The field
// sets are identical; this keeps the contract package free of an internal
// import.
func toRecord(r service.Record) rpc.Record {
	return rpc.Record{
		Path:        r.Path,
		Title:       r.Title,
		Frontmatter: r.Frontmatter,
		Body:        r.Body,
	}
}

// toRecordList projects a service.RecordList onto rpc.RecordList.
func toRecordList(l service.RecordList) rpc.RecordList {
	records := make([]rpc.Record, 0, len(l.Records))
	for _, r := range l.Records {
		records = append(records, toRecord(r))
	}
	return rpc.RecordList{
		Collection: l.Collection,
		Folder:     l.Folder,
		Records:    records,
	}
}

// toPredicates projects the wire predicates onto service.Predicate (an alias of
// index.Predicate). The field sets are identical.
func toPredicates(ps []rpc.Predicate) []service.Predicate {
	if ps == nil {
		return nil
	}
	out := make([]service.Predicate, 0, len(ps))
	for _, p := range ps {
		out = append(out, service.Predicate{Field: p.Field, Op: p.Op, Value: p.Value})
	}
	return out
}

// toQueryResult projects a service.QueryResult onto rpc.QueryResult, mapping each
// index.Hit onto the wire-typed rpc.Hit. The field sets are identical.
func toQueryResult(r service.QueryResult) rpc.QueryResult {
	hits := make([]rpc.Hit, 0, len(r.Hits))
	for _, h := range r.Hits {
		hits = append(hits, rpc.Hit{
			Path:    h.Path,
			Title:   h.Title,
			Heading: h.Heading,
			Snippet: h.Snippet,
			Score:   h.Score,
		})
	}
	return rpc.QueryResult{Query: r.Query, Mode: r.Mode, Hits: hits}
}

// toBundleResult projects a service.BundleResult onto rpc.BundleResult, mapping
// each service.BundleItem onto the wire-typed rpc.BundleItem.
func toBundleResult(r service.BundleResult) rpc.BundleResult {
	items := make([]rpc.BundleItem, 0, len(r.Items))
	for _, it := range r.Items {
		items = append(items, rpc.BundleItem{
			Path:    it.Path,
			Title:   it.Title,
			Snippet: it.Snippet,
			Score:   it.Score,
		})
	}
	return rpc.BundleResult{Task: r.Task, Items: items, Markdown: r.Markdown, Tokens: r.Tokens}
}

// toGraphResult projects a service.GraphReport onto rpc.GraphResult, mapping the
// graph package's BrokenLink and PageRankEntry onto their wire-typed twins.
func toGraphResult(r service.GraphReport) rpc.GraphResult {
	broken := make([]rpc.BrokenLink, 0, len(r.Broken))
	for _, b := range r.Broken {
		broken = append(broken, rpc.BrokenLink{From: b.From, Target: b.Target})
	}
	pagerank := make([]rpc.PageRankEntry, 0, len(r.PageRank))
	for _, p := range r.PageRank {
		pagerank = append(pagerank, rpc.PageRankEntry{Path: p.Path, Title: p.Title, Score: p.Score})
	}
	return rpc.GraphResult{
		Notes:    r.Notes,
		Links:    r.Links,
		Orphans:  r.Orphans,
		Broken:   broken,
		PageRank: pagerank,
	}
}

// toCheckResult projects a service.CheckResult onto rpc.CheckResult, mapping each
// service.Issue onto the wire-typed rpc.Issue.
func toCheckResult(r service.CheckResult) rpc.CheckResult {
	issues := make([]rpc.Issue, 0, len(r.Issues))
	for _, is := range r.Issues {
		issues = append(issues, rpc.Issue{
			Severity: is.Severity,
			Kind:     is.Kind,
			Path:     is.Path,
			Detail:   is.Detail,
		})
	}
	return rpc.CheckResult{Issues: issues, Errors: r.Errors, Warnings: r.Warnings, Markdown: r.Markdown}
}

// toNote projects a service.Note onto the wire-typed rpc.Note, mapping each
// service.LinkTarget onto rpc.LinkTarget. The field sets are identical.
func toNote(n service.Note) rpc.Note {
	targets := make([]rpc.LinkTarget, 0, len(n.LinkTargets))
	for _, lt := range n.LinkTargets {
		targets = append(targets, rpc.LinkTarget{Link: lt.Link, Path: lt.Path})
	}
	return rpc.Note{
		Path:        n.Path,
		Title:       n.Title,
		Tags:        n.Tags,
		Links:       n.Links,
		LinkTargets: targets,
		Frontmatter: n.Frontmatter,
		Body:        n.Body,
	}
}

// toCollection projects a service.CollectionInfo onto the wire-typed
// rpc.Collection, mapping each collections.Field onto rpc.Field.
func toCollection(c service.CollectionInfo) rpc.Collection {
	fields := make([]rpc.Field, 0, len(c.Fields))
	for _, f := range c.Fields {
		fields = append(fields, rpc.Field{
			Name:     f.Name,
			Type:     f.Type,
			Required: f.Required,
			Enum:     f.Enum,
			Default:  f.Default,
		})
	}
	return rpc.Collection{
		Name:        c.Name,
		Path:        c.Path,
		Description: c.Description,
		Fields:      fields,
		Records:     c.Records,
	}
}

// toCollections projects a slice of service.CollectionInfo onto rpc.Collection.
func toCollections(cs []service.CollectionInfo) []rpc.Collection {
	out := make([]rpc.Collection, 0, len(cs))
	for _, c := range cs {
		out = append(out, toCollection(c))
	}
	return out
}

// toMounts projects a slice of service.MountInfo onto the wire-typed rpc.Mount.
// The field sets are identical.
func toMounts(ms []service.MountInfo) []rpc.Mount {
	out := make([]rpc.Mount, 0, len(ms))
	for _, m := range ms {
		out = append(out, rpc.Mount{
			Name:   m.Name,
			Kind:   m.Kind,
			Target: m.Target,
			Args:   m.Args,
			Tool:   m.Tool,
		})
	}
	return out
}

// toIndexStats projects a service.IndexStats onto the wire-typed rpc.IndexStats.
// The field sets are identical.
func toIndexStats(s service.IndexStats) rpc.IndexStats {
	return rpc.IndexStats{
		Indexed: s.Indexed,
		Skipped: s.Skipped,
		Deleted: s.Deleted,
		Vectors: s.Vectors,
	}
}

// toCronJob flattens a cron.Job onto the wire-typed rpc.CronJob: the Trigger's
// schedule, event, and path globs and the Run's kind plus its kind-specific
// fields are lifted into a flat shape for the wire.
func toCronJob(j cron.Job) rpc.CronJob {
	return rpc.CronJob{
		Name:     j.Name,
		Schedule: j.Trigger.Schedule,
		On:       j.Trigger.On,
		Paths:    j.Trigger.Paths,
		Kind:     j.Run.Kind,
		Command:  j.Run.Command,
		Exec:     j.Run.Exec,
		Prompt:   j.Run.Prompt,
		Model:    j.Run.Model,
	}
}

// toCronJobs projects a slice of cron.Job onto rpc.CronJob.
func toCronJobs(js []cron.Job) []rpc.CronJob {
	out := make([]rpc.CronJob, 0, len(js))
	for _, j := range js {
		out = append(out, toCronJob(j))
	}
	return out
}
