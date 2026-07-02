---
title: Query-aware mount routing prunes only on confidence and always falls back to all
status: Accepted
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-1850-query-aware-mount-routing.md
  - docs/plans/2026-07-02-1850-query-aware-mount-routing.md
  - internal/mounts/mounts.go
  - internal/service/mounts.go
  - internal/service/service.go
  - internal/embed/ollama.go
  - internal/cli/query.go
---

# Query-aware mount routing prunes only on confidence and always falls back to all

`query --mounts` scopes its fan-out to the mounts a query is about, using a routing signal derived from mount self-description (name, an optional `description`, optional `keywords`) matched against the query with the embedder Stardust already has. Routing removes an external mount only when it is confident the mount is irrelevant; it never removes the local vault, never removes a metadata-less mount, and falls back to searching all mounts on any ambiguity. Single-mount, no-mount, and metadata-less workspaces behave byte-identically to today.

## Context

Mounts are external MCP servers, not local index rows. `mounts.Mount.Search` launches a connector as a subprocess per call (spawn plus MCP handshake plus tool call plus close), and `service.QueryMounts` runs these sequentially over every configured mount, then RRF-fuses. So a query against N mounts pays N sequential subprocess round-trips, most against sources irrelevant to the query, plus RRF noise from those sources. SPEC.md section 11 names "semantic mount routing at scale" as future work, and section 12.1 records the current-scale position bluntly: "At this scale, fan-out-to-all-mounts + RRF beats a query router." Stardust is a thin aggregator for one trusted human, not an enterprise gateway.

Three facts constrain the design.

First, the saved cost is subprocess-launch latency and result noise, not SQLite latency. Mount content is never indexed locally, so any signal that assumes local chunks (a per-mount centroid) or a local probe (an FTS hit-count pre-pass) does not apply: there are no chunks to average, and probing an external mount costs the same as searching it.

Second, the error is asymmetric. A wrongly excluded mount loses recall silently, with no error and no visible symptom; a wrongly included mount costs one extra subprocess and a little RRF dilution. Over-inclusion is cheap and reversible; under-inclusion is a silent correctness failure. The design must be biased toward searching too much.

Third, the win is scale-dependent. At the counts Stardust runs today (this vault has zero mounts; SPEC 12.1 assumes a handful), fan-out-plus-RRF already wins, so routing must be a no-op there and only earn its keep as the mount count grows. Retrieval already models a two-mode world (`hybrid-semantic` when Ollama is up, `fts-only` when it is not) and announces it via `RetrievalMode` plus `RetrievalReason`; routing must fit both modes and mirror that visibility.

## Decision

Add query-aware routing to `QueryMounts` as a pre-fan-out plan, governed by one invariant and a conservative fallback.

- **Signal without a new model: mount self-description.** Extend `mounts.Config` with optional `Description string` and `Keywords []string`, read from the mount's existing `config.toml`. A mount with neither is unroutable. In hybrid-semantic mode, embed the query once (reusing the vector `Query` already computes) and cosine-compare it to each mount's embedded description; a mount is a candidate at or above `routeCosineThreshold = 0.35`. In fts-only mode, match by case-folded token overlap between the query and the mount's name, keywords, and description. Explicit scope outranks both: an explicit `--mounts=a,b` value list, or a mount name mentioned in the query text, scopes directly.
- **The invariant.** Routing removes an external mount from the fan-out only when (a) there are two or more mounts, (b) the removed mount carries routing metadata, (c) that metadata did not match the query above threshold, and (d) at least one mount remains. In every other case, search all. A metadata-less mount is never removed. The local vault is never removed.
- **The fallback (the conservative spine).** Compute a confident subset = matched mounts union metadata-less mounts. Fall back to ALL, mode `fallback`, whenever the subset is empty, the subset equals the full set, or routing could not be computed (embeddings wanted but the query embed failed with no lexical signal, or no mount carries metadata). Route only when the plan is a strict, non-empty subset of the mount set.
- **Per-mode behavior.** Semantic routing runs only in hybrid-semantic mode; fts-only routes lexically or not at all, and is strictly more conservative. Routing mode is independent of retrieval mode and travels beside it.
- **Metadata home and cache.** Routing metadata lives in the mount `config.toml`. Description vectors are cached keyed by a hash of `embed_model` plus the description text, so an edit to either invalidates the cache; the index `meta` key-value table is the default store.
- **Visibility, mirroring `RetrievalMode`.** `QueryMounts` returns a `MountQueryResult` struct carrying `RoutingMode` (`all` / `routed` / `fallback`), an optional `RoutingReason`, `MountsSearched`, `MountsSkipped`, and the inherited `RetrievalMode` / `RetrievalReason`. The CLI prints a routing line in the `renderFused` header; JSON callers read the fields.

