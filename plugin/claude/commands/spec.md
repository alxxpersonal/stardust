---
description: Write a technical spec and its ADRs inline .
argument-hint: "[what to spec]"
allowed-tools: Bash, Read, Write, Edit, Task, WebSearch, WebFetch
---

You are `/stardust:spec`, the spec authoring command. Author the spec and its ADRs in this turn, then regenerate the registry. This command produces the design (the spec plus its ADRs); to write the executable plan use `/stardust:plan`, and to spec, plan, and build it all in one turn use `/stardust:execute`. Do not print a second slash command for the user to run.

First resolve the workspace: run `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and read the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace resolved and stop; in a docs-convention repo the user can run `stardust init --docs`, and for a vault point them to `/stardust:setup`. Run every `date`, `stardust`, and file operation from `${ROOT}`. Treat `$ARGUMENTS` as the topic to spec; if it is empty, ask the user to name the feature, topic, or decision and stop (a clarifying question, not a plan-approval gate). Then run the workflow below verbatim from `${ROOT}`, through the spec and its ADRs.

# Spec and plan workflow

Turn a non-trivial task into a technical spec and the ADRs for its locked decisions, written into the repo's `docs/` folder in the docs convention. Explore the codebase with Stardust before proposing anything, and regenerate the docs index when done. The executable plan is written by `/stardust:plan`; `/stardust:execute` does the spec, the plan, and the build together.

**This replaces Claude's native plan mode.** Use it instead of entering plan mode for non-trivial work. It produces a durable spec committed in the repo, not an ephemeral in-session plan.

## When to use

Use for non-trivial implementation work, matching the plan-mode gate:

- A new feature, a multi-file change (more than 2-3 files), an architectural decision, or work with multiple valid approaches or unclear requirements.

Do NOT use for:

- Single or few-line fixes, one function with clear requirements, very specific detailed instructions, or pure research and exploration.

**IMPORTANT:** This command writes files into the repo. Do not auto-fire it on trivial work. If the task is small or the user gave exact instructions, skip the spec and do the work directly.

## Scale to the ask

Match depth to the request in both directions. Trivial work is skipped. Ambitious work is met head on: when the user asks for breadth or rigor ("every side angle", "harden like crazy", "exhaustive"), do not fear a large spec. Enumerate every angle they asked for and the edge cases that come with it. Size that comes from coverage is correct; size that comes from padding is not. Stay dense, never truncate scope.

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

### 4. Regenerate the index

- Run `stardust registry` to regenerate `docs/INDEX.md` from the collections. With the Stardust post-commit hook installed, this also runs on every commit.
- If Stardust is not available, skip this step and say so.
- Write the documents; do not commit them unless the user asks. The registry regenerates on the user's next commit.

### 5. Self-review

Re-read the spec with fresh eyes before requesting approval, and fix inline:

- No placeholders, TBDs, or vague requirements.
- No internal contradictions.
- No requirement that could be read two ways; pick one reading and state it.
- The Work breakdown section is complete enough that `/stardust:plan` can turn it into tasks without guessing.

## Replaces native plan mode

When you would enter plan mode, use this command instead. Follow the plan-mode approval discipline:

- **Separate clarifying from approving.** Use the clarify mechanism (multiple-choice questions) only to resolve requirements or choose between approaches. Use the native approval mechanism (ExitPlanMode) to request approval. Never ask "is this okay?" or "should I proceed?" as plain text.
- **Do not reference "the plan" in clarifying questions** before the user can see it.
- **Verification is mandatory.** The spec's Verification section states how to prove the change works end to end. Be adversarial: the goal is to break it, not to confirm the happy path.
- **Explore before you design.** No proposal before the exploration in step 1.
- **Do not make large assumptions about intent.** Tie loose ends with clarifying questions before writing the spec, not during implementation.

## Voice and formatting

The spec is clean and neutral, not chat voice. It is technical but easy on the first read: a one-line thesis up top, defined sections, and scannable tables, so a reader gets the gist in 30 seconds and the depth on demand. The same document must read for a human skimming it, a teammate cold, and an agent executing it.

- Open with a one-line thesis (BLUF). No greeting, no preamble.
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

## Validation

- [ ] Explored with Stardust or grep before writing; not a duplicate of an existing spec
- [ ] Spec and any ADRs use the timestamped or numbered naming and YAML frontmatter
- [ ] Status is from the closed set; version starts at 1
- [ ] Sections collapsible, the one-line thesis visible outside
- [ ] Work breakdown is complete enough to feed `/stardust:plan`
- [ ] Approval, when sought, requested via the native mechanism (ExitPlanMode), not a text question
- [ ] No em dash, no emoji, no AI-slop in any document
- [ ] `stardust registry` was run, or its skip was noted; `docs/INDEX.md` is current
- [ ] Real timestamp from `date`, not guessed

## Operating rules

- Run every `date`, `stardust`, and file command from `${ROOT}`.
- Author the spec and its ADRs this turn, then stop. Do not print a second slash command for the user to run.
- Do not commit or push; the registry regenerates on the user's next commit.
- No em dash or en dash, no AI-slop, no emoji, no co-author or generated-by trailers.
- Never create a `docs/superpowers/` or other mirror folder.
