---
title: Query-aware mount routing with conservative fallback to all
status: Implemented
version: 1
date: 2026-07-02
related:
  - docs/adr/0042-query-aware-mount-routing.md
  - docs/plans/2026-07-02-1850-query-aware-mount-routing.md
  - internal/mounts/mounts.go
  - internal/service/mounts.go
  - internal/service/service.go
  - internal/embed/ollama.go
  - internal/cli/query.go
  - rpc/contract.go
---

# Query-aware mount routing with conservative fallback to all

When a workspace has several mounts, scope a `query --mounts` fan-out to the mounts a query is actually about, so a query stops paying the launch-and-search tax for mounts that cannot help. Routing only ever prunes external mounts it is confident are irrelevant; it never drops the local vault, never drops a mount that carries no routing metadata, and degrades to searching every mount whenever confidence is low. Single-mount and no-mount workspaces behave byte-identically to today.

<details>
<summary><b>Problem</b></summary>
<br>

SPEC.md names "semantic mount routing at scale (query-aware routing to the right mounts)" as genuine future work (SPEC section 11, "semantic mount routing at scale"). Today `service.QueryMounts` loads every configured mount and searches all of them on every query, then fuses with RRF. There is no routing: a query either fans out to everything or the caller filters by hand after the fact.

The cost this saves is not SQLite latency. Mounts are not locally indexed. Each mount is an external MCP server, and `mounts.Mount.Search` launches it as a subprocess per call (`exec.CommandContext` plus an MCP `Connect` handshake plus `CallTool` plus `Close`), and the fan-out loop in `QueryMounts` runs these sequentially. So a query against N mounts pays N sequential (process spawn plus stdio MCP handshake plus remote search) round-trips, most of them against sources that have nothing to say about the query. The local vault SQLite query is the cheap part; the mounts are the slow, high-variance part. On top of the latency, RRF fuses every mount's top-k into the ranking, so irrelevant mounts dilute the result set with noise.

The asymmetry that shapes the whole design: a wrongly EXCLUDED mount loses recall silently. The user never sees the note that would have answered them, and there is no error to notice. A wrongly INCLUDED mount only costs one extra subprocess launch and some RRF noise. So over-inclusion is cheap and reversible; under-inclusion is a silent correctness failure. The design must be biased hard toward searching too much rather than too little.

</details>

<details>
<summary><b>Context</b></summary>
<br>

- `internal/mounts/mounts.go`: a `Mount` is `{Name string, Cfg Config}`. `Config` is `{Command, Args, Env, Tool}` read from `.stardust/mounts/<name>/config.toml`. There is no description, no keyword list, no self-description of any kind today; a mount knows only how to launch its connector. `Load(mountsDir)` reads every subfolder, sorts by name, and returns `nil` (no error) when the dir is absent. `Search(ctx, query, limit)` spawns the connector, calls its search tool with `{query, limit}`, and parses hits; a per-call subprocess.
- `internal/service/mounts.go`: `QueryMounts(ctx, query, limit)` calls `s.Query` for the local vault, then loops every mount calling `m.Search`, skips a mount that errors (graceful degradation is already the rule), RRF-fuses all lists, and returns a bare `[]FusedHit`. It carries no metadata about what was searched. `MountNames()` and `Mounts()` are the read-only inventory behind the `/mounts` surface and `mount/list` RPC.
- `internal/service/service.go`: `Query` embeds the query when `s.embed.Available(ctx)` is true, runs `s.store.Hybrid`, and announces `RetrievalMode` (`hybrid-semantic` or `fts-only`) plus a one-line `RetrievalReason` when degraded. This is the exact visibility pattern to mirror: a typed result struct with a mode string and an optional reason. The query vector it computes is local to `Query` and is not returned today.
- `internal/embed/ollama.go`: `Available(ctx)` probes Ollama with a 3s timeout; `Embed(ctx, texts)` is a batch endpoint (one call embeds many strings). Both already gate the whole vector path, so "semantic available vs fts-only" is a solved, reused distinction.
- `internal/cli/query.go`: the only caller of `QueryMounts`. `--mounts` is a bool flag ("also search configured mounts and fuse the results"). `renderFused` prints a header with the fused-source note and then the hits. `renderHits` (local path) already prints the retrieval `mode` in its header, the surface to mirror.
- `internal/service/bundle.go`: `Bundle` seeds from `s.Query` (local vault plus link graph) and never fans out to mounts. Bundles are out of scope for routing; this spec touches the `query --mounts` path only.
- SPEC.md section 12.1 is explicit on the current-scale tradeoff: "At this scale, fan-out-to-all-mounts + RRF beats a query router." Stardust is a thin MCP aggregator for one trusted human, not an enterprise gateway, and the discipline guardrail is "the crazy lives in routing/fusion, never in rebuilding storage." Routing therefore must not fight fan-out-plus-RRF at low mount counts; it must be a no-op there and earn its keep only as the mount count grows.
- This vault has zero mounts configured (`.stardust/mounts/` is absent), so the byte-identical constraint is directly testable here: nothing about `query` or `query --mounts` output may change in this repo.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. When a workspace has two or more mounts, scope a `query --mounts` fan-out to the mounts a query is about, cutting the subprocess-launch tax and the RRF noise from irrelevant mounts.
2. Never lose recall silently. Routing may exclude an external mount only when it is confident the mount is irrelevant; every uncertain case searches all mounts.
3. Zero new heavyweight dependency and no external routing service. Reuse the existing embedder plumbing, mount config, and RRF fusion. Derive the routing signal without a new model.
4. Work in both retrieval modes. When embeddings are available, route semantically; when Stardust is fts-only, route lexically or not at all, and be strictly more conservative.
5. Make the routing decision visible, mirroring `retrieval_mode`: query output and the JSON result announce which mounts were searched, which were skipped, and why (routed, all, or fallback).
6. Byte-identical behavior for single-mount and no-mount workspaces, and for any mount that carries no routing metadata.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No routing for the local vault. The vault is always searched; routing only ever prunes external mounts. The primary recall source is never at risk.
- No routing for `bundle`. `Bundle` does not fan out to mounts today and this spec does not change that.
- No indexing of mount content. Stardust stays a thin aggregator (SPEC 12.1); it never ingests a mount's corpus to build a centroid or a local mirror. The routing signal comes from a mount's self-description, not its data.
- No learned or LLM-based query router, no new model, no network service for routing. The signal is metadata matching plus the embedder Stardust already has.
- No parallelization of the fan-out loop. Parallelizing reduces the per-mount cost and is a real, complementary optimization, but it is orthogonal to deciding which mounts to launch. It is recorded as future work so the two are not conflated.
- No change to RRF fusion, to `mount/list`, or to how a mount's hits are parsed.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