## Consequences

- Single-mount and no-mount workspaces are byte-identical: the `<= 1` gate short-circuits to `all` with no routing logic. This repo (zero mounts) is the direct regression proof.
- Metadata-less mounts stay byte-identical too: with no descriptions, the confident subset equals the full set, so routing falls back to all. This makes the design honor SPEC 12.1's "fan-out beats a router at this scale" automatically, and routing only begins pruning once mounts carry descriptions and a real strict subset emerges.
- The saved cost is real and scale-growing: as mount count rises, a scoped query skips the subprocess-launch tax and the RRF noise of mounts a query is not about, which is the "at scale" win SPEC 11 asked for.
- Recall is protected by construction: the loose threshold, the metadata-less rule, the vault-always-searched rule, and fallback-to-all mean a mount is dropped only on a confident non-match, and never silently in an uncertain case.
- The decision is legible: every query announces whether it routed, searched all, or fell back, and which mounts it touched and why, in both human and JSON output.
- One caller changes (`internal/cli/query.go`); the JSON shape gains fields additively. `Bundle` is untouched (it does not fan out to mounts). No new dependency, no external service.

## Alternatives considered

- **Mount centroid vectors from indexed chunks.** Rejected. Mount content is never indexed locally, so there are no chunks to average; building a centroid means ingesting each mount's corpus, violating the thin-aggregator and derive-don't-store principles (SPEC 3, 12.1).
- **FTS hit-count probe per mount as a cheap pre-pass.** Rejected. Probing an external MCP mount costs the same as searching it (subprocess plus handshake plus call), so a "cheap pre-pass" is a contradiction here; the idea only fits a local index, which mounts lack.
- **A learned or LLM query router.** Rejected. New model dependency, violates no-new-heavyweight-deps and no-external-routing-service, and SPEC 12.1 says fan-out-plus-RRF beats a router at this scale.
- **Prune the local vault when a query looks external.** Rejected. The vault is the cheap, always-available, highest-trust recall floor; dropping it saves one local SQLite query and risks silent recall loss.
- **Dedicated per-mount vector table in sqlite-vec.** Deferred. A batched embed of a few short descriptions is cheap enough that a dedicated table is premature; the hash-keyed `meta` cache is the first optimization if N grows.
- **Parallelize the fan-out loop instead of routing.** Complementary, not a substitute: parallelizing lowers per-mount cost but still pays every irrelevant mount. Recorded as future work; the two compose.

## References

- docs/specs/2026-07-02-1850-query-aware-mount-routing.md
- docs/plans/2026-07-02-1850-query-aware-mount-routing.md
- SPEC.md sections 11 (future work) and 12.1 (mounts / context mesh, "fan-out-to-all-mounts + RRF beats a query router")
- internal/mounts/mounts.go (`Config`, `Mount.Search`, `Load`)
- internal/service/mounts.go (`QueryMounts`, `FusedHit`, `Mounts`)
- internal/service/service.go (`Query`, `RetrievalMode`, `RetrievalReason`)
- internal/embed/ollama.go (`Available`, batch `Embed`)
- internal/cli/query.go (`--mounts` flag, `renderFused`)
