---
description: Add one convention-correct doc inline - an ADR, research note, or runbook (the full doc-forge skill).
argument-hint: "[adr|research|runbook] [topic]"
allowed-tools: Bash, Read, Write, Edit
---

You are `/stardust:doc`, running the complete doc-forge skill inline for one convention-correct document. Do not invoke the unrelated Microsoft `.docx` tooling. Do not print a second slash command for the user to run. Author the doc in this turn, regenerate the registry, and stop gracefully when the workspace cannot be resolved. For the ADR-only shorthand use `/stardust:adr`; for a full spec plus plan of non-trivial work, use `/stardust:spec` or `/stardust:execute` instead.

First resolve the workspace: run `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and read the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace resolved and stop; in a docs-convention repo the user can run `stardust init --docs`, and for a vault point them to `/stardust:setup`. Run every `date`, `stardust`, and file operation from `${ROOT}`. Parse `$ARGUMENTS`: the first token is the doc type (`adr`, `research`, or `runbook`), the rest is the topic. If the type is invalid, print the valid set and stop. If the topic is empty, ask the user to name it and stop. Then run the skill below verbatim from `${ROOT}`.

# Doc Forge

Add one convention-correct document to a repo's `docs/` folder: an ADR, a research note, or a runbook. The lightweight sibling of spec-forge. Where spec-forge writes a full spec plus plan plus ADRs for a whole feature, doc-forge adds a single doc and regenerates the index.

## When to use

- Recording one architectural decision (ADR).
- Capturing a point-in-time research finding or audit (research note).
- Writing an operational procedure (runbook).
- Any single quick doc that should live in `docs/` under the convention.

Do NOT use for a full feature spec plus implementation plan. Use spec-forge for that.

## Doc types

| Type | Folder | File name | Sections |
|------|--------|-----------|----------|
| ADR | `docs/adr/` | `NNNN-<slug>.md` (next number) | Context, Decision, Consequences, Alternatives considered, References |
| Research | `docs/research/` | `YYYY-MM-DD-HHMM-<slug>.md` | Question, Sources, Findings, Recommendation, Open questions, See also |
| Runbook | `docs/runbooks/` | `<slug>.md` | Purpose, Prerequisites, Steps, Rollback, References |

## Process

Do not skip steps.

### 1. Pick the type and explore

- Confirm which doc type fits. If the work is a whole feature needing a spec and a plan, stop and use spec-forge.
- `stardust query "<topic>"` to find related or superseded docs. For an ADR, check whether an existing one already covers the decision or should be superseded.
- Get the real date and time: `date "+%Y-%m-%d-%H%M"`. For an ADR, find the next zero-padded four-digit number by listing `docs/adr/`.

### 2. Write the doc

- Write to the type's folder with its file name from the table above.
- YAML frontmatter, because Stardust reads `title` and `status`:

```yaml
---
title: <Title>
status: <status>
date: <YYYY-MM-DD>
related: [<paths>]
---
```

- Use the type's sections. Wrap each section in a collapsible block (`<details><summary><b>...</b></summary>`), with the one-line thesis outside, always visible.
- For an ADR, set status `Proposed` or `Accepted`. ADRs are immutable once accepted; supersede with a new ADR, never edit one.

### 3. Regenerate the index

- Run `stardust registry` to update `docs/INDEX.md`, or rely on the commit hook. Skip if Stardust is not set up, and say so.
- Write the doc; do not commit unless the user asks.

## Status vocabularies (closed sets)

- ADR: Proposed, Accepted, Deferred, Rejected, Superseded
- Research: Active, Archived, Superseded
- Runbook: Active, Deprecated

## Voice and formatting

- One-line thesis up top, no preamble. Third person, neutral, declarative.
- Sentence-case headers, RFC 2119 for requirements, tables and lists where scannable.
- Hard bans: no em dash or en dash, no AI-slop phrases ("furthermore", "it is worth noting", "moreover"), no emoji, no co-author or generated-by commit trailers.
- Follow the repo's own `CLAUDE.md` and `.claude/rules` conventions.

## Validation

- [ ] Correct type, folder, and file name (ADR numbered, research timestamped)
- [ ] YAML frontmatter with `title` and a `status` from the closed set
- [ ] Sections collapsible, thesis visible outside
- [ ] No em dash, no emoji, no AI-slop
- [ ] `stardust registry` was run, or its skip noted
- [ ] Real timestamp from `date`; ADR number is the next free one

## Operating rules

- Run every `date`, `stardust`, and file command from `${ROOT}`.
- Author the one doc this turn, then stop. Do not print a second slash command for the user to run.
- Do not commit or push; the registry regenerates on the user's next commit.
- No em dash or en dash, no AI-slop, no emoji, no co-author or generated-by trailers.
- Never create a `docs/superpowers/` or other mirror folder.