One invariant governs everything: routing may remove an external mount from the fan-out only when (a) there are two or more mounts, (b) the removed mount carries routing metadata, (c) that metadata did not match the query above threshold, and (d) at least one mount still remains after removal. In every other situation, search all mounts. A metadata-less mount is never removed. The local vault is never removed.

**Routing metadata (where it lives).** Extend `mounts.Config` with two optional fields, read from the mount's existing `config.toml`:

```toml
description = "team wiki and project notes in Notion"
keywords = ["notion", "wiki", "projects", "meeting notes"]
```

Both are optional. A mount with neither is unroutable and therefore always searched. This is the backward-compatible default: every mount that exists today has no metadata, so every existing mount is always searched, which is byte-identical to current behavior.

**The routing plan (computed before any subprocess launches).** `QueryMounts` builds a plan from mount metadata and the query alone, paying zero connector cost to decide:

1. Load mounts. If `len(mounts) <= 1`, the plan is ALL with mode `all`. No routing logic runs. This is the byte-identical path for this repo and every single-mount workspace.
2. Explicit scope wins next, highest confidence:
   - An explicit `--mounts=notion,postgres` value list scopes to exactly those named mounts. The bare `--mounts` flag keeps meaning "all mounts". An explicit list has no soft fallback: the user asked for it.
   - A mount name appearing as a token in the query text ("search notion for the launch plan") routes to the mentioned mounts plus every metadata-less mount. A name mention is a strong signal but not an explicit scope: a free-text token can coincide with a mount name, so it never excludes a mount that has no metadata to be judged by.
3. Otherwise soft routing runs over the metadata-bearing mounts:
   - hybrid-semantic mode (embeddings available): embed the query once and reuse that vector; cosine-compare it to each mount's embedded `description`. A mount is a candidate when cosine is at or above `routeCosineThreshold`.
   - fts-only mode (embeddings unavailable): case-folded token overlap between the query and each mount's `name` + `keywords` + `description`. A mount is a candidate when it has at least one strong token hit.
   - Every mount with no metadata is unconditionally a candidate (it cannot be confidently excluded).
   - The confident subset is the union of matched mounts and metadata-less mounts.
4. Apply the fallback gate (below) to the confident subset. If it passes, the plan is that subset with mode `routed`; otherwise the plan is ALL with mode `fallback`.
5. The local vault is always in the plan, independent of steps 1 to 4.

**Fallback-to-all rule (the conservative spine).** Two distinct outcomes both search every mount, and they carry different modes so the visibility stays honest:

