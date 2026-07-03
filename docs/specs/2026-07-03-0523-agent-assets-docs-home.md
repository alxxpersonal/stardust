---
title: Agent assets live in docs/agents
status: Implemented
version: 1
date: 2026-07-03
related:
  - docs/adr/0047-agent-assets-docs-home.md
  - docs/plans/2026-07-03-0523-agent-assets-docs-home.md
  - docs/adr/0039-rules-adapter-sync.md
  - internal/agentsync/config.go
---

Shared agent assets (skills, subagents, rules) live under `docs/agents/`, stardust understands the area natively, placement alone decides sharing, and sync runs automatically through the git hooks with a CI drift gate.

<details>
<summary><b>Problem</b></summary>
<br>

agentsync ships (skills, agents, rules synced into `.claude/`, `.codex/`, `.gemini/` and the root instruction files), but its canonical sources are scattered: repo-root `skills/` and `agents/` folders plus `.stardust/rules.md`. None of them ride the doc workflow, so shared agent assets are not indexed, searched, registry-listed, or drift-tracked the way every other durable document is. Sharing intent is also invisible: nothing about a root folder says "this is the cross-agent surface". And sync is manual: forget to run `stardust sync` after a pull and the tool dirs go stale silently.
</details>

<details>
<summary><b>Context and background</b></summary>
<br>

- `internal/agentsync` resolves sources by priority with `ImportOnly` compat entries, discovers items per Kind (`KindSkill`, `KindAgent`, `KindRules`), and applies symlink/copy plans per tool target with `--check` and `--repair` (ADR 0039 added rules with sentinel-block compose).
- `checkStrayDoc` (internal/convention/check.go:105) rejects markdown under `docs/` outside registered collection folders, exempting only `docs/INDEX.md` and `docs/templates/`. Anything landed under `docs/agents/` today is a stray-doc error.
- `CheckSkills` walks the repo for `SKILL.md` files and validates their frontmatter (`name`, `targets`), so skill validation is location-independent already.
- The hooks (`internal/hooks`) run guarded `stardust index` and `stardust registry` lines on post-commit and post-merge, in owned or composed mode (ADR 0007/0008); there is no sync line.
- CI (`.github/workflows/ci.yml`) builds and tests; it runs no stardust drift gate.
- The user locked three requirements: the canonical home is `docs/agents/` with a subfolder per asset kind that stardust understands natively; sharing is decided purely by placement; sync is automatic with a CI check.
</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. `docs/agents/` is the canonical home: `docs/agents/skills/<name>/SKILL.md`, `docs/agents/subagents/<name>.md`, and `docs/agents/rules.md`, scaffolded by `stardust init --docs`.
2. stardust understands the area natively: never a stray-doc, indexed and searchable, listed in the registry, validated by the existing skill and agent checks, zero per-repo registration ceremony.
3. Placement is the sharing switch: an asset under `docs/agents/` syncs to every configured tool; an asset dropped directly into `.claude/skills/` (or any tool dir) stays tool-local and untouched. The `targets:` frontmatter remains an optional narrowing.
4. Sync runs itself: the stardust git hooks re-run `stardust sync` on post-commit and post-merge, and CI fails on drift via `stardust sync --check`.
5. Existing layouts keep working: root `skills/`, root `agents/`, and `.stardust/rules.md` remain real compat sources at lower precedence, still materialized until their files migrate; only the tool dirs are import-only.
</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- Syncing slash commands (`.claude/commands/`), MCP configs, hooks, or settings across harnesses. Deferred in ADR 0047 with triggers.
- Making `.agents/` a sync target; it stays an ImportOnly source until a harness demonstrably reads it natively.
- Changing what sync does (symlink/copy/compose mechanics, `--check`, `--repair` are untouched); this moves WHERE sources live and WHEN sync runs.
- A migration command beyond the existing ImportOnly flow.
</details>

<details>
<summary><b>Approach</b></summary>
<br>

**The area.** `docs/agents/` is a convention-known area beside the collections, mirroring how `docs/templates/` is known: it is not a registered collection (skills carry skill frontmatter, not doc frontmatter, so collection field validation must not apply). Three kind subfolders:

| Path | Kind | Item shape |
|---|---|---|
| `docs/agents/skills/<name>/` | skill | `SKILL.md` plus supporting files, existing skill frontmatter |
| `docs/agents/subagents/<name>.md` | agent | existing agent definition shape |
| `docs/agents/rules.md` | rules | the sentinel-composed rules source (ADR 0039) |

**Native understanding, four seams:**

1. `checkStrayDoc` exempts `docs/agents/` exactly as it exempts `docs/templates/`.
2. `CheckSkills` already validates any `SKILL.md` it finds; confirm coverage of the new path and extend the agent-definition validation the same way if it is folder-scoped.
3. The registry gains an "Agents" section listing skills (name, description, targets) and subagents from `docs/agents/`, generated like the collection sections.
4. `agentsync.DefaultConfig` repo sources repoint: `repo-skills` to `docs/agents/skills`, `repo-agents` to `docs/agents/subagents`, `repo-rules` to `docs/agents/rules.md`, all Priority 100. The old paths (`skills/`, `agents/`, `.stardust/rules.md`) stay real compat sources at Priority 200, still materialized so existing repos keep syncing, with the docs/agents copy winning any name collision; only the tool dirs are import-only.

