---
title: Rules adapter sync for CLAUDE.md, AGENTS.md, and GEMINI.md
status: Implemented
version: 1
date: 2026-07-02
related:
  - docs/adr/0039-rules-adapter-sync.md
  - docs/adr/0007-stardust-composes-hooks-never-clobbers.md
  - docs/adr/0008-sentinel-block-hook-injection.md
  - docs/plans/2026-07-02-0316-rules-adapter-sync.md
  - internal/agentsync/inventory.go
  - internal/agentsync/plan.go
  - internal/agentsync/apply.go
  - internal/agentsync/config.go
  - internal/hooks/block.go
---

# Rules adapter sync for CLAUDE.md, AGENTS.md, and GEMINI.md

Author agent rules once in a canonical `.stardust/rules.md`, and have `stardust sync` render them per tool and compose them into a sentinel-delimited block inside `CLAUDE.md`, `AGENTS.md`, and `GEMINI.md`, preserving every user line outside the block.

<details>
<summary><b>Problem</b></summary>
<br>

`stardust sync` maintains skills and agents (`internal/agentsync`), but rules-adapter sync is explicitly deferred in SPEC.md section 4.2 and README line 214: "`CLAUDE.md`, `AGENTS.md`, and `GEMINI.md` need format-aware adapters, not blind symlinks."

Today a workspace with per-tool rules keeps them in sync by hand. There is no single source of truth, so the three files drift apart, and the tempting fix (symlink all three to one file) is wrong: those files hold human-authored and tool-specific content too, so seizing the whole file clobbers it, and a byte copy leaves no room for per-tool formatting.

</details>

<details>
<summary><b>Context</b></summary>
<br>

- `internal/agentsync` has two kinds: `KindSkill` (a directory with `SKILL.md`) and `KindAgent` (a `.md` file). Both are discovered by walking a `Source` folder, routed to per-tool directories (`SkillsPath` / `AgentsPath`), and materialized as symlinks or copies. `Discover` -> `[]Item`, `BuildPlan` -> `[]Action` with statuses `create` / `ok` / `drift` / `conflict`, `Apply` executes them.
- `Target` already carries per-kind path fields (`SkillsPath`, `AgentsPath`) and a `Mode` (`symlink` / `copy`). `Source` carries a `Kind` (`skill` / `agent`), a `Priority`, and `ImportOnly`. Frontmatter `targets: [claude, codex, gemini]` narrows routing per item.
- `stardust sync` supports `--dry-run`, `--check` (non-zero exit when missing or drift or conflicts), and `--repair`. The service layer (`internal/service/sync.go`) loads config, discovers, plans, and applies.
- The hooks subsystem already owns part of a shared file safely: ADR 0007 (compose, never clobber) and ADR 0008 (sentinel-block idempotent injection), implemented in `internal/hooks/block.go` with shell-comment markers, a single read-modify-write, in-place block replacement, and untouched surrounding lines.
- `Layout` (`internal/config/config.go`) exposes `.stardust` paths: `Config()`, `SyncConfig()`, `Manifest()`, `IndexMD()`. Rules needs a `Rules()` sibling.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. One canonical, committed source of rules per workspace: `.stardust/rules.md`.
2. `stardust sync` renders that body per tool and composes it into `CLAUDE.md` (claude), `AGENTS.md` (codex), and `GEMINI.md` (gemini) at the repo root.
3. Compose, never clobber: stardust owns only a sentinel-delimited block; all user content outside it survives every sync. Injection is idempotent.
4. `stardust sync --check` reports rules drift (missing block or stale block) and exits non-zero; `stardust sync` writes and updates.
5. Per-tool rendering goes through an adapter seam, so "format-aware" is structural: a future per-tool wrapper lands for one tool without touching the others.
6. No new command, no new flag, no new config file. Rules is a third `Kind` on the existing sync flow.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No global-scope rules in this pass. `DefaultConfig` wires repo-scope targets only; global `RulesPath` stays empty and skipped. Adding `~/.claude/CLAUDE.md` later is a config change, not a code change.
- No multi-section or multi-file rules composition. One canonical file, one managed block per target.
- No merge of pre-existing non-stardust rules content into the block. Users move what they want managed into `.stardust/rules.md`; everything else stays as their own lines outside the block.
- No change to skill or agent sync behavior, or to `--repair` semantics for symlink and copy targets.
- No folding of rules drift into `stardust check`. Drift is surfaced by `stardust sync --check`, consistent with skills and agents.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

One rule, inherited from ADR 0007: stardust owns only its block, never the file.

**Canonical source.** Add `Layout.Rules()` returning `<.stardust>/rules.md`. Hand-authored markdown, committed. Optional YAML frontmatter (`name`, `targets`) parsed by the existing `parseFrontmatter` / `ParseTargets`.

**New kind.** Add `KindRules Kind = "rules"`. `Source.Kind` accepts `"rules"` in `normalizeConfig`. `discoverSource` gains a `rules` branch that reads the single file at `src.Path` (a file, not a directory) and returns exactly one `Item{Kind: KindRules, Name: frontmatter name or "rules", Frontmatter, Hash, Source}`, then applies `ParseTargets`. A missing file returns no items, matching the missing-folder behavior.

**Config.** Add `RulesPath string` (`toml:"rules_path"`) to `Target`, expanded by `expandPath` like the other path fields. `DefaultConfig` adds a `repo-rules` source pointing at `Layout.Rules()` and sets `RulesPath` on the three repo targets: claude -> `CLAUDE.md`, codex -> `AGENTS.md`, gemini -> `GEMINI.md` (repo root). Global targets keep `RulesPath == ""`.

