---
title: Single declarative per-collection schema is the one source of truth
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-doc-code-coherence-engine.md
---

# Single declarative per-collection schema is the one source of truth

The per-collection `config.toml` schema is the only place doc field rules are declared; the scaffolder, checker, and autofixer all derive from it.

## Context

Doc field rules live in three disagreeing places. `internal/service/docs.go:renderNewDoc` (99-117) emits `title, type, status, created, updated` plus optional `governs, related`. `internal/convention/check.go:checkDocFile` (line 103) demands those five via a hardcoded `[]string{...}` loop. `.stardust/collections/specs/config.toml` and `internal/cli/init.go:docCollectionConfig` (122-133) declare only `title` and `status`. A `collections.Validate(frontmatter, fields)` function already exists (`internal/collections/collections.go:130`) and enforces required-ness, enums, and types, but the checker never calls it. The tool can therefore produce docs against rules its own linter does not know about, and a developer has no single place to read what a doc must contain.

## Decision

The per-collection schema at `.stardust/collections/<name>/config.toml` is the single source of truth for doc frontmatter. `internal/convention/convention.go:DocCollection` is extended to declare the full field set (`title, type, status, created, updated` required; `governs, related` optional `ref`). `internal/cli/init.go:docCollectionConfig` codegens all of those fields from that declaration. `internal/convention/check.go:checkDocFile` loads the owning collection and validates through `collections.Validate(fm, cfg.Fields)` instead of the hardcoded loop, folding the existing type-mismatch and status-enum checks into it. `internal/service/checkfix.go` derives fixability from the schema: a required field with a derivation rule is fixable, one without (`title`, `status`) is report-only. When no collection config is present the checker falls back to the legacy hardcoded set, preserving behavior on un-scaffolded repos.

One rule: the schema is declared once and every component reads it; failing docs become structurally unrepresentable rather than merely fixed.

## Consequences

- A generated doc cannot fail its own linter, because both sides read the same schema.
- A vault owner customizes doc rules by editing one `config.toml`, with no code change.
- The checker gains a `collections.LoadOne` dependency per checked collection; results are cached per run.
- A migration path is needed for existing two-field configs; the legacy fallback plus an optional `collections sync` covers it.
- `collections.ErrValidation` already maps validation failures to the JSON-RPC domain band (ADR 0006), so the transport mapping is unchanged.

## Alternatives considered

- Sync the three sites by hand. Rejected: the divergence is structural and recurs; only a single source makes it unrepresentable.
- One global schema shared by all collections. Rejected: each collection has its own status enum and type; per-collection config keeps them distinct while sharing the field machinery.
- Keep the checker authoritative and generate config from it. Rejected: config is the file a human edits and commits, so it is the natural source; the checker is a consumer.

## References

- `internal/convention/check.go`, `internal/collections/collections.go`, `internal/cli/init.go`, `internal/convention/convention.go`, `internal/service/docs.go`, `internal/service/checkfix.go`.
- ADR 0006 (validation maps to the domain band).
