---
title: Plugin authoring commands for the the spec workflow and the doc workflow write path
status: Superseded
version: 1
date: 2026-06-26
related:
  - plugin/claude/commands/spec.md
  - plugin/claude/commands/plan.md
  - plugin/claude/commands/doc.md
  - plugin/claude/commands/adr.md
  - plugin/claude/commands/setup.md
  - plugin/claude/commands/status.md
  - plugin/claude/scripts/resolve-root.sh
  - plugin/claude/hooks/session-start.sh
  - docs/adr/0020-authoring-commands-reference-canonical-forge-skills.md
  - docs/adr/0021-authoring-commands-delegate-never-reimplement.md
  - docs/adr/0022-docs-plans-canonical-native-plan-mirrors.md
---

# Plugin authoring commands for the the spec workflow and the doc workflow write path

Four new slash commands (`/stardust:spec`, `/stardust:plan`, `/stardust:doc`, `/stardust:adr`) close the authoring loop by routing the user into the canonical the spec workflow and the doc workflow skills to write specs, plans, ADRs, and single docs into the repo docs convention, while the plugin keeps reading that same state at session start.

<details>
<summary><b>Problem</b></summary>
<br>

The stardust plugin already owns the read side of the docs loop. `session-start.sh` injects a stardust-first policy plus a live `<workspace-state>` block (active plans, recent specs, verification counts) read from `docs/INDEX.md`. The prompt-submit hook can nudge toward retrieval. Nothing in the plugin owns the write side.

To produce a spec, plan, ADR, or single doc, the user must remember that the discipline lives in two separate skills (the spec workflow, the doc workflow) that are not surfaced by the plugin at all. The plugin advertises `/stardust:setup`, `/stardust:status`, `/stardust:refresh`, and `/stardust:crons`, but offers no command to start the authoring workflows it depends on for its own injected state. The loop is open: the plugin shows what was written but gives no affordance to write.

A naive fix (have new commands write specs and plans directly with their own embedded copy of the doc convention) would fork the convention into the command bodies. When the spec workflow or the doc workflow changes a section list, a frontmatter field, or the collapsible pattern, the command copies go stale. That is the exact no-fork, no-drift failure the canonical-skill setup exists to prevent.
</details>

<details>
<summary><b>Context and background</b></summary>
<br>

Canonical skill homes. The the spec workflow, the doc workflow, and stardust skills are canonical in a private skills repo and symlinked into the user skills dir:

- `the spec authoring workflow -> the spec authoring workflow`
- `the doc authoring workflow -> the doc authoring workflow`
- `~/.claude/skills/stardust -> a private skills repo/skills/stardust`

There is exactly one copy of each skill on disk. Any second copy is drift.

the spec workflow already owns the discipline. Per its SKILL.md it explores with stardust first, writes `docs/specs/<ts>-<slug>.md`, spins ADRs at `docs/adr/<NNNN>-<slug>.md`, writes the executable plan to the single canonical location `docs/plans/<ts>-<slug>.md`, runs `stardust registry`, and self-reviews. It explicitly replaces native plan mode and requires the native `/plan` to track the canonical `docs/plans/` plan and its checkbox state, surfaced via ExitPlanMode.

the doc workflow owns the single-doc path. It writes one ADR, research note, or runbook to the right folder with convention-correct frontmatter and collapsible sections, then regenerates the index. It defers full feature work to the spec workflow.

Existing command shape. Commands live at `plugin/claude/commands/*.md` with YAML frontmatter (`description`, `argument-hint`, `allowed-tools`) and a terse imperative body. The shared precondition across `status`, `refresh`, and `crons` is `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"`, which emits `MODE` (repo, vault, or none) and `ROOT`. `${CLAUDE_PLUGIN_DATA}` holds config, `${ARGUMENTS}` holds the invocation args.

Command tool scoping. A slash command runs as a prompt under the tool whitelist in its `allowed-tools` frontmatter. The Skill tool is not among the tools existing commands grant themselves, and the established pattern (see status, refresh, crons) is a command that resolves state and hands off, not one that performs a deep workflow inline.

