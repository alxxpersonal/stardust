---
title: The TUI reads through the service layer
status: Accepted
date: 2026-06-26
---

# The TUI reads through the service layer

## Context

The current TUI's `backend` opens `index.Store` and `embed.Client` directly and the tabs call store methods (`Hybrid`, `Count`, `GetMeta`) and `graph.Build` themselves. The new TUI needs reads the store does not expose directly: hybrid query with retrieval-mode announcement and reranking, collections and records, the doc-code drift and stale math, and the full vault status probe. All of these already live in `internal/service` as the one core every surface calls (ADR 0003). Re-implementing query fusion, drift commit-distance, or status probing in the TUI would fork logic that must stay identical across CLI, MCP, and TUI.

## Decision

The TUI `backend` holds a `*service.Service`, opened once at launch via `service.Open(ctx, layout.Root)`, and every tab reads through service methods:

- Search -> `Query`
- Browse -> `ListCollections`, `ListRecords`, `GetRecord`, `GetNote`
- Graph -> `Graph`
- Drift -> `Check`, `DriftDocs`, `StaleDocs`
- Status -> `GatherStatus` (with `Status` as the fallback for index health)

No tab touches `index.Store`, `embed.Client`, or `graph.Build` directly. `tui.Run` keeps its `(config.Layout, config.Config)` signature and opens the service internally, so the launch caller in `root.go` is unchanged (ADR 0031).

## Consequences

- The TUI has the same capability surface as the CLI and the MCP server. A query in the TUI fuses and reranks exactly as `stardust query` does, because it is the same code.
- Drift and stale are surfaced through the service's own `Markdown` fields, re-rendered with `render.GlamourRender`, so the Drift tab matches `stardust check`/`stale` output.
- One more consumer of the service, which is the intended shape: thin surfaces over one core.
- The service opens its own index handle; the TUI owns its lifecycle and closes it on exit.

## Alternatives considered

- **Keep routing tabs at `index.Store`/`embed.Client`.** Forks query, drift, and status logic; the TUI would drift from the CLI. Rejected.
- **Add TUI-specific methods to the service.** Not needed; the existing read methods cover all five tabs. Rejected as premature.

## References

- Spec: `docs/specs/2026-06-26-2352-interactive-tui.md`.
- ADR 0003 one-method-registry-multi-transport, ADR 0031 additive-default-path-tui.
- `internal/service/service.go`, `internal/service/{records,governs,status_report,check}.go`, `internal/tui/run.go`.
