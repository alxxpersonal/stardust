---
title: Drift detection binds docs to referenced code by commit-distance
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-doc-code-coherence-engine.md
---

# Drift detection binds docs to referenced code by commit-distance

A doc that references code through `governs:`, `related:`, or an inline path is flagged stale when that code has commits since the doc was last touched, measured as a git commit count.

## Context

The commit-distance machinery already exists: `internal/service/governs.go:annotateStaleness` (192-213) and `internal/convention/check.go:checkStale` (169-180) compute `gitx.LastCommit` of a doc and `gitx.CommitCountSince` over governed code. But both are gated on a `governs:` glob plus `status: Implemented`, so the common case (an ADR or research note that points at `internal/store/daemon.go` through `related:` or an inline path, carrying no `governs:` block and no implementation status) is never checked. This is the moat: a tool that lives in the repo can bind docs to code and see the code move; a generic notes tool cannot. The capability is the single biggest reason for Stardust to exist, and today it is mostly unreachable.

## Decision

Generalize the existing commit-distance to every code binding. Bind each doc to code through three channels: `governs:` globs (existing), `related:` targets that resolve to code (ADR 0015), and inline code-path refs (ADR 0015). For each bound code file, compute commits since the doc's last commit using `gitx.LastCommit` plus `gitx.CommitCountSince`, the same primitives in use today. Drop the `status == "Implemented"` gate for reference-derived bindings, since an ADR that points at moved code is stale regardless of an implementation status it never carries; `governs:` bindings MAY keep the Implemented gate for backward compatibility. Surface drift in `stardust check` as a `warn` of kind `drift`, in the agent manifest, and in the context bundle, naming the doc, the referenced file, and the commit count. Drift is phrased as a review prompt, never a hard failure, because a formatting-only code commit can trip it.

This decision is built last in the implementation sequence, because it depends on the edge extractor (ADR 0015) and the git-date primitives (ADR 0017).

## Consequences

- Repo docs become trustworthy: a stale doc announces itself instead of silently misleading.
- The check surface gains `warn`-level drift output that CI can ratchet on (ADR 0019) once tuned.
- Drift can over-fire on cosmetic code commits; mitigated by review phrasing and a future `--drift-since` threshold.
- Git cost scales with the number of bound files per doc; commit-distance is one `rev-list --count` per binding, batched per run.
- No code parsing or embedding; the binding is a path plus git, keeping Stardust in its markdown lane.

## Alternatives considered

- Keep drift gated on `governs:` plus `Implemented`. Rejected: it excludes the common ADR-references-code case, which is the whole point.
- Detect drift by mtime or a stored last-checked timestamp. Rejected: mtime is unreliable under cloud sync; commit-distance is exact and already available.
- Semantic drift via code embeddings. Rejected as a non-goal: commit-distance is exact, cheap, and explainable.

## References

- `internal/service/governs.go`, `internal/convention/check.go`, `internal/gitx/gitx.go`, `internal/manifest`, `internal/service/bundle.go`.
- ADR 0015 (edges), ADR 0017 (git dates), ADR 0019 (CI ratchet).