Distribution asymmetry. The plugin is publishable. The forge skills are private. A public consumer of the plugin may not have the spec workflow or the doc workflow installed. The commands must degrade to a documented pointer rather than error when a named skill is absent.
</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. The plugin exposes four authoring commands that route the user into the canonical write workflows: `/stardust:spec` and `/stardust:plan` into the spec workflow, `/stardust:doc` and `/stardust:adr` into the doc workflow.
2. Each command resolves and validates workspace state before routing, and surfaces the relevant existing docs (recent specs, active plans, next ADR number) so the user starts informed.
3. The produced plan is linked to Claude native `/plan`: the canonical `docs/plans/` file is the source of truth and the native in-session plan tracks it, inherited from the spec workflow's existing sync.
4. The commands hold zero embedded copy of the doc convention. The canonical skills remain the single source of the writing discipline, so the write side cannot drift from the read side the plugin already injects.
5. The commands degrade gracefully when stardust is idle, no workspace resolves, or a forge skill is not installed, matching the quiet, no-nag posture of the existing commands and hooks.
</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No new authoring engine. The commands do not re-implement exploration, frontmatter, section layout, ADR numbering logic, index regeneration, or native-plan sync. Those stay in the skills.
- No move of the spec workflow, the doc workflow, or stardust into the plugin tree (the canonical-home restructure). That is option (b) in ADR 0020 and is explicitly rejected here.
- No change to the four existing commands, the hooks, or `resolve-root.sh` beyond reuse.
- No write of any doc by the command turn itself. All writing is performed by the delegated skill or the follow-on turn.
- No coverage of the doc-code coherence engine. That is the sibling spec; this spec only consumes a distinct ADR range to avoid a numbering collision with it.
</details>

<details>
<summary><b>Approach</b></summary>
<br>

One rule: the command is a thin router and precondition-checker; the skill owns the writing.

Files to create, all under `plugin/claude/commands/`:

| File | Command | Routes to | Role |
|------|---------|-----------|------|
| `spec.md` | `/stardust:spec [topic]` | the spec workflow | full spec, ADRs, plan |
| `plan.md` | `/stardust:plan [topic]` | the spec workflow (or list) | list active plans, or start a spec+plan |
| `doc.md` | `/stardust:doc [adr\|research\|runbook] [topic]` | the doc workflow | one convention-correct doc |
| `adr.md` | `/stardust:adr [decision]` | the doc workflow | ADR shorthand |

Shared frontmatter for all four: `allowed-tools: Bash, Read`. No `Write`. The command turn never writes a doc, so it never needs Write, and never embeds the convention.

Shared precondition (step 1 of every command body):

```sh
sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"
```

Read `MODE` and `ROOT`. If `MODE` is `none`, the command reports that no workspace resolved and stops, pointing to `stardust init --docs` for a repo or `/stardust:setup` for a vault. This reuses the exact pattern in `status.md` and `refresh.md`.

Delegation, not invocation. After preconditions, the command delegates to the named canonical skill. The normative mechanism is a terminal handoff: the command's final output is the exact skill invocation to run next, for example `/the spec workflow "<topic>"` or `/the doc workflow adr "<decision>"`. If a future harness exposes the Skill tool to commands, a command MAY add `Skill` to its `allowed-tools` and invoke the canonical skill in the same turn; the handoff remains the fallback. Either way the canonical skill runs with its full, unmodified discipline. The command MUST NOT reproduce the skill's writing steps inline (ADR 0021).

Per-command behavior.

`/stardust:spec [topic]`:
1. Resolve workspace; stop on `none`.
2. If `$ARGUMENTS` is empty, ask the user to name the feature, topic, or decision (a clarifying question, not a plan-approval question).
3. Surface prior art: list the most recent specs from the `## Specs` table in `docs/INDEX.md` (title, status, path), so the user can update or supersede instead of duplicating.
4. Delegate to the spec workflow with the topic. The command states that the spec workflow explores with stardust, writes `docs/specs/`, `docs/adr/`, and `docs/plans/`, runs `stardust registry`, and keeps native `/plan` in sync.

