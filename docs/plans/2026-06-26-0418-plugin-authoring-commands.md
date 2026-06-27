---
title: Plugin authoring commands - implementation plan
status: Draft
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-0418-plugin-authoring-commands.md
  - docs/adr/0020-authoring-commands-reference-canonical-forge-skills.md
  - docs/adr/0021-authoring-commands-delegate-never-reimplement.md
  - docs/adr/0022-docs-plans-canonical-native-plan-mirrors.md
---

# Plugin authoring commands - implementation plan

Add four thin router commands (`spec`, `plan`, `doc`, `adr`) under `plugin/claude/commands/` that resolve workspace state, surface relevant docs, and delegate to the canonical the spec workflow and the doc workflow skills, with zero embedded doc convention.

## Header

- **Goal:** the plugin exposes `/stardust:spec`, `/stardust:plan`, `/stardust:doc`, `/stardust:adr`, each a precondition-checker and router into the canonical forge skills; the produced plan is linked to native `/plan` by inheriting the spec workflow's sync.
- **Architecture:** each command is a markdown file with `allowed-tools: Bash, Read` frontmatter and a terse body that (1) resolves the workspace via `resolve-root.sh`, (2) validates and parses `$ARGUMENTS`, (3) surfaces read-only state from `docs/INDEX.md` or `docs/adr/`, (4) delegates to the named skill by a terminal handoff. No command writes a doc.
- **Tech stack:** Claude Code plugin command markdown, POSIX sh for the precondition snippet, the existing `${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh`. No new code, no new deps.
- **Global constraints:** ZERO em or en dashes anywhere. No emoji in docs. No `docs/superpowers/` or any plan mirror. Commands hold no embedded frontmatter, section list, or collapsible markup (ADR 0021). `allowed-tools` is `Bash, Read` for all four (no `Write`). Conventional commits, no co-author trailers. Follow repo CLAUDE.md and `.claude/rules`.

## Context

The plugin owns the read side of the docs loop (SessionStart injects active plans and recent specs from `docs/INDEX.md`). It has no write-side affordance. These four commands route the user into the canonical the spec workflow (spec, plan, ADRs) and the doc workflow (single doc) skills, which already own the writing discipline and the native `/plan` sync. See spec `2026-06-26-0418-plugin-authoring-commands.md` and ADRs 0020, 0021, 0022.

## Reuse map (read first)

- `plugin/claude/commands/status.md` - the canonical precondition pattern: run `resolve-root.sh`, read `MODE`/`ROOT`, stop on `none`. Copy its shape.
- `plugin/claude/commands/refresh.md` - terse body, the `MODE=none` pointer wording (repo: `stardust init --docs`; vault: `/stardust:setup`).
- `plugin/claude/commands/crons.md` - reading config and surfacing state with `jq`; frontmatter with a wider `allowed-tools`.
- `plugin/claude/commands/setup.md` - tone and structure of a longer command body.
- `plugin/claude/scripts/resolve-root.sh` - emits `MODE` and `ROOT`; never reimplement resolution.
- `plugin/claude/hooks/session-start.sh` - how the `## Plans` and `## Specs` tables in `docs/INDEX.md` are parsed (the awk extract); mirror its column reading for the surfacing steps.
- `the spec authoring workflow/SKILL.md`, `the doc authoring workflow/SKILL.md` - the canonical skills being delegated to; confirm their invocation names and argument hints in source, do not copy their bodies.
- `plugin/claude/README.md` - the existing four-command list to extend.

## Task 1: spec.md command

- Create: `plugin/claude/commands/spec.md`
- Interfaces: Consumes `$ARGUMENTS` (topic), `${CLAUDE_PLUGIN_ROOT}`, `docs/INDEX.md`. Produces a handoff naming `the spec workflow`.

- [x] Write frontmatter: `description` (verb-first, "Start a technical spec, ADRs, and implementation plan via the spec workflow."), `argument-hint: "[what to spec]"`, `allowed-tools: Bash, Read`.
- [x] Write body steps: 1) resolve workspace via `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"`, stop on `MODE=none` with the repo/vault pointer. 2) if `$ARGUMENTS` empty, ask the user to name the topic. 3) surface the last 3 rows of the `## Specs` table from `${ROOT}/docs/INDEX.md` (title, status, path). 4) delegate: state that the spec workflow explores with stardust, writes `docs/specs/`, `docs/adr/`, `docs/plans/`, runs `stardust registry`, and syncs native `/plan`; end with the exact handoff `/the spec workflow "<topic>"`.
- [x] Validate: frontmatter parses as YAML; body contains no frontmatter keys, no `<details>`, no section-name list (ADR 0021); `allowed-tools` has no `Write`.
- [ ] Commit `feat(plugin): add /stardust:spec authoring command`.

## Task 2: plan.md command

- Create: `plugin/claude/commands/plan.md`
- Interfaces: Consumes `$ARGUMENTS` (optional topic), `docs/INDEX.md` `## Plans` table. Produces either a list of active plans with paths, or a handoff naming `the spec workflow`.

