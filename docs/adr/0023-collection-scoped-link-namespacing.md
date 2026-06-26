---
title: Wikilinks resolve through collection-scoped names
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-doc-code-coherence-engine.md
---

# Wikilinks resolve through collection-scoped names

A wikilink may be qualified by its collection (the path-style `specs/slug` or the prefix-style `spec:slug`, written in double brackets) so the same slug under different collections is unambiguous.

## Context

`internal/vault/vault.go:NormalizeLink` (60-69) reduces a target to its lowercased basename without extension, and the graph keys notes by that basename (`graph.go:49`). When the same slug exists under `specs/`, `plans/`, and `research/` (common, since a spec and its plan share a timestamped slug), the keys collide: the graph cannot tell them apart and `check` emits duplicate-name warnings. Link resolution is therefore ambiguous exactly where the docs convention makes collisions routine.

## Decision

Introduce collection-qualified link keys. A wikilink may be written in double brackets as the path-style `specs/game-state-backend` or the prefix-style `spec:game-state-backend`; both are accepted on read, and emitted output uses path-style. The resolver keys notes by collection-qualified name and resolves an unqualified link by preferring an in-collection target, then a unique global match, then warning on genuine ambiguity. Unqualified links keep working for backward compatibility; only the disambiguation rule is new.

## Consequences

- Duplicate-name warnings caused solely by cross-collection slug reuse disappear.
- A spec and its same-slug plan are distinct graph nodes with distinct edges.
- `NormalizeLink` keeps its basename behavior for unqualified inputs; a new qualified-key path is added beside it, so existing callers are unaffected.
- Wikilink syntax gains an optional, documented qualifier; existing links need no change.

## Alternatives considered

- Disambiguate by full path only. Rejected: too verbose for everyday wikilinks; the collection qualifier is the minimal disambiguator.
- Rename slugs to be globally unique. Rejected: fights the convention, where a spec and plan intentionally share a timestamped slug.
- Silently pick the first match. Rejected: that is the current ambiguity; resolution must be deterministic and warn on real ties.

## References

- `internal/vault/vault.go`, `internal/graph/graph.go`, `internal/convention/check.go`.
