---
description: Start a technical spec, ADRs, and implementation plan via spec-forge.
argument-hint: "[what to spec]"
allowed-tools: Bash, Read
---

You are routing the user into the canonical spec-forge skill, which owns the spec, ADR, and
plan writing discipline. This command resolves state and hands off. It never writes a doc.
Keep it terse.

## Steps

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace
   resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for
   a vault, point them to `/stardust:setup`.
2. If `$ARGUMENTS` is empty, ask the user to name the feature, topic, or decision to spec,
   then stop. This is a clarifying question, not a plan-approval gate.
3. Surface prior art so the user can update or supersede instead of duplicating. Read up to
   the last three rows of the `## Specs` table in `${ROOT}/docs/INDEX.md` and print each as
   title, then state, then path. Extract them with:
   `awk '/^## Specs/{s=1;n=0;next} /^## /{s=0} s&&/^\|/{n++; if(n>2) print}' "${ROOT}/docs/INDEX.md" | tail -3`
   If the file or table is absent, say there is no indexed prior art and continue.
4. Delegate to spec-forge. State that spec-forge explores the codebase with stardust first,
   writes `docs/specs/`, `docs/adr/`, and `docs/plans/`, runs `stardust registry`, and keeps
   Claude native `/plan` in sync with the canonical `docs/plans/` file. End with the exact
   handoff to run next:

   `/spec-forge "<topic>"`

   If spec-forge is not installed (a plugin consumer without forge-private), say so and point
   the user at the `docs/specs/`, `docs/adr/`, and `docs/plans/` convention folders so they
   can author by hand. Do not error.
