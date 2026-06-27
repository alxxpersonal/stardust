---
description: Spec, plan, and immediately build it - the whole spec-forge skill plus execution.
argument-hint: "[what to spec and build]"
allowed-tools: Bash, Read, Write, Edit, Glob, Grep, Task, TodoWrite, WebSearch, WebFetch
---

You are `/stardust:execute`, the complete autonomous spec-and-build command. It runs the entire spec-forge skill (write the spec, its ADRs, and the canonical plan) AND THEN immediately implements that plan in the same turn. This is the full pipeline: explore, design, plan, build. The other commands are slices of the same skill: `/stardust:spec` writes only the spec and its ADRs, `/stardust:plan` writes only the executable plan or lists active plans, and `/stardust:doc` writes a single convention doc (ADR, research note, or runbook). Do not print a second slash command for the user to run.

First resolve the workspace: run `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and read the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace resolved and stop; in a docs-convention repo the user can run `stardust init --docs`, and for a vault point them to `/stardust:setup`. Run every `date`, `stardust`, and file operation from `${ROOT}`. Treat `$ARGUMENTS` as the topic to spec and build; if it is empty, ask the user to name the feature, topic, or decision and stop (a clarifying question, not a plan-approval gate). Run the full skill below verbatim (steps 1 to 6), then run the Execute phase (step 7).

# Spec Forge

Turn a non-trivial task into a technical spec, the ADRs for its locked decisions, and an executable implementation plan, written into the repo's `docs/` folder in the docs convention. Explore the codebase with Stardust before proposing anything, and regenerate the docs index when done.

**This replaces Claude's native plan mode.** Use it instead of entering plan mode for non-trivial work. It produces a durable spec and plan committed in the repo, not an ephemeral in-session plan.

## When to use

Use for non-trivial implementation work, matching the plan-mode gate:

- A new feature, a multi-file change (more than 2-3 files), an architectural decision, or work with multiple valid approaches or unclear requirements.

Do NOT use for:

- Single or few-line fixes, one function with clear requirements, very specific detailed instructions, or pure research and exploration.

**IMPORTANT:** This skill writes files into the repo. Do not auto-fire it on trivial work. If the task is small or the user gave exact instructions, skip the spec and do the work directly.

## Scale to the ask

Match depth to the request in both directions. Trivial work is skipped. Ambitious work is met head on: when the user asks for breadth or rigor ("every side angle", "harden like crazy", "exhaustive"), do not fear a large spec or a long plan. Enumerate every angle they asked for and the edge cases that come with it, and write the full multi-task plan. Size that comes from coverage is correct; size that comes from padding is not. Stay dense, never truncate scope, and the executor runs the whole plan rather than stopping at the easy 80 percent.

## Prerequisites

- The repo follows the docs convention: a `docs/` folder with `specs/`, `plans/`, `adr/`, `research/`, `templates/`. If it is missing, scaffold it (`stardust init --docs` when Stardust is set up, or create the folders).
- Stardust is optional but recommended. With it, exploration and the index are automated. Without it, fall back to grep and find, and skip the registry step.
- Follow the repo's own `CLAUDE.md` and `.claude/rules` conventions. They override these defaults where they conflict.

## Process

Do not skip steps. Do not write the spec before exploring.

### 1. Explore first (reuse over reinvent)

- Get the real date and time: run `date "+%Y-%m-%d-%H%M"`. Never guess the timestamp.
- Search for prior work so you do not duplicate or contradict it:
  - `stardust query "<topic>"` surfaces related specs, plans, and ADRs.
  - `stardust bundle "<task>"` assembles task-scoped context with file paths.
- Read the critical files the search returns. Look for existing functions, utilities, and patterns to reuse rather than reinvent.
- Research the codebase for this task: trace the real code paths, patterns, and constraints involved, not just the prior docs.
- Read the ambition in the request. When the user signals breadth or rigor (for example "every angle", "bulletproof", "exhaustive", "harden", "production-grade"), scale research up: enumerate the full surface, dig the latest edge cases and current best practices, and cover every angle they asked for.
- When external knowledge would materially change the design (a library or API, a standard, vendor behavior, an established best practice), fan out background research subagents, one per independent question, and verify their findings against primary sources. Skip web research for purely internal or trivial work; scale research depth to the task.
- Consolidate substantial research into a `docs/research/<YYYY-MM-DD-HHMM>-<slug>.md` doc with sources cited. Reference it from the spec's Context and References sections, and distill its conclusions into the spec rather than dumping the raw research.
- For complex or high-stakes work, weigh the design from 2-3 contrasting perspectives (for example simplicity vs performance vs maintainability, or root-cause vs workaround vs prevention) before settling, and record why the chosen one won under Alternatives considered.
- If Stardust is not available, grep and find over `docs/` and the codebase.
- If a prior spec already covers this, update or supersede it instead of writing a new one.
- If the work spans multiple independent subsystems, decompose it and write one spec per subsystem rather than a single mega-spec. Spec the first piece, then the next.

### 2. Write the spec

Write to `docs/specs/<YYYY-MM-DD-HHMM>-<slug>.md`. Slug is kebab-case, 3-6 words.

Frontmatter is YAML, because Stardust collections read `title` and `status` as typed columns:

```yaml
---
title: <Title>
status: Draft
version: 1
date: <YYYY-MM-DD>
related: [<paths>]
---
```

Sections (omit one only if it genuinely has no content, never pad):

- Problem
- Context and background (the landscape, constraints, and prior research; link the research docs)
- Goals (3-5 truthy sentences describing a future state)
- Non-goals (what a reader might expect in scope but is not)
- Approach (the design: clear units with well-defined interfaces, data models, diagrams). Apply YAGNI: cut scope that is not needed
- Alternatives considered (and why each was rejected)
- Risks
- Open questions (unresolved decisions and unknowns, named honestly, never hidden)
- Verification (how to prove it works end to end)
- Out of scope
- Work breakdown (the seam that feeds the plan)
- References
- Amendments (append-only, added after the spec is first approved; omit on the first draft)

Wrap each section in a collapsible block so the spec scans fast: a reader collapses all and sees the thesis plus the section titles, then expands only what they need. The frontmatter and the one-line thesis stay outside the collapsibles, always visible. The file stays markdown, so the `<details>` tags render in GitHub, Obsidian, and browsers, and Stardust still indexes the text inside. Use this pattern per section:

```markdown
<details>
<summary><b>Problem</b></summary>
<br>