- Mode `fallback` (routing engaged but pruned nothing). Any of: the confident subset is empty (never scope to zero; that would silently zero out all mount recall); the confident subset equals the full mount set (routing ran and every mount stayed in); or semantic routing was wanted but the query embed failed while metadata is present (no usable signal, so nothing is confidently excludable). The routing line renders, announcing the safe back-off.
- Mode `all` (routing never engaged). One or zero mounts, or no mount carries any routing metadata: there is nothing to route on, so the search is byte-identical to a pre-routing workspace and the routing line stays silent.

The gate is exactly "the plan is a strict, non-empty subset of the mount set." A strict non-empty subset routes; an empty or full subset (after routing engaged) falls back; a workspace with nothing to route on stays quiet `all`. Note the emergent, desired consequence at low scale: when most mounts have no metadata (the current reality), the workspace has nothing to route on and stays `all`, matching SPEC 12.1's "fan-out-to-all beats a query router at this scale" for free. Routing only starts pruning once enough mounts carry descriptions that a real strict subset emerges.

**Thresholds (documented, tunable, deliberately loose).**

- `routeCosineThreshold = 0.35`. A mount description must be at least this cosine-similar to the query to be a semantic candidate. Set low on purpose: false-exclude is the costly error, so we over-include.
- Lexical: at least one exact case-folded token match between the query and the mount's `name` + `keywords` + `description`.
- No minimum-mount-count floor beyond the `>= 2` gate. At tiny counts the metadata-less rule and the strict-subset gate already keep routing quiet.

**Per-mode behavior.** Semantic routing is active only in hybrid-semantic mode and reuses the query vector already computed for the local query. In fts-only mode semantic routing is off and only explicit scope and lexical keyword matching are available, so fts-only routes strictly less and falls back to all strictly more often. The routing mode is independent of the retrieval mode but travels beside it, so output shows both (for example `retrieval_mode: fts-only` alongside `routing_mode: fallback`).

**Description-embedding cost and cache.** Embedding N short descriptions is one batched Ollama call, far cheaper than N subprocess launches, and it only runs when routing is live (two or more mounts, metadata present, embeddings up). Mount description vectors are cached keyed by a hash of `embed_model` plus the description text (recomputed only when the description or model changes); the exact store is an implementation detail for the plan, with the index `meta` key-value table the default home. v1 may compute per call and add the cache in the same pass since it is small.

**Visibility surface (mirror `retrieval_mode`).** Change `QueryMounts` to return a typed result instead of a bare slice:

```go
type MountQueryResult struct {
    Query           string     `json:"query"`
    Hits            []FusedHit `json:"hits"`
    RoutingMode     string     `json:"routing_mode"`     // all | routed | fallback
    RoutingReason   string     `json:"routing_reason,omitempty"`
    MountsSearched  []string   `json:"mounts_searched"`
    MountsSkipped   []string   `json:"mounts_skipped,omitempty"`
    RetrievalMode   string     `json:"retrieval_mode"`
    RetrievalReason string     `json:"retrieval_reason,omitempty"`
}

const (
    RoutingAll      = "all"
    RoutingRouted   = "routed"
    RoutingFallback = "fallback"
)
```

`renderFused` prints a routing line in its header (searched mounts, skipped mounts, and the reason on fallback), exactly as `renderHits` prints the retrieval `mode`. JSON callers read the struct fields. The one caller, `internal/cli/query.go`, is updated; `rpc/contract.go` gains the wire type if and when the RPC path exposes `query --mounts` (it does not today, so the contract change is additive and optional in v1).

</details>

<details>
<summary><b>Alternatives</b></summary>
<br>

