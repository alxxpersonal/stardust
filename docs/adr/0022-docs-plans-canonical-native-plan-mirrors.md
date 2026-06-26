---
title: docs/plans is canonical and native /plan mirrors it
status: Proposed
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-plugin-authoring-commands.md
  - plugin/claude/hooks/session-start.sh
---

# docs/plans is canonical and native /plan mirrors it

The plan file at `docs/plans/<ts>-<slug>.md` is the single source of truth. Claude native `/plan` is a synced in-session mirror of it, and `/stardust:plan` is the read affordance over it.

## Context

spec-forge writes the executable plan to one canonical location, `docs/plans/<ts>-<slug>.md`, indexed by stardust and read by an executing agent. Inside native plan mode it mirrors that plan through ExitPlanMode so the in-session checkbox state tracks the file. The plugin already reads this state: the SessionStart hook injects an `<active-plans>` block from the `## Plans` table in `docs/INDEX.md`.

The new `/stardust:plan` command adds the symmetric write-side affordance. The relationship between the canonical file, the native in-session plan, and the command needs to be stated so no second plan store appears.

## Decision

`docs/plans/<ts>-<slug>.md` is canonical. Native `/plan` is a synced view of it, owned by spec-forge, which keeps the in-session checkbox state tracking the file via ExitPlanMode. `/stardust:plan` is the read affordance: with no topic it lists active plans from the registry with their paths; with a topic it routes into spec-forge to start a new spec and plan. `/stardust:plan` MUST NOT write a plan itself, and no plan is stored anywhere outside `docs/plans/` (no `docs/superpowers/` or other mirror folder).

## Consequences

- One plan store. The SessionStart injection, `/stardust:plan`, and the native in-session plan all reflect the same canonical files.
- Native-plan checkbox state stays truthful because spec-forge syncs it from the canonical file as execution proceeds.
- The read side (hook injection) and the write side (`/stardust:plan` plus spec-forge) are symmetric over the same registry.
- The linkage depends on spec-forge's sync; a change there propagates to every consumer, which is the intended single-sourcing.

## Alternatives considered

- A separate in-session plan store maintained by the command: a second source of truth that drifts from `docs/plans/`; rejected.
- Mirroring the plan into a second folder for the native view: forbidden by spec-forge's single-canonical-location rule; rejected.

## References

- docs/specs/2026-06-26-0418-plugin-authoring-commands.md
- spec-forge SKILL.md (Replaces native plan mode)
- plugin/claude/hooks/session-start.sh (active-plans injection)
