---
title: Sentinel-block idempotent hook injection
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0345-hooks-compose-not-clobber.md
  - docs/adr/0007-stardust-composes-hooks-never-clobbers.md
---

# Sentinel-block idempotent hook injection

In compose mode, stardust writes its hook lines inside a sentinel-delimited block so install is idempotent and uninstall is exact.

## Context

Composing into an existing hook file (ADR 0007) requires a way to add, update, and remove stardust's lines without touching the user's lines, and without duplicating on re-run.

## Decision

stardust wraps its contribution in a fixed sentinel:

```sh
# >>> stardust >>> (managed block, do not edit)
...stardust index / registry / check lines...
# <<< stardust <<<
```

Install: if the block is absent, append it (creating the file with a `#!/bin/sh` shebang if needed); if present, replace the block in place. Uninstall: remove the block and collapse surrounding blank lines. Lines outside the markers are never read or written. The operation is a single read-modify-write of the whole file.

## Consequences

- Re-running install never duplicates lines; the block is the unit of update.
- Uninstall removes exactly stardust's lines, leaving the user's hook intact.
- The markers are greppable, so the state is auditable (`grep -l "stardust >>>"`).
- A manager that regenerates its hook drops the block; re-running install restores it (documented).

## Alternatives considered

- Append without markers: not idempotent, not removable.
- A separate stardust-owned file sourced from the user's hook: adds a moving part and a path the user must keep; the inline block is simpler and self-contained.

## References

- docs/specs/2026-06-25-0345-hooks-compose-not-clobber.md
