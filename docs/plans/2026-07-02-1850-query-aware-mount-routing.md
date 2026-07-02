---
title: Query-aware mount routing - implementation plan
status: Draft
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-1850-query-aware-mount-routing.md
  - docs/adr/0042-query-aware-mount-routing.md
---

Add a query-aware routing plan to `QueryMounts` that scopes the mount fan-out on confidence and falls back to searching all mounts on any ambiguity, then adversarially prove single-mount, no-mount, and metadata-less workspaces are byte-identical.

## Header

- **Goal:** `query --mounts` searches only the mounts a query is about when it is confident, and searches everything otherwise; recall never drops silently.
- **Architecture:** a pure `routePlan` helper decides the plan from mount metadata plus the query before any subprocess launches; `QueryMounts` searches only planned mounts and returns a typed `MountQueryResult` mirroring `RetrievalMode`.
- **Tech stack:** Go 1.26, existing embedder (`internal/embed`), existing RRF (`internal/fusion`). No new dependency, no external routing service.
- **Global constraints:** the invariant and fallback rule from ADR 0042 are normative; single-mount, no-mount, and metadata-less workspaces stay byte-identical; the local vault is always searched; conventional commits, no co-author or generated-by trailers, zero U+2014 / U+2013, gate green before every commit.

## Context

Read first: `docs/specs/2026-07-02-1850-query-aware-mount-routing.md` (the Approach and Verification sections are normative), `docs/adr/0042-query-aware-mount-routing.md` (the invariant and the fallback gate), `internal/service/mounts.go` (`QueryMounts`, `FusedHit`), `internal/service/service.go` (`Query`, `RetrievalMode` / `RetrievalReason` as the visibility pattern to mirror), `internal/mounts/mounts.go` (`Config`, `Mount.Search`), `internal/embed/ollama.go` (batch `Embed`, `Available`), and `internal/cli/query.go` (the only `QueryMounts` caller, `renderFused`).

## Task 1: routing plan, wiring, and visibility

Files:

- Modify: `internal/mounts/mounts.go` (add optional `Description`, `Keywords` to `Config`)
- Modify: `internal/service/mounts.go` (`MountQueryResult`, routing constants, `routePlan` helper, rewire `QueryMounts`)
- Modify: `internal/service/service.go` (expose the query vector or a small embed-reuse seam so routing does not re-embed)
- Modify: `internal/cli/query.go` (extend `--mounts` to accept an optional value list, render the routing line, emit the struct in JSON)
- Create: `internal/service/routing_test.go` (unit tests for `routePlan` and the fallback gate)
- Optional: `rpc/contract.go` (additive result type, only if the RPC path exposes `query --mounts`)

Steps:

- [ ] Add optional `Description string` and `Keywords []string` to `mounts.Config`; confirm a mount with neither parses fine and is treated as unroutable.
- [ ] Add `MountQueryResult` and the `RoutingAll` / `RoutingRouted` / `RoutingFallback` constants to `internal/service/mounts.go`, mirroring `QueryResult` / `RetrievalMode`.
- [ ] Write `routing_test.go` first (test-driven): cover the `<= 1` gate, explicit list, name mention, semantic match above and below `routeCosineThreshold`, lexical match in fts-only, metadata-less always-in, empty-subset fallback, full-set fallback, and no-metadata fallback. Assert `MountsSearched` / `MountsSkipped` / `RoutingReason` in each.
- [ ] Implement `routePlan(mounts, query, queryVec, semantic bool)` as a pure function returning the planned mount subset plus mode and reason, enforcing the invariant and the strict-non-empty-subset gate. No subprocess launches inside it.
- [ ] Thread the query vector out of `Query` (or add a minimal embed-reuse seam) so routing reuses it; cache mount description vectors keyed by a hash of `embed_model` plus description text (index `meta` table).
- [ ] Rewire `QueryMounts`: build the plan, search only planned mounts, keep the graceful skip-on-error behavior, RRF-fuse as before, and populate `MountQueryResult` including the inherited `RetrievalMode` / `RetrievalReason`.
- [ ] Extend `--mounts` to accept an optional comma list (bare `--mounts` still means all); render the routing line in `renderFused`; emit the struct in JSON.
- [ ] Gate: `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` empty, dash-scan (no U+2014 / U+2013), `stardust check` exit 0.
- [ ] Commit `feat(mounts): route query --mounts to relevant mounts with fallback to all`.

## Task 2: adversarial review

Steps:

- [ ] Byte-identical proof, no-mount: capture `query` and `query --mounts` output in this repo before and after the change; assert identical (zero mounts hits the `<= 1` gate).
- [ ] Byte-identical proof, single-mount and metadata-less: fixtures for one mount, and two mounts with no metadata, both search everything with mode `all` and no routing logic engaged.
- [ ] Recall-safety proof: construct a query that just misses a mount's description under threshold while that mount is the only metadata-less one, and confirm it is still searched; construct an all-metadata no-match case and confirm fallback to all, not empty.
- [ ] Routed proof: semantic match, explicit list, and name mention each scope correctly, and `MountsSkipped` names every excluded mount with a reason.
- [ ] Mode proof: with embeddings down, routing degrades to lexical or fallback, and the result shows both `retrieval_mode: fts-only` and the routing mode.
- [ ] Vault-always-searched proof: every routed and fallback case still returns vault hits.
- [ ] Confirm `Bundle` output is unchanged (it never fanned out to mounts).
- [ ] Verify `git log` shows clean conventional commits with no trailers; dash-scan every touched file.
- [ ] Report defects; do not fix silently.

## Verification

The spec's Verification cases green; no-mount, single-mount, and metadata-less workspaces byte-identical; a mount is dropped only on a confident non-match and never in an uncertain case; the vault is always searched; routing and retrieval modes both visible in human and JSON output; gate clean.

## Self-review gate

- Every ADR 0042 clause of the invariant maps to a `routePlan` test case; the fallback gate has an empty-subset, full-set, and no-metadata test.
- The only behavior change is on the `query --mounts` path; `query`, `bundle`, and `mount/list` are untouched.
- Routing never launches a subprocess to decide; it plans from metadata plus the query alone.
