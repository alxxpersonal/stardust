---
title: Stardust manages docs in both repos
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0144-jsonrpc-typed-contract.md
---

# Stardust manages docs in both repos

Both the stardust repo and the exo-jobs repo use `.stardust` with the docs convention to manage their docs.

## Context

The docs convention (typed `specs`, `plans`, `adr`, `research` collections plus a generated registry) is implemented in stardust. The stardust repo had collection configs but no vault `config.toml`, and exo-jobs had a `docs/` folder but no `.stardust`. Neither repo dogfooded the system it depends on.

## Decision

Run `stardust init --docs` in both repos so each carries `.stardust`, the docs collections, the post-commit hook, and a generated `docs/INDEX.md`. This contract spec, its ADRs, and its plan are written under the docs convention as the first dogfood.

## Consequences

- The decision record for this contract is queryable through stardust itself (`query`, `bundle`, `registry`).
- Both repos get the registry, stale-doc detection, and convention linting for free.
- The post-commit hook keeps each repo's `docs/INDEX.md` current.
- stardust proves out as a repo dev tool on its own and a sibling codebase.

## Alternatives considered

- Manage docs by hand: drifts, and contradicts the tool's own thesis.
- Docs in only one repo: leaves the consumer repo's docs untracked.

## References

- `stardust init --docs`, the docs convention
