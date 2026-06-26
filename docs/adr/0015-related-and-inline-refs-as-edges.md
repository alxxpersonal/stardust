---
title: Related and inline path refs are first-class edges
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-doc-code-coherence-engine.md
---

# Related and inline path refs are first-class edges

Edge extraction covers wikilinks, `related:` frontmatter refs, and inline repo-path references, classified into doc-to-doc graph edges and doc-to-code references by on-disk resolution.

## Context

`internal/vault/vault.go:ExtractLinks` (45-58) extracts wikilinks only. `internal/graph/graph.go:Build` (line 50) sets `Out: note.Links`, so `related:` and inline paths never become edges. `internal/convention/check.go:checkRelated` (124-140) validates that `related:` targets exist but discards them. Consequences: a doc linked only by `related:` reports as an orphan (`graph.go:Orphans`, line 69) and is unreachable by `PersonalizedPageRank` bundle expansion (`bundle.go:59`); and doc-to-code references, the substrate for drift detection, are never captured. The SPEC already calls for deriving the graph from wikilinks plus frontmatter relations; only the wikilink half is implemented.

## Decision

Add `vault.ExtractEdges(note) []Edge` with `Edge{Target, Kind}` where `Kind` is `wikilink`, `related`, or `inline-path`. Wikilinks come from `wikilinkRe`; `related` from `note.Frontmatter["related"]` via `convention.StringList`; inline-path candidates from backtick-delimited inline-code spans matching a `dir/dir/file.ext` shape, kept only when they resolve to an existing repo file. A resolver classifies each resolved target: a markdown note in the vault is a graph edge; a repo file that is not a `.md` note is a code reference. Code references never become graph nodes; they feed drift detection (ADR 0018). `internal/graph/graph.go:Node` carries typed out-edges so `Orphans`, `BrokenLinks`, `Neighbors`, and `PersonalizedPageRank` operate over the union of wikilink and `related` doc edges. `checkRelated` keeps validating existence and now also contributes edges.

## Consequences

- A doc linked only by `related:` stops reading as an orphan and is reachable by bundle expansion.
- Doc-to-code references become a first-class signal, enabling drift detection without a separate parse.
- Inline-path extraction resolves by existence, so false positives are bounded; an unresolvable token is not an edge.
- The graph stays a rebuildable cache derived by regex plus YAML, under two seconds, at zero cost, consistent with derive-don't-store.
- Edge `Kind` is available to bundle provenance (spec point 9).

## Alternatives considered

- Treat `related:` as a graph edge but ignore inline code paths. Rejected: ADRs reference code through inline paths, and drift needs those bindings.
- LLM-extract edges. Rejected by the SPEC: regex plus frontmatter yields more edges, faster, free.
- Make code references graph nodes too. Rejected: code files are not notes; mixing them pollutes orphan and PageRank semantics. They are a separate reference channel.

## References

- `internal/vault/vault.go`, `internal/graph/graph.go`, `internal/convention/check.go`, `internal/service/bundle.go`.
- ADR 0018 (drift consumes code references).
