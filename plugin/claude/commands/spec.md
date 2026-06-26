---
description: Write a technical spec, ADRs, and implementation plan inline.
argument-hint: "[what to spec]"
allowed-tools: Bash, Read, Write
---

You are running the complete spec-forge workflow inline for the resolved Stardust
workspace. Do not print a second slash command for the user to run. Author the docs in this
turn, regenerate the registry, and stop gracefully when the workspace cannot be resolved.

## Workflow

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace
   resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for
   a vault, point them to `/stardust:setup`.
2. If `$ARGUMENTS` is empty, ask the user to name the feature, topic, or decision to spec,
   then stop. This is a clarifying question, not a plan approval gate.
3. Use `$ARGUMENTS` as the topic. Get the real timestamp with
   `date "+%Y-%m-%d-%H%M"` from `${ROOT}`. Derive a 3 to 6 word kebab-case slug from the
   topic.
4. Explore before writing:
   - Run `stardust query "<topic>"` from `${ROOT}` when available.
   - Run `stardust bundle "<topic>"` from `${ROOT}` when available.
   - Read the critical files returned by those commands.
   - If Stardust is unavailable, use `rg` and direct file reads instead.
   - Check prior specs, plans, and ADRs so the new doc updates or supersedes rather than
     duplicates.
5. Write the spec to `docs/specs/<timestamp>-<slug>.md`. Create `docs/specs` if needed.
   Frontmatter must be YAML:

   ```yaml
   ---
   title: <Title>
   status: Draft
   version: 1
   date: <YYYY-MM-DD>
   related: [<paths>]
   ---
   ```

   Put a one-line thesis after frontmatter. Wrap each section in this collapsible pattern:

   ```markdown
   <details>
   <summary><b>Problem</b></summary>
   <br>

   <content>

   </details>
   ```

   Include these sections when they have real content: Problem, Context and background,
   Goals, Non-goals, Approach, Alternatives considered, Risks, Open questions, Verification,
   Out of scope, Work breakdown, References. Do not pad empty sections.
6. Write ADRs for locked decisions to `docs/adr/<NNNN>-<slug>.md`, using the next free
   zero-padded four-digit number in `docs/adr`. ADR sections are Context, Decision,
   Consequences, Alternatives considered, References. ADR status is `Proposed` or
   `Accepted`. Supersede older ADRs by referencing them in the new ADR, never by editing an
   accepted record.
7. Write the executable plan to `docs/plans/<timestamp>-<slug>.md`. Create `docs/plans` if
   needed. Use YAML frontmatter with title, status, version, date, and related. The plan must
   be canonical in `docs/plans`; never write `docs/superpowers` or another mirror folder.
   The body must include Goal, Architecture, Tech Stack, Global Constraints, Context, a reuse
   map, bite-sized tasks, Files, Interfaces, red-green steps, validation loops, full
   verification, and a self-review gate. Use `[ ]`, `[wip]`, `[x]`, and `[f]` markers.
8. If working inside native plan mode, keep the native plan view synced to the canonical
   `docs/plans` file. The file remains the source of truth.
9. Run `stardust registry` from `${ROOT}`. If Stardust is unavailable, say the registry step
   was skipped and why.
10. Self-review the written docs:
    - no placeholders, TBDs, or vague requirements
    - no internal contradictions
    - every requirement maps to a plan task
    - no U+2014 or U+2013
    - no `docs/superpowers`
    - no generated-by or co-author trailers

Write the files directly with the available tools. Do not commit or push.
