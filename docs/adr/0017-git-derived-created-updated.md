---
title: Git-derived created and updated dates
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-doc-code-coherence-engine.md
---

# Git-derived created and updated dates

A doc's `created` is the date of its first commit and `updated` is the date of its last commit, computed from git rather than hand-typed.

## Context

`internal/service/checkfix.go:fileDate` (176-181) backfills `created` and `updated` from `os.Stat().ModTime()`. Mtime is unreliable: cloud-synced storage rewrites it, which is the very reason Stardust keys its index off content hash, not mtime (`CLAUDE.md`, `vault.ContentHash`). Hand-typed dates rot the moment a doc is edited without touching the frontmatter. Git already holds the authoritative timeline and is already the change feed (`internal/gitx/gitx.go`, used by `digest.go` and `index.go`).

## Decision

Add two `internal/gitx/gitx.go` primitives: `FirstCommitDate(ctx, root, path)` (`git log --diff-filter=A --follow --format=%ad --date=short -- <path>`, last line) for `created`, and `LastCommitDate(ctx, root, paths...)` (`git log -1 --format=%ad --date=short -- <paths>`) for `updated`. Both return a `YYYY-MM-DD` string, empty when untracked. `checkfix.fixDocFields` derives `created` and `updated` from these when the repo tracks the file, falling back to the existing mtime path only for untracked or non-repo cases. A not-yet-committed new doc keeps its scaffold date until its first commit, after which `--fix` reconciles it.

## Consequences

- The hand-typed-date error class is eliminated at the source; dates become a derived projection of git, consistent with derive-don't-store.
- `--fix` and `check` gain git invocations per doc; `--follow` first-commit lookups are O(history) per file, so they run only in those paths and are batched and cached per invocation.
- On a non-git vault, behavior is unchanged (mtime fallback).
- Dates stay in frontmatter as committed values (so Obsidian and humans still read them); git is the authority that `--fix` reconciles them to, not a runtime replacement.

## Alternatives considered

- Keep mtime. Rejected: unreliable under cloud sync, the documented reason content-hash exists.
- Store dates in a sidecar DB. Rejected: violates files-as-truth; git already holds the timeline.
- Compute dates live on every read instead of materializing in frontmatter. Rejected: humans and Obsidian read the frontmatter; materialize-and-reconcile keeps the file self-describing.

## References

- `internal/service/checkfix.go`, `internal/gitx/gitx.go`, `internal/vault/vault.go`.
- `CLAUDE.md` (content hash over mtime).
