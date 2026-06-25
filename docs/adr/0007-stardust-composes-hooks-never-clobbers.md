---
title: Stardust composes hooks, never clobbers
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-0345-hooks-compose-not-clobber.md
---

# Stardust composes hooks, never clobbers

When a repo already runs hooks, stardust adds itself to the existing chain rather than seizing `core.hooksPath`.

## Context

`hooks.Install` sets `core.hooksPath` to `.stardust/hooks`. git resolves exactly one hooks path, so this silently disables any pre-existing chain: husky (`core.hooksPath=.husky`), a hand-written `.git/hooks/post-commit`, or another manager. A docs and index tool must not turn off a repo's commit automation as a side effect of `init`.

## Decision

stardust detects an existing hooks manager or existing hooks before installing. When one is present (compose mode), it appends its index and registry invocations into the existing hook file and leaves `core.hooksPath` untouched. Only when a repo has no manager and no existing hooks (owned mode) does it write `.stardust/hooks` and set `core.hooksPath`, as today.

One rule: stardust adds itself to the chain, never takes the chain over.

## Consequences

- husky, lefthook, and hand-rolled hooks keep working; stardust runs alongside them.
- Pure-Go repos with nothing installed keep the current zero-config behavior.
- `Uninstall` must become surgical: strip stardust's contribution, and only unset `core.hooksPath` when stardust set it.
- The guarded hook bodies (`command -v stardust ... || true`) already make stardust's lines safe to drop into any shell hook.

## Alternatives considered

- Keep seizing `core.hooksPath`: the bug itself.
- Relocate the other manager's hooks into `.stardust/hooks`: invasive and fragile.
- Refuse to install when a manager exists: worse ergonomics than a clean idempotent append.

## References

- `internal/hooks/hooks.go`
- git `core.hooksPath` documentation
