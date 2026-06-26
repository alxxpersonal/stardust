---
description: List active plans from docs/plans, or start a new spec and plan via spec-forge.
argument-hint: "[topic to plan, or empty to list]"
allowed-tools: Bash, Read
---

You are the read affordance over the canonical `docs/plans/` plans, and the route into
spec-forge when the user names a new topic. The canonical plan file is the source of truth;
Claude native `/plan` is its synced mirror, kept in sync by spec-forge. This command never
writes a plan. Keep it terse.

## Steps

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace
   resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for
   a vault, point them to `/stardust:setup`.
2. If `$ARGUMENTS` is empty, list the active plans. Read the `## Plans` table in
   `${ROOT}/docs/INDEX.md`, drop settled rows (`Done`, `Abandoned`, `Implemented`,
   `Superseded`, `Archived`), and print each remaining plan, most recent first, as title,
   then state, then path. Extract them with:
   `awk '/^## Plans/{s=1;n=0;next} /^## /{s=0} s&&/^\|/{n++; if(n>2) print}' "${ROOT}/docs/INDEX.md"`
   Filter the settled rows out of that output. If none remain active, say so and point the
   user to `/stardust:spec "<topic>"` to start one. The canonical files live in
   `docs/plans/`; re-hydrating one into native `/plan` is done by a follow-on spec-forge run
   or by reading the file, not by this command.
3. If `$ARGUMENTS` is provided, delegate to spec-forge with the topic. The plan is the
   deliverable; spec-forge writes it to the canonical `docs/plans/` and keeps native `/plan`
   in sync with it. End with the exact handoff to run next:

   `/spec-forge "<topic>"`

   If spec-forge is not installed, say so and point the user at the `docs/plans/` convention
   folder so they can author by hand. Do not error.
