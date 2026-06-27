---
description: List active plans, or write an executable plan inline (the plan slice of spec-forge).
argument-hint: "[topic to plan, or empty to list]"
allowed-tools: Bash, Read, Write, Edit, Task, WebSearch, WebFetch
---

You are `/stardust:plan`, the plan slice of the spec-forge skill. With no topic, list active plans from the resolved workspace. With a topic, author the executable plan in this turn, then regenerate the registry. This command produces the plan; the spec and its ADRs come from `/stardust:spec`, and `/stardust:execute` does the spec, the plan, and the build together. Do not print a second slash command for the user to run.

First resolve the workspace: run `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and read the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace resolved and stop; in a docs-convention repo the user can run `stardust init --docs`, and for a vault point them to `/stardust:setup`. Run every `date`, `stardust`, and file operation from `${ROOT}`.

If `$ARGUMENTS` is empty, list active plans instead of writing: read `${ROOT}/docs/INDEX.md` if present, extract the rows from the `## Plans` table, drop the settled statuses (`Done`, `Abandoned`, `Implemented`, `Superseded`, `Archived`), and print the title, status, and path for each remaining plan. If `docs/INDEX.md` is missing or has no `## Plans` table, fall back to globbing `${ROOT}/docs/plans/*.md` and reading each file's YAML `title` and `status`, then apply the same settled-status filter. If none are active, say so and ask for a topic. Stop without writing files.

If `$ARGUMENTS` is present, treat it as the topic and run the skill below verbatim from `${ROOT}`.

# Spec Forge: the executable plan

Turn a spec or a topic into an executable implementation plan, written into the repo's `docs/plans/` folder in the docs convention. Explore first so the plan reuses existing code, then regenerate the docs index when done.

**This replaces Claude's native plan mode.** It produces a durable plan committed in the repo, not an ephemeral in-session plan.

## Prerequisites

- The repo follows the docs convention: a `docs/` folder with `specs/`, `plans/`, `adr/`, `research/`, `templates/`. If it is missing, scaffold it (`stardust init --docs` when Stardust is set up, or create the folders).
- Stardust is optional but recommended. Without it, fall back to grep and find, and skip the registry step.
- Follow the repo's own `CLAUDE.md` and `.claude/rules` conventions. They override these defaults where they conflict.

## Process

Do not skip steps. Do not write the plan before exploring.

### 1. Explore first (reuse over reinvent)

- Get the real date and time: run `date "+%Y-%m-%d-%H%M"`. Never guess the timestamp. Derive a 3 to 6 word kebab-case slug from the topic.
- Find the spec this plan implements: `stardust query "<topic>"` surfaces a matching spec, plan, or ADR. If a spec exists, read it and base the plan on its Approach and Work breakdown. If no spec exists, plan directly from the topic but keep the plan self-contained; and when that spec-less work is design-heavy or depends on external knowledge, recommend `/stardust:spec` or `/stardust:execute` first for the full research and design rigor, then plan from that spec.
- `stardust bundle "<task>"` assembles task-scoped context with file paths. Read the critical files it returns, and look for existing functions, utilities, and patterns to reuse rather than reinvent.
- Trace the real code paths, patterns, and constraints involved. Confirm real signatures in source, not from memory.
- When the user signals breadth or rigor ("every angle", "exhaustive", "harden", "production-grade"), scale the plan up: one bite-sized task per behavior across the full surface, never truncating to the easy 80 percent.
- If Stardust is not available, grep and find over `docs/` and the codebase.
- If a prior plan already covers this, update or supersede it instead of writing a new one.

### 2. Write the executable plan

Write the plan to ONE canonical location: `docs/plans/<YYYY-MM-DD-HHMM>-<slug>.md`. This is the source of truth, indexed by Stardust, regenerated into the registry, and the file an executing agent reads. Do not mirror it into `docs/superpowers/` or any other plans directory.

Frontmatter is YAML so Stardust reads `title` and `status` as typed columns:

```yaml
---
title: <Title>
status: Draft
version: 1
date: <YYYY-MM-DD>
related: [<the spec path and any ADRs>]
---
```

If you are working inside the harness's native plan mode (Claude Code `/plan`), surface this same plan through the native plan mechanism and keep it in sync as execution proceeds: the native `/plan` MUST track the canonical `docs/plans/` plan and its checkbox state, so the in-session plan view always reflects what has actually been done. Present it via ExitPlanMode, which reads the in-session plan file. "Native" means that session plan file, never a plugin folder.

Plan content, assuming the implementer has zero context for this codebase:

- Open with a header (Goal, Architecture, Tech Stack, and Global Constraints copied verbatim from the spec), then a Context section, then a reuse map: the files to read first, with paths.
- Break into bite-sized tasks. Each task carries Files (Create / Modify / Test), Interfaces (Consumes / Produces with exact names and types), and steps that are one action each: write the failing test, run it, implement, run it, commit.
- No placeholders. Show the actual code for new code. For existing code to integrate with, point at the file and have the implementer confirm the real signature in source, not from the plan.
- Track each step with a status marker: `[ ]` idle, `[wip]` in progress, `[x]` complete, `[f]` failed. The executing agent flips them live: the moment a step is done, go back and tick its box before moving on. Never batch the ticks.
- The plan instructs its executor to mirror its tasks into the harness todo tool (for example TodoWrite) when one exists, keep exactly one task in progress at a time, mark each complete immediately, and keep the todo tool in sync with the checkboxes.
- Each task ends with a validation loop: do not exit the task until its tests pass. If a command fails, fix the cause and re-run, looping until green.
- End every task with an independently testable deliverable, and the whole plan with a verification section and a self-review gate.
- Keep it tight. Prose is a sign of padding. Favor exact file paths and bite-sized steps over narration.

### 3. Regenerate the index

- Run `stardust registry` to regenerate `docs/INDEX.md` from the collections. With the Stardust post-commit hook installed, this also runs on every commit.
- If Stardust is not available, skip this step and say so.
- Write the plan; do not commit it unless the user asks. The registry regenerates on the user's next commit.

### 4. Self-review

Re-read the plan with fresh eyes before requesting approval, and fix inline:

- No placeholders, TBDs, or vague requirements.
- No internal contradictions; the plan matches its spec where one exists.
- Names and types are consistent across plan tasks. A function called one thing in task 3 and another in task 7 is a bug.
- Every spec requirement maps to a plan task; add any that is missing.
- The plan ends with a verification section that proves the change works end to end. Be adversarial: aim to break it, not to confirm the happy path.

## Replaces native plan mode

When you would enter plan mode, run this skill instead. Follow the plan-mode approval discipline:

- **Separate clarifying from approving.** Use the clarify mechanism (multiple-choice questions) only to resolve requirements or choose between approaches. Use the native approval mechanism (ExitPlanMode) to request plan approval. Never ask "is this plan okay?" or "should I proceed?" as plain text.
- **Do not reference "the plan" in clarifying questions** before the user can see it.
- **Verification is mandatory.** Every plan ends with how to prove the change works end to end (build, tests, run the app, MCP tools). Be adversarial: the goal is to break it, not to confirm the happy path.
- **Explore before you design.** No proposal before the exploration in step 1.
- **Do not make large assumptions about intent.** Tie loose ends with clarifying questions before writing the plan, not during implementation.

## Voice and formatting

The plan is a tickable checklist, executed with its boxes flipped (`[ ] [wip] [x] [f]`). It reads for a human skimming it, a teammate cold, and an agent executing it.

- Open with a one-line thesis (BLUF). No greeting, no preamble.
- Third person, objective, declarative. No persona, no "I" or "you".
- Sentence-case headers, two heading levels below the title at most.
- RFC 2119 requirement language: MUST, SHOULD, MAY. Reserve it for real requirements, not style.
- Tables and lists where scannable, prose where reasoning matters.
- **Hard bans:** no em dash or en dash, no AI-slop phrases ("furthermore", "it is worth noting", "moreover"), no emoji, no co-author or generated-by commit trailers.

## Versioning

- The plan carries a `version`, starting at 1, and a `status` from its closed set: Draft, Active, Done, Abandoned.
- Material changes bump `version` and append a dated amendment. Documents are never deleted.

## Validation

- [ ] Explored with Stardust or grep before writing; based on the spec when one exists
- [ ] Plan uses the timestamped naming and YAML frontmatter
- [ ] Status is from the closed set (Draft, Active, Done, Abandoned); version starts at 1
- [ ] Plan written to the canonical `docs/plans/`; no `docs/superpowers/` or other mirror folder created
- [ ] Plan tasks are bite-sized, with exact file paths, Files, Interfaces, red-green steps, and validation loops
- [ ] The plan ends with a full verification section
- [ ] No em dash, no emoji, no AI-slop
- [ ] `stardust registry` was run, or its skip was noted; `docs/INDEX.md` is current
- [ ] Real timestamp from `date`, not guessed

## Operating rules

- Run every `date`, `stardust`, and file command from `${ROOT}`.
- With no topic, list active plans and stop. With a topic, author the executable plan this turn, then stop. Do not print a second slash command for the user to run.
- Do not commit or push; the registry regenerates on the user's next commit.
- Keep the plan canonical to `docs/plans/`; never create a `docs/superpowers/` or other mirror folder.
- No em dash or en dash, no AI-slop, no emoji, no co-author or generated-by trailers.
