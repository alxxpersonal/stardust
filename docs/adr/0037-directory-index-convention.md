---
title: Directory indexes are an opt-in managed-block convention
status: Accepted
date: 2026-06-29
related:
  - docs/specs/2026-06-29-0229-directory-index-convention.md
  - docs/plans/2026-06-29-0229-directory-index-convention.md
  - internal/config/config.go
  - internal/service/directory_indexes.go
  - internal/service/check.go
---

# Directory indexes are an opt-in managed-block convention

## Context

Some Stardust vaults need local navigation files inside business folders. The global docs registry is still useful, but it does not replace folder-level `INDEX.md` files that summarize the contents of a specific operational directory.

Plain folder indexes create noisy checks because every file is named `INDEX.md`. They also drift unless an agent remembers to update them after each file move or addition.

## Decision

Stardust adds an opt-in `[conventions.directory_indexes]` config block. When enabled, Stardust discovers configured directory roots, maintains a generated block in each directory's index file, and leaves human-authored text outside that block intact.

The generated files are treated as convention-owned structural files for check purposes. `stardust check` suppresses duplicate-name and orphan warnings for those configured index paths, while still checking their generated block for missing or stale content.

Directory indexes remain separate from `docs/INDEX.md`. The docs registry is the canonical collection registry. Directory indexes are local folder tables.

## Consequences

- Vaults can carry many `INDEX.md` files without duplicate-name noise.
- Agents can safely maintain local folder indexes by running `stardust indexes` or `stardust registry`.
- Human notes above or below the managed block are preserved.
- The convention is explicit and disabled by default, so ordinary Stardust repos keep their current behavior.
- Orphan suppression is narrow and does not hide unrelated content quality issues.

## Alternatives considered

- Treat every `INDEX.md` as structural. Rejected because many repos have meaningful manually authored indexes outside this convention.
- Replace local indexes with only `docs/INDEX.md`. Rejected because business vaults need folder-local navigation.
- Require manual maintenance only. Rejected because it was the source of drift and duplicate warnings.
- Store directory indexes in `.stardust/` only. Rejected because users need the index next to the files it describes.

## References

- Spec: docs/specs/2026-06-29-0229-directory-index-convention.md
- Plan: docs/plans/2026-06-29-0229-directory-index-convention.md
