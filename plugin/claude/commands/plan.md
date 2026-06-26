---
description: List active plans, or write a full spec and plan inline.
argument-hint: "[topic to plan, or empty to list]"
allowed-tools: Bash, Read, Write
---

You are the plan-facing entrypoint for the complete spec-forge workflow. With a topic, write
the spec, ADRs, and canonical plan inline in this turn. With no topic, list active plans from
the resolved workspace. Do not print a second slash command for the user to run.

## Workflow

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace
   resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for
   a vault, point them to `/stardust:setup`.
2. If `$ARGUMENTS` is empty, list active plans:
   - Read `${ROOT}/docs/INDEX.md` if present.
   - Extract rows from the `## Plans` table.
   - Drop settled statuses: `Done`, `Abandoned`, `Implemented`, `Superseded`, `Archived`.
   - Print title, status, and path for each remaining plan.
   - If no active plan exists, say so and ask for a topic to plan.
   - Stop without writing files.
3. If `$ARGUMENTS` is present, use it as the topic and run the complete spec workflow:
   - Get the real timestamp with `date "+%Y-%m-%d-%H%M"` from `${ROOT}`.
   - Derive a 3 to 6 word kebab-case slug.
   - Explore before writing with `stardust query "<topic>"` and `stardust bundle "<topic>"`
     when available; otherwise use `rg` and direct file reads.
   - Read prior specs, plans, ADRs, and relevant source files.
4. Write the spec to `docs/specs/<timestamp>-<slug>.md` with YAML frontmatter:

   ```yaml
   ---
   title: <Title>
   status: Draft
   version: 1
   date: <YYYY-MM-DD>
   related: [<paths>]
   ---
   ```

   Keep a one-line thesis outside collapsibles. Use collapsible sections for Problem,
   Context and background, Goals, Non-goals, Approach, Alternatives considered, Risks, Open
   questions, Verification, Out of scope, Work breakdown, and References.
5. Write ADRs for locked decisions to `docs/adr/<NNNN>-<slug>.md`, using the next free
   zero-padded four-digit number. ADR sections are Context, Decision, Consequences,
   Alternatives considered, References.
6. Write the canonical plan to `docs/plans/<timestamp>-<slug>.md`. The plan must contain:
   Goal, Architecture, Tech Stack, Global Constraints, Context, reuse map, one bite-sized
   task per behavior, Files, Interfaces, red-green steps, validation loop, full verification,
   and self-review. Use `[ ]`, `[wip]`, `[x]`, and `[f]` markers. Never write a mirror under
   `docs/superpowers`.
7. If native plan mode is active, mirror the canonical plan state there while keeping the
   `docs/plans` file authoritative.
8. Run `stardust registry` from `${ROOT}`. If Stardust is unavailable, report the skip.
9. Self-review for placeholders, contradictions, missing plan tasks, stale related paths,
   U+2014, U+2013, and generated-by or co-author trailers.

Write the files directly with the available tools. Do not commit or push.
