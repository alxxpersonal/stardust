// Package rpcserver assembles the typed JSON-RPC method registry over the
// Stardust service core. It maps the canonical slash method names to typed
// handlers, each a thin caller of an internal/service method. jrpc2 owns request
// decoding, response encoding, id correlation, and the error band (ADR 0006); a
// handler returns a typed value and a plain error, nothing more.
package rpcserver

import (
	"context"

	"github.com/creachadair/jrpc2/handler"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/rpc"
)

// NewRegistry builds the handler.Map of canonical slash method names to typed
// handlers over svc. The same map backs every transport (stdio, HTTP, MCP) so
// the surfaces stay at structural parity.
func NewRegistry(svc *service.Service) handler.Map {
	return handler.Map{
		"status":        handler.New(statusHandler(svc)),
		"record/create": handler.New(createRecordHandler(svc)),
		"record/get":    handler.New(getRecordHandler(svc)),
		"record/list":   handler.New(listRecordsHandler(svc)),
		"record/patch":  handler.New(patchRecordHandler(svc)),
		"record/delete": handler.New(deleteRecordHandler(svc)),
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
			return rpc.Record{}, err
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
			return rpc.Record{}, err
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