- [x] Frontmatter: `description` ("List active plans from docs/plans, or start a new spec and plan via the spec workflow."), `argument-hint: "[topic to plan, or empty to list]"`, `allowed-tools: Bash, Read`.
- [x] Body steps: 1) resolve workspace, stop on `none`. 2) if `$ARGUMENTS` empty: list rows from `## Plans` in `docs/INDEX.md`, dropping `Done` and `Abandoned`, most recent first, each with path; if none active, say so and point to `/stardust:spec "<topic>"`. 3) if `$ARGUMENTS` provided: delegate to the spec workflow with the topic, noting the plan is written to canonical `docs/plans/` and native `/plan` is kept in sync (ADR 0022); end with handoff `/the spec workflow "<topic>"`.
- [x] Validate: YAML parses; listing drops settled statuses; no embedded convention; no `Write`; references `docs/plans/` as canonical, no mirror folder named.
- [ ] Commit `feat(plugin): add /stardust:plan authoring command`.

## Task 3: doc.md command

- Create: `plugin/claude/commands/doc.md`
- Interfaces: Consumes `$ARGUMENTS` (`<type> <topic>`), `docs/adr/` listing. Produces a validated handoff naming `the doc workflow`.

- [x] Frontmatter: `description` ("Add an ADR, research note, or runbook to docs/ via the doc workflow."), `argument-hint: "[adr|research|runbook] [topic]"`, `allowed-tools: Bash, Read`.
- [x] Body steps: 1) parse `$ARGUMENTS`: first token is type, rest is topic. 2) validate type against `{adr, research, runbook}`; reject otherwise and print the valid set. 3) resolve workspace, stop on `none`. 4) for `adr`, compute the next zero-padded four-digit number by listing `${ROOT}/docs/adr/` and incrementing the max, shown as a hint only (the doc workflow assigns the final number). 5) delegate to the doc workflow with `<type> <topic>`; end with handoff `/the doc workflow <type> "<topic>"`.
- [x] Validate: YAML parses; bogus type is rejected; ADR-number hint reads `docs/adr/` and does not write; no embedded convention; no `Write`.
- [ ] Commit `feat(plugin): add /stardust:doc authoring command`.

## Task 4: adr.md command

- Create: `plugin/claude/commands/adr.md`
- Interfaces: Consumes `$ARGUMENTS` (decision), `docs/adr/` listing. Produces a handoff naming `the doc workflow` with type `adr`.

- [x] Frontmatter: `description` ("Record an architectural decision as an ADR via the doc workflow."), `argument-hint: "[decision to document]"`, `allowed-tools: Bash, Read`.
- [x] Body steps: 1) resolve workspace, stop on `none`. 2) if `$ARGUMENTS` empty, ask the user to name the decision. 3) compute the next ADR number as a hint. 4) delegate to the doc workflow with `adr <decision>`; note this equals `/stardust:doc adr <decision>`; end with handoff `/the doc workflow adr "<decision>"`.
- [x] Validate: YAML parses; empty args ask for the decision; no embedded convention; no `Write`.
- [ ] Commit `feat(plugin): add /stardust:adr authoring command`.

## Task 5: surface the commands in the README

- Modify: `plugin/claude/README.md`

- [x] Add the four authoring commands to the command list next to `setup`, `status`, `refresh`, `crons`, each with a one-line purpose and its argument hint.
- [x] State that the authoring commands route into the canonical the spec workflow and the doc workflow skills and that those skills must be installed for the full write workflow (ADR 0020).
- [x] Validate: no em or en dash; the four commands and their argument hints match the frontmatter written in Tasks 1-4.
- [ ] Commit `docs(plugin): document the authoring commands in the readme`.

## Task 6: regenerate the index

- Modify: `docs/INDEX.md` (generated)

- [x] Run `stardust registry` from the repo root to regenerate `docs/INDEX.md` so the new spec, plan, and ADRs 0020-0022 appear in their tables.
- [x] Confirm the spec is under `## Specs`, the plan under `## Plans`, and ADRs 0020, 0021, 0022 under `## ADRs` with the right titles and statuses.
- [x] Do not commit unless the user asks; the post-commit hook also regenerates on the next commit.

## Verification

Run from the repo root. Adversarial: try to break routing, validation, and degradation, not just the happy path.

- [x] All four files parse: each frontmatter block is valid YAML with `description`, `argument-hint`, and `allowed-tools: Bash, Read`.
- [x] No-drift grep: none of the four command bodies contain `title:` / `status:` frontmatter keys, `<details>`, or a doc-convention section list. They contain routing and surfacing only.
- [x] No `Write` in any of the four `allowed-tools`.
- [x] Precondition gate: in a directory with no `.stardust` and no configured vault, each command resolves `MODE=none` and stops with the setup pointer, writing nothing (`git status` unchanged by the command turn).
- [x] `/stardust:doc bogus "x"` is rejected with the valid type set; `/stardust:doc adr "x"` prints the next ADR number and the `the doc workflow` handoff.
- [x] `/stardust:plan` with no args lists only non-settled plans with paths; `/stardust:spec`/`/stardust:adr` with empty args ask for the topic/decision.
- [x] No `docs/superpowers/` or any plan mirror folder was created.
- [x] `stardust registry` ran; `docs/INDEX.md` shows the new spec, plan, and ADRs 0020-0022.

## Self-review gate

- [x] Every spec requirement (four commands, precondition gate, surfacing, delegation, native-plan linkage, graceful degradation, no embedded convention) maps to a task above.
- [x] Command names, argument hints, and descriptions are consistent across the four files, the README, and this plan.
- [x] ADRs 0020, 0021, 0022 are referenced by the commands' behavior (reference canonical skills, delegate not reimplement, docs/plans canonical).
- [x] No em or en dash, no emoji, no AI-slop phrasing in any created file.