`/stardust:plan [topic]`:
1. Resolve workspace; stop on `none`.
2. If `$ARGUMENTS` is empty: list active plans from the `## Plans` table in `docs/INDEX.md`, dropping settled statuses (`Done`, `Abandoned`), most recent first, each with its path. If none are active, say so and point to `/stardust:spec "<topic>"` to start one. The command surfaces paths; re-hydrating a plan into the native `/plan` view is done by a follow-on the spec workflow run or the agent reading the file, not by the command turn.
3. If `$ARGUMENTS` is provided: delegate to the spec workflow with the topic. The plan is the deliverable; the spec workflow writes it to the canonical `docs/plans/` and syncs native `/plan` (ADR 0022).

`/stardust:doc [adr|research|runbook] [topic]`:
1. Parse `$ARGUMENTS`: first token is the type, the rest is the topic.
2. Validate the type against the closed set `{adr, research, runbook}`. Reject anything else and print the valid set.
3. Resolve workspace; stop on `none`.
4. For `adr`, compute and show the next free zero-padded four-digit number by listing `docs/adr/` and incrementing the max, as a convenience hint only. the doc workflow remains the authority that assigns the final number.
5. Delegate to the doc workflow with `<type> <topic>`.

`/stardust:adr [decision]`:
1. Resolve workspace; stop on `none`.
2. If `$ARGUMENTS` is empty, ask the user to name the decision.
3. Compute the next ADR number as a hint.
4. Delegate to the doc workflow with `adr <decision>`. This is exactly `/stardust:doc adr <decision>` with a shorter path for inline decision capture.

Native `/plan` linkage. The link is inherited, not re-built. the spec workflow already writes the canonical plan to `docs/plans/` and, inside native plan mode, mirrors it through ExitPlanMode so the in-session checkbox state tracks the file. `/stardust:spec` and `/stardust:plan` route into the spec workflow, so they inherit that sync for free. `/stardust:plan` with no topic is the read affordance over `docs/plans/`, the symmetric write-side counterpart to the `<active-plans>` block the SessionStart hook already injects. ADR 0022 records that `docs/plans/` is canonical and native `/plan` is its mirror.

No drift. The commands name the canonical skills that live in a private skills repo and are symlinked into the user skills dir (ADR 0020, option a). The plugin ships no copy of any forge skill. There is one writing discipline, in the skill, read by both the command-driven write path and the hook-driven read path.

Graceful degradation. When `MODE` is `none`, the command stops with the setup pointer. When the stardust binary is absent, the surfacing steps that shell out (registry reads) degrade to a note, matching `status.md`. When a named forge skill is not installed (a public plugin consumer without a private skills repo), the handoff names the skill and additionally points at the docs convention folders so the user can author by hand. The commands never error loudly and never nag.
</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

Commands write docs directly (thick commands). Each command embeds the doc convention (frontmatter, sections, collapsibles, ADR numbering) and writes with the Write tool. Rejected: this forks the convention into four command bodies that drift from the spec workflow and the doc workflow the moment either skill changes. It is the precise failure the canonical-skill setup prevents.

Make the plugin the canonical home for the trio (option b). Move the spec workflow, stardust, and the doc workflow into `plugin/claude/skills/` and have a private skills repo symlink to the plugin. Rejected as the default: it couples the private skill home to the plugin's release cadence, risks shipping private skills inside a publishable plugin, and is a large restructure for no gain over referencing. Reconsider only if a deliberate canonical-home consolidation is chosen (ADR 0020).

A single `/stardust:author` command with a mode flag. One command dispatching spec, plan, doc, and adr by a leading keyword. Rejected: it buries discoverability (four named commands show up in the command list, one polymorphic command does not) and complicates argument parsing for no real saving.

Auto-fire a skill from the SessionStart hook. Rejected: violates the no-nag posture and the skill discipline (the spec workflow must not auto-fire on trivial work). The write path is user-initiated by design.
</details>

<details>
<summary><b>Risks</b></summary>
<br>

- Forge skills absent for a public plugin user. The full workflow needs the spec workflow and the doc workflow, which are private. Mitigation: the handoff degrades to naming the docs convention folders; the read side (SessionStart injection) keeps working regardless.
- Drift creeps back if a future maintainer adds inline writing to a command for convenience. Mitigation: ADR 0021 states the rule; the `allowed-tools` of all four commands omit Write, making inline authoring a visible, reviewable change.
- Native-plan sync depends entirely on the spec workflow. If the spec workflow changes how it syncs ExitPlanMode, the commands inherit the change silently. Acceptable: that is the point of single sourcing, and it is recorded in ADR 0022.
- ADR number races. Two authors writing ADRs concurrently can collide on the next number. This spec mitigates by taking the distinct 0020-0022 range, leaving 0014-0019 for the sibling coherence spec; the doc workflow remains the final number authority at write time.
</details>