Indexing and search need no work: the area is markdown under the scan root, and lifting the stray-doc error is what admits it to the healthy path. Drift tracking applies as it does to any note.

**Placement is the switch.** Tool dirs (`.claude/skills/` etc) remain ImportOnly sources: never modified by sync unless the user imports. An asset is shared by living in `docs/agents/`, local by living in a tool dir. No mandatory frontmatter; `targets:` narrows when present.

**Auto-sync.** The hook bodies (owned and composed, post-commit and post-merge) gain one guarded line after registry: `command -v stardust >/dev/null 2>&1 && stardust sync >/dev/null 2>&1 || true`. Never fails a commit, self-heals stale symlinks on every pull. CI gains a `stardust sync --check` step (build the binary in-workflow, run the gate), so drift fails CI. CI cannot create symlinks on developer machines; the hooks own the machines, CI owns the truth. `stardust init --docs` scaffolds the three subfolders with a rules.md stub, and `stardust hooks install` re-runs pick up the new body idempotently (sentinel replace).
</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Registering `docs/agents` as a normal collection: rejected, collection field validation (title, status enums) is wrong for skill frontmatter, and auto-registering per repo is ceremony the convention-known area avoids.
- Keeping root `skills/` and `agents/` as the canonical home: rejected by the user; the assets belong in the doc workflow.
- Frontmatter-based sharing opt-in: rejected, placement is simpler, visible in the tree, and needs no ceremony; `targets:` already covers refinement.
- CI creating the symlinks: rejected as mechanism confusion; CI verifies, hooks materialize.
</details>

<details>
<summary><b>Risks</b></summary>
<br>

- The sync line in hooks adds latency to every commit. Mitigated: sync on a clean tree is a no-op stat pass over a handful of paths, and the line is backgrounded-safe and guarded like the index line.
- Repos with BOTH old and new sources could double-define an item. The existing priority resolution already handles collisions (higher priority wins, ImportOnly never writes back); the plan pins this with a test.
- Skills under `docs/` get indexed into search results; that is desired (they are docs) but changes result sets slightly. Acceptable, they are legitimate content.
</details>

<details>
<summary><b>Open questions</b></summary>
<br>

- None blocking. Whether subagent definitions warrant their own registry columns beyond name/description can be settled in the build.
</details>

<details>
<summary><b>Verification</b></summary>
<br>

End to end in a temp repo: `stardust init --docs` scaffolds `docs/agents/`; a skill created at `docs/agents/skills/demo/SKILL.md` appears in `stardust check` (clean), the registry, and search; a commit (through the installed hooks) materializes symlinks in `.claude/skills/`, `.codex/skills/`, `.gemini/skills/` with no manual sync; deleting a symlink and running `stardust sync --check` exits nonzero (the CI gate works); a skill dropped only in `.claude/skills/` is never touched or replicated; a legacy repo with root `skills/` still syncs via compat. The stardust repo itself stays at check 0 errors, 0 warnings, and its own CI carries the new gate.
</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- Command, MCP, hook, and settings sync across harnesses (ADR 0047 defers with triggers).
- Global-scope (`~/`) source relocation; global `~/skills` and `~/agents` stay as they are.
</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. Convention + agentsync: the docs/agents area known natively (stray-doc exemption, registry section, init scaffold) and the DefaultConfig repoint with compat demotions.
2. Auto-sync: the hook-body sync line (owned + composed) and the CI drift gate, self-hosted in this repo's workflow.
3. README: the agent-assets story plus the full post-v0.5.0 feature sweep.
4. Adversarial review: the end-to-end verification above, run live.
</details>

<details>
<summary><b>References</b></summary>
<br>

- internal/agentsync/config.go, inventory.go, plan.go, apply.go
- internal/convention/check.go (checkStrayDoc, CheckSkills)
- internal/hooks/hooks.go (the guarded hook bodies)
- .github/workflows/ci.yml
- docs/adr/0039-rules-adapter-sync.md, docs/adr/0047-agent-assets-docs-home.md
- Reviewed 2026-07-03 against the Task 2 implementation: `internal/hooks/hooks.go` gained the guarded `stardust sync` line and `.github/workflows/ci.yml` gained the drift gate, both as specified; references hold.
</details>

## Amendments

- 2026-07-03: adversarial review caught the original wording contradiction ("ImportOnly compat" versus "existing repos keep syncing"; BuildPlan skips ImportOnly items, so a legacy-only layout silently stopped syncing). Corrected: compat sources are real, lower-precedence sources; only tool dirs are import-only. Also exempted docs/agents/ from the orphan warning (synced assets stand alone by design). Pinned by TestBuildPlanMaterializesLegacyCompatSkill and TestCheckDocsAgentsAreaIsNotOrphaned.