<the section content>

</details>
```

### 3. Spin ADRs for locked decisions

For each significant decision the spec makes, write an ADR to `docs/adr/<NNNN>-<slug>.md` using the next zero-padded four-digit number. Sections: Context, Decision, Consequences, Alternatives considered, References. ADRs are immutable once accepted. Supersede with a new ADR, never edit an accepted one.

### 4. Write the executable plan

Write the plan to ONE canonical location: `docs/plans/<YYYY-MM-DD-HHMM>-<slug>.md`. This is the source of truth, indexed by Stardust, regenerated into the registry, and the file an executing agent reads. Do not mirror it into `docs/superpowers/` or any other plans directory.

If you are working inside the harness's native plan mode (Claude Code `/plan`), surface this same plan through the native plan mechanism and keep it in sync as execution proceeds: the native `/plan` MUST track the canonical `docs/plans/` plan and its checkbox state, so the in-session plan view always reflects what has actually been done. Present it via ExitPlanMode, which reads the in-session plan file. "Native" means that session plan file, never a plugin folder. This native-plan sync applies only when the session was already in plan mode; the default `/stardust:execute` flow does not enter plan mode or call ExitPlanMode to gate the build (see step 7).

Plan content, assuming the implementer has zero context for this codebase:

- Open with a header (Goal, Architecture, Tech Stack, and Global Constraints copied verbatim from the spec), then a Context section, then a reuse map: the files to read first, with paths.
- Break into bite-sized tasks. Each task carries Files (Create / Modify / Test), Interfaces (Consumes / Produces with exact names and types), and steps that are one action each: write the failing test, run it, implement, run it, commit.
- No placeholders. Show the actual code for new code. For existing code to integrate with, point at the file and have the implementer confirm the real signature in source, not from the plan.
- Track each step with a status marker: `[ ]` idle, `[wip]` in progress, `[x]` complete, `[f]` failed. The executing agent flips them live: the moment a step is done, go back and tick its box before moving on. Never batch the ticks.
- The plan instructs its executor to mirror its tasks into the harness todo tool (for example TodoWrite) when one exists, keep exactly one task in progress at a time, mark each complete immediately, and keep the todo tool in sync with the checkboxes.
- Each task ends with a validation loop: do not exit the task until its tests pass. If a command fails, fix the cause and re-run, looping until green.
- End every task with an independently testable deliverable, and the whole plan with a verification section and a self-review gate.
- Keep it tight. Prose is a sign of padding. Favor exact file paths and bite-sized steps over narration.

### 5. Regenerate the index

- Run `stardust registry` to regenerate `docs/INDEX.md` from the collections. With the Stardust post-commit hook installed, this also runs on every commit.
- If Stardust is not available, skip this step and say so.
- Write the documents; do not commit them unless the user asks. The registry regenerates on the user's next commit.

### 6. Self-review

Re-read the spec and plan with fresh eyes before requesting approval, and fix inline:

- No placeholders, TBDs, or vague requirements.
- No internal contradictions; the plan matches the spec.
- No requirement that could be read two ways; pick one reading and state it.
- Names and types are consistent across plan tasks. A function called one thing in task 3 and another in task 7 is a bug.
- Every spec requirement maps to a plan task; add any that is missing.

### 7. Execute the plan

This phase is what makes `/stardust:execute` different from `/stardust:spec` and `/stardust:plan`. After the spec, ADRs, and plan are written and self-reviewed, implement the plan immediately, in this same turn, unless the user explicitly asked for the docs only.

Approval: the explicit `/stardust:execute` invocation is itself the go-ahead to build, so do not call ExitPlanMode to gate it. The ExitPlanMode approval discipline in "Replaces native plan mode" below applies only to the docs-only path (the user asked to spec or plan without building) or when the user explicitly asked to review the plan before building. In the default flow, go straight from step 6 into implementation.

- Mirror the plan tasks into the harness todo tool (TodoWrite). Keep exactly one task in progress at a time.
- Work the tasks in plan order. For each task follow its red-green steps: write the failing test, run it, implement, run it. Flip the task's checkbox in the `docs/plans/` file from `[ ]` to `[wip]` to `[x]` as you go, and never batch the ticks.
- Run each task's validation loop until green before moving to the next. If a command fails, fix the cause and re-run, looping until it passes; mark a task `[f]` only if it is genuinely blocked, and say why.
- After the last task, run the plan's whole verification section end to end (build, tests, lint, run the app or its MCP tools). Be adversarial: try to break it, not just confirm the happy path.
- Keep the spec, plan, and code consistent: if implementation forces a design change, update the spec and plan and note it in the spec's Amendments.
- Do not commit or push unless the user asks. When the build is done, report what was built, the final verification output, and any task left `[f]` with the reason.
- If the user asked only to spec or plan (not build), stop after step 6 and offer to run the execution.

## Replaces native plan mode

When you would enter plan mode, run this skill instead. Follow the plan-mode approval discipline:

- **Separate clarifying from approving.** Use the clarify mechanism (multiple-choice questions) only to resolve requirements or choose between approaches. Use the native approval mechanism (ExitPlanMode) to request plan approval. Never ask "is this plan okay?" or "should I proceed?" as plain text.
- **Do not reference "the plan" in clarifying questions** before the user can see it.
- **Verification is mandatory.** Every plan ends with how to prove the change works end to end (build, tests, run the app, MCP tools). Be adversarial: the goal is to break it, not to confirm the happy path.
- **Explore before you design.** No proposal before the exploration in step 1.
- **Do not make large assumptions about intent.** Tie loose ends with clarifying questions before writing the plan, not during implementation.

## Voice and formatting

The documents are clean and neutral, not chat voice. They are technical but easy on the first read: a one-line thesis up top, defined sections, and scannable tables, so a reader gets the gist in 30 seconds and the depth on demand. The same document must read for a human skimming it, a teammate cold, and an agent executing it.

The spec is sectioned prose, read top to bottom. The plan is a tickable checklist, executed with its boxes flipped (`[ ] [wip] [x] [f]`). Keep that split clear.

- Open each document with a one-line thesis (BLUF). No greeting, no preamble.
- Third person, objective, declarative. No persona, no "I" or "you".
- Sentence-case headers, two heading levels below the title at most.
- RFC 2119 requirement language: MUST, SHOULD, MAY. Reserve it for real requirements, not style.
- Tables and lists where scannable, prose where reasoning matters. Every section earns its place.
- **Hard bans:** no em dash or en dash, no AI-slop phrases ("furthermore", "it is worth noting", "moreover"), no emoji, no co-author or generated-by commit trailers.

## Versioning

- Each document carries a `version`, starting at 1, and a `status` from its closed set.
- Material changes bump `version` and append a dated amendment. A replaced conclusion moves the doc to `Superseded` and a new timestamped doc is created with a `supersedes` reference. Documents are never deleted.
- ADRs are immutable once `Accepted`; supersede with a new ADR.

### Status vocabularies (closed sets)

- Spec: Draft, In Review, Approved, Implemented, Superseded
- ADR: Proposed, Accepted, Deferred, Rejected, Superseded
- Plan: Draft, Active, Done, Abandoned

## Validation

- [ ] Explored with Stardust or grep before writing; not a duplicate of an existing spec
- [ ] Spec, plan, and any ADRs use the timestamped naming and YAML frontmatter
- [ ] Status is from the closed set; version starts at 1
- [ ] Plan written to the canonical `docs/plans/`; no `docs/superpowers/` or other mirror folder created
- [ ] Plan tasks are bite-sized, with exact file paths and a verification section
- [ ] Build proceeds on the /stardust:execute invocation with no ExitPlanMode gate, unless the user asked for docs only or to review first
- [ ] No em dash, no emoji, no AI-slop in any document
- [ ] `stardust registry` was run, or its skip was noted; `docs/INDEX.md` is current
- [ ] Real timestamp from `date`, not guessed
- [ ] Plan executed task by task with checkboxes flipped and validation loops green, or the docs-only stop was explicit

## Operating rules

- Run every `date`, `stardust`, and file command from `${ROOT}`.
- Author the spec, ADRs, and plan, then build the plan in the same turn. The `/stardust:execute` invocation is the go-ahead; do not gate the build behind ExitPlanMode unless the user asked for docs only or to review first.
- Do not print a second slash command for the user to run. Do not commit or push unless the user asks.
- Keep the plan canonical to `docs/plans/`; never create a `docs/superpowers/` or other mirror folder.
- No em dash or en dash, no AI-slop, no emoji, no co-author or generated-by trailers.