<details>
<summary><b>Open questions</b></summary>
<br>

- Does the current Claude Code harness let a slash command invoke the Skill tool in the same turn, or is the named handoff the only mechanism today? The spec treats the handoff as normative and the in-turn Skill call as an optional enhancement; this should be confirmed against the running harness before implementation hardens either path.
- Should `/stardust:plan` with no topic re-hydrate the most recent active plan into native `/plan` automatically, or only list? This spec lists by default and leaves re-hydration to a follow-on. Revisit if users want one-step resume.
- Should `/stardust:doc` accept a fourth type later (for example `spec` as a thin alias that forwards to `/stardust:spec`)? Out of scope for v1.
</details>

<details>
<summary><b>Verification</b></summary>
<br>

Adversarial, end to end. The goal is to break the routing and the degradation, not to confirm the happy path.

1. Command discovery. After install, all four commands appear with their descriptions in the command list. Each frontmatter parses (`description`, `argument-hint`, `allowed-tools: Bash, Read`).
2. Precondition gate. In a directory with no `.stardust` and no configured vault, each command resolves `MODE=none` and stops with the setup pointer, writing nothing.
3. Repo happy path. In this repo (repo mode), `/stardust:spec "demo topic"` surfaces recent specs and ends by naming the the spec workflow invocation. `/stardust:doc adr "demo decision"` validates the type, prints the next ADR number, and names the the doc workflow invocation. Neither command writes a file itself (confirm `git status` is unchanged by the command turn).
4. Argument validation. `/stardust:doc bogus "x"` is rejected with the valid type set. `/stardust:spec` and `/stardust:adr` with empty args ask for the topic or decision rather than proceeding.
5. Plan listing. `/stardust:plan` with no args lists only non-settled plans from `docs/INDEX.md` with paths; with a fresh index containing only `Done` plans, it reports none active and points to `/stardust:spec`.
6. Native-plan linkage. Running the named the spec workflow workflow inside native plan mode produces a `docs/plans/<ts>-<slug>.md` whose checkbox state is mirrored in the in-session plan view via ExitPlanMode.
7. No-drift check. Grep the four command bodies for frontmatter keys, section names, or collapsible markup. None are present; the commands contain routing and surfacing only.
8. Degradation. With the stardust binary off PATH, the surfacing steps degrade to a note and the command still names the correct handoff.
</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- The doc-code coherence engine (sibling spec).
- Any change to the spec workflow, the doc workflow, or stardust skill internals.
- Publishing or packaging the forge skills with the plugin.
- A non-Claude-Code surface (API or CLI wrapper) for the same commands; the bodies are written terse enough to reuse later, but that surface is not built here.
</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. `plugin/claude/commands/spec.md`: route to the spec workflow, surface recent specs.
2. `plugin/claude/commands/plan.md`: list active plans or route to the spec workflow.
3. `plugin/claude/commands/doc.md`: parse and validate type, route to the doc workflow, hint next ADR number.
4. `plugin/claude/commands/adr.md`: ADR shorthand over the doc workflow.
5. Three ADRs: 0020 (reference the canonical skills, reject canonical-home), 0021 (delegate, never reimplement), 0022 (docs/plans canonical, native /plan mirrors).
6. Update `plugin/claude/README.md` to list the four authoring commands alongside the existing four.
7. Regenerate `docs/INDEX.md` with `stardust registry`.
</details>

<details>
<summary><b>References</b></summary>
<br>

- the spec workflow SKILL.md (sections: Process, Replaces native plan mode, Voice and formatting)
- the doc workflow SKILL.md (Doc types, Process)
- `plugin/claude/commands/status.md`, `refresh.md`, `crons.md`, `setup.md`
- `plugin/claude/scripts/resolve-root.sh`
- `plugin/claude/hooks/session-start.sh`, `policy.txt`
- ADR 0005 (Stardust manages docs in both repos), ADR 0007 (Stardust composes hooks, never clobbers): prior single-source-of-truth and compose-not-clobber precedents
</details>