**Adapters.** New `internal/agentsync/rules.go`:

- A `rulesAdapter` per `Tool` (`fileName` plus a `render(body string) string`). A `rulesAdapters` map keys claude / codex / gemini. `render` strips the source frontmatter, normalizes trailing whitespace, and applies any tool wrapper. Bodies converge today; the map is the seam for later divergence.
- Markdown sentinel markers, a mirror of `internal/hooks/block.go` for markdown: `<!-- >>> stardust rules >>> (managed block, do not edit) -->` and `<!-- <<< stardust rules <<< -->`.
- `injectRulesBlock(path, body)` and `stripRulesBlock(path)`: read-modify-write, replace the block in place (idempotent), preserve lines outside the markers, create the file if absent (no shebang; markdown starts with the block or the preamble). Reuse the collapse-blank-runs and ensure-trailing-newline shape from the hooks pattern.

**Planning.** In `BuildPlan`, skip a `KindRules` item against a target with empty `RulesPath`. `buildAction` branches to `buildRulesAction(target, item)`:

| Target state | Status | Count |
|---|---|---|
| file missing, or block absent | `create` | missing |
| block present, equals rendered body | `ok` | - |
| block present, differs from rendered body | `drift` | drift |

`Mode` is `compose`. `Source` is the canonical file, `Target` is `RulesPath`. There is no `conflict` state for rules: composing into a user file is always safe.

**Apply.** `createTarget` gains `case "compose": composeRules(action)`, which reads `action.Source`, renders for `action.Tool`, and `injectRulesBlock`s into `action.Target`. In `applyAction`, the `drift` branch checks the mode first: `compose` drift re-injects on a plain `sync` (safe and idempotent per ADR 0007); `symlink` and `copy` drift keep the `--repair` guard unchanged.

**Surface.** No CLI change. Rules missing counts toward `Plan.Missing`, stale counts toward `Plan.Drift`, so `stardust sync --check` already fails and `stardust sync` already writes.

</details>

<details>
<summary><b>Alternatives</b></summary>
<br>

- **Symlink each target to `.stardust/rules.md`.** The explicitly rejected baseline: seizes the whole file, breaks on any human or tool write, no per-tool format.
- **Byte-copy the canonical file to each target.** Same clobber, plus no render seam.
- **A separate `stardust rules` command and `rules.toml`.** New surface for no gain; discovery, planning, `--check`, `--repair` already exist on `stardust sync`.
- **Reuse `internal/hooks/block.go` directly or extract a shared block package.** Its markers are shell comments and helpers unexported; extracting now is a refactor of working code with no second caller. Mirror the small pattern in `agentsync` with markdown markers; revisit sharing if a third consumer appears (recorded in ADR 0039).

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- A user pastes rules directly into `CLAUDE.md` outside the block, then also into `.stardust/rules.md`: duplication. Mitigation: document that managed rules live only in the canonical file; the block is the single managed region.
- A pre-existing `AGENTS.md` at the repo root with unrelated content. Mitigation: compose appends the block and leaves that content intact; no conflict state, by design.
- Marker drift between inject and strip. Mitigation: the two markers are single constants shared by both paths, tested for round-trip idempotence, mirroring the hooks block tests.
- CRLF files. Mitigation: reuse the newline handling the hooks block already exercises; test a CRLF target.
- Frontmatter leaking into the target. Mitigation: `render` strips the source frontmatter before composing; test that the block carries no `---` fence.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- Unit: `discoverRules` returns exactly one `KindRules` item from a `.stardust/rules.md`, honoring `targets` frontmatter; a missing file yields none.
- Unit: `injectRulesBlock` into a file with user lines keeps them and adds exactly one block; a second inject replaces in place (one block, idempotent); `stripRulesBlock` removes only the block and collapses blank runs.
- Unit: `render` strips frontmatter and produces per-tool bodies via the adapter map; unknown tool is a clear error.
- Plan: missing block -> `create`; matching block -> `ok`; differing block -> `drift`; a target with empty `RulesPath` is skipped; rules never produce `conflict`.
- Apply: `compose` create writes the block into a fresh `CLAUDE.md`; `compose` drift re-injects on a plain `sync` without `--repair`; symlink and copy `--repair` behavior is unchanged.
- Integration: seed `.stardust/rules.md`, run `stardust sync`, assert all three root files carry the block with user lines intact; run `stardust sync --check` and assert exit 0; edit the canonical file, assert `--check` exits non-zero, then `sync` heals it.
- Gate: `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` empty, zero U+2014 / U+2013, `stardust check` clean.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. `Layout.Rules()` and `KindRules`; `Source.Kind` accepts `rules`; `Target.RulesPath` added and expanded; `DefaultConfig` wires the source and the three repo targets.
2. `discoverRules` plus the `discoverSource` branch (single-file source).
3. `internal/agentsync/rules.go`: adapters, markdown sentinel markers, `injectRulesBlock` / `stripRulesBlock`.
4. `buildRulesAction` and the `BuildPlan` skip for empty `RulesPath`; `buildAction` branch.
5. `composeRules`; `createTarget` `compose` case; `applyAction` `compose`-drift auto-heal.
6. Tests across discover, block, plan, apply; the integration round-trip.
7. Refresh SPEC.md 4.2 and README to mark rules sync shipped; regenerate the docs index.

</details>