- **Mount centroid vectors from indexed chunks (a suggested signal).** Rejected outright for this architecture. Mount content is never indexed locally, so there are no chunks to average into a centroid. Building one would require ingesting every mount's corpus, which violates the thin-aggregator and derive-don't-store principles (SPEC sections 3 and 12.1) and turns Stardust into the "ingest everything" system it explicitly refuses to be.
- **FTS hit-count probe per mount as a cheap pre-pass (a suggested signal).** Rejected. You cannot FTS an external MCP mount without calling it, and calling it is precisely the expensive operation routing exists to avoid. A "cheap pre-pass probe" is a contradiction here; the probe costs the same as the search. The probe idea only makes sense against a local index, which mounts do not have.
- **A learned or LLM query router.** Rejected. It adds a model dependency and violates the no-new-heavyweight-deps and no-external-routing-service constraints, and SPEC 12.1 states fan-out-plus-RRF beats a query router at this scale.
- **Route by embedding the query against a dedicated per-mount vector table in sqlite-vec.** Deferred, not rejected. A per-call batched embed of a handful of short descriptions is cheap enough that a dedicated table is premature; the hash-keyed cache in the `meta` table is the first optimization if N grows large.
- **Let routing prune the local vault too when a query looks purely external.** Rejected. The vault is the cheap, always-available, highest-trust source; dropping it to save one local SQLite query trades the guaranteed recall floor for nothing.
- **Parallelize the fan-out loop instead of routing.** Complementary, not a substitute. Parallelizing lowers the cost of each mount but still pays every irrelevant mount; routing removes them. Recorded as future work; the two compose.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- **Silent recall loss (the worst case).** A relevant mount is pruned because its description did not match a query phrased differently. Mitigation: the loose `0.35` threshold, the metadata-less-mounts-always-searched rule, the strict-subset gate, and fallback-to-all on any ambiguity. The design is biased to over-include; a mount is dropped only on a confident non-match.
- **Stale description embeddings.** A mount description is edited but the cached vector is not recomputed. Mitigation: cache key is a hash of model plus description text, so any edit invalidates it.
- **Metadata rot.** Descriptions drift from what a mount actually holds, degrading routing quality over time. Accepted: a bad description only causes over-inclusion (search a mount that was not needed) as long as the threshold stays loose; it never causes silent exclusion of a metadata-less mount, and an out-of-date description that stops matching simply routes that mount out, which fallback catches when it empties the subset.
- **Embedding on the hot path.** Routing embeds descriptions during a query. Mitigation: it runs only when routing is live and it is one batched call, strictly cheaper than the subprocess launches it avoids; the cache removes it after the first query.
- **Output contract change.** `QueryMounts` returns a struct now, not a slice. Blast radius is one caller (`internal/cli/query.go`); the JSON shape gains fields additively (hits still present).

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- Byte-identical, no-mount: in this repo, `query` and `query --mounts` produce identical output before and after. Zero mounts means the `<= 1` gate short-circuits to `all` with no routing.
- Byte-identical, single-mount: a one-mount fixture searches that mount with mode `all`, no routing logic.
- Byte-identical, metadata-less: a two-mount fixture where neither mount has a description or keywords falls back to `all` (confident subset equals full set), searching both.
- Routed, semantic: a two-mount fixture with descriptions, embeddings available, a query matching one description above threshold, routes to that mount, mode `routed`, `mounts_skipped` names the other with the non-match reason.
- Routed, explicit list: `--mounts=a` on a multi-mount workspace scopes to `a` with no fallback.
- Routed, name mention: a query naming a mount routes to it plus any metadata-less mounts; only described, unmentioned mounts are pruned.
- Fallback, empty subset: descriptions present but none match and none are metadata-less, so the subset would be empty; falls back to `all`.
- Fallback, fts-only: embeddings unavailable, no lexical hit; routes nothing, mode `fallback`, and the result shows both `retrieval_mode: fts-only` and `routing_mode: fallback`.
- Visibility: JSON carries `routing_mode`, `routing_reason`, `mounts_searched`, `mounts_skipped`; the human header prints the routing line.
- Vault-always-searched: every routed and fallback case still includes vault hits.
- Gate: `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` empty, zero U+2014 / U+2013, `stardust check` exit 0.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. `internal/mounts`: add optional `Description` and `Keywords` to `Config`; unit-test that absent metadata leaves a mount unroutable.
2. `internal/service`: add `MountQueryResult`, the `RoutingAll` / `RoutingRouted` / `RoutingFallback` constants, and a pure `routePlan` helper (mounts, query, query vector, mode) computing the plan with the invariant and fallback gate; unit-test every branch of the decision rule and the gate.
3. `internal/service`: thread the query vector out of `Query` (or expose a small embed reuse) so routing does not re-embed; wire `QueryMounts` to build the plan, search only planned mounts, and populate the result; cache description vectors keyed by model-plus-description hash.
4. `internal/cli/query.go`: extend `--mounts` to accept an optional value list; render the routing line in `renderFused`; emit the struct in JSON.
5. `rpc/contract.go`: add the additive result type if the RPC path exposes `query --mounts` (optional in v1).
6. Regenerate the docs index with `stardust registry`.

</details>

## Amendments

- 2026-07-02: adversarial review found the name-mention path violating the recall-safety invariant (a free-text token coinciding with a mount name silently excluded a metadata-less mount) and routing work running on single-mount workspaces. Corrected: only the explicit `--mounts` list scopes directly; a name mention unions with the metadata-less mounts; the single-mount gate now short-circuits before any description embedding. Pinned by TestRoutePlanNameMentionZoteroRepro, TestRoutePlanNameMentionKeepsMetadataless, TestRoutePlanNameMentionPrunesDescribedOnly, and TestQueryMountsSingleMountSkipsRoutingWork.
