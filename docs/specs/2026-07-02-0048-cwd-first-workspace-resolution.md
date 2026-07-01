---
title: Cwd-first workspace resolution with walk-up
status: Implemented
version: 1
date: 2026-07-02
related:
  - docs/adr/0038-resolve-workspace-from-where-you-stand.md
  - docs/adr/0037-per-project-vault-resolution.md
  - docs/plans/2026-07-02-0048-cwd-first-workspace-resolution.md
---

The plugin resolver walks up from the current directory to find `.stardust/`, honors an env override, and falls back to the per-project vault map, so sessions resolve their workspace from where they actually stand.

<details>
<summary><b>Problem</b></summary>
<br>

`resolve-root.sh` decides repo mode with a single exact check on `$CLAUDE_PROJECT_DIR/.stardust`. Verified failures on live sessions:

- `CLAUDE_PROJECT_DIR` is unset in the Bash environment where slash commands run the resolver, so repo mode cannot fire at all from commands. A session standing in a directory that contains `.stardust/` still got `MODE=none` (observed in the monodispatch-fable-review worktree).
- There is no walk-up. A session in a subdirectory of an initialized workspace resolves to none (verified from `Stardust/internal`).
- A session whose shell has moved elsewhere loses the workspace entirely, even though its context should keep flowing.

Every hook injection and plugin command gates on this resolution, so a wrong `none` starves the session of stardust context.
</details>

<details>
<summary><b>Context</b></summary>
<br>

- The resolver prints eval-safe `MODE=` and `ROOT=` lines consumed by the SessionStart hook, the prompt-submit hook, and every command preamble. That contract must not change.
- ADR 0037 fixed vault-mode collisions with a per-project `vaults` map keyed by `CLAUDE_PROJECT_DIR`. Correct, but it inherited the same env fragility: an unset key looks up nothing.
- `STARDUST_VAULT` is the established override env var across the stardust ecosystem (the binary and exo-jobs both honor it).
- git solves the identical problem with a physical walk-up from the cwd; that behavior is well understood.
</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. A session standing in, or anywhere below, a directory containing `.stardust/` MUST resolve it as repo mode, with or without `CLAUDE_PROJECT_DIR`.
2. A session whose shell moved away from its start dir MUST still resolve the start dir's workspace when one exists there.
3. `STARDUST_VAULT` MUST override everything when set to an existing directory.
4. Vault-mode lookups MUST work from subdirectories of a mapped project via longest-prefix matching.
5. The `MODE`/`ROOT` output contract MUST stay byte-compatible; a new `SOURCE=` line MAY be appended for diagnosis.
6. ADR 0037 isolation MUST hold: an unmapped, un-walkable directory resolves to none, never to another project's vault.
</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- Auto-detecting Obsidian vaults without `.stardust/` by heuristics. Vault mode stays configuration-driven.
- Caching resolutions in plugin data.
- Changing the stardust binary's own vault resolution; this is the plugin resolver only.
</details>

<details>
<summary><b>Approach</b></summary>
<br>

Layered resolution in `resolve-root.sh`, first hit wins:

| Layer | Source | Mode | SOURCE tag |
|---|---|---|---|
| 1 | `STARDUST_VAULT` set and an existing dir | repo when it contains `.stardust/`, else vault | `env` |
| 2 | Walk up from `$PWD` to `/`, first dir with `.stardust/` | repo | `cwd` |
| 3 | Walk up from `$CLAUDE_PROJECT_DIR` when set | repo | `project` |
| 4 | `vaults` map: exact `$CLAUDE_PROJECT_DIR`, exact `$PWD`, then longest prefix of either | vault | `vault-map` |
| 5 | Legacy `vaultPath`, only when no `vaults` key exists | vault | `legacy` |
| 6 | Nothing | none | `none` |

Implementation notes:

- The walk-up uses physical paths (`cd -P`) and plain stat checks; a dozen iterations at worst.
- The mode gate for layers 4 and 5 stays as today: config `mode` is `vault` or `auto`.
- `SOURCE=` is appended after `ROOT=`; consumers reading the two documented lines are unaffected, and `eval` consumers just gain a harmless variable.
- A POSIX test script, `plugin/claude/scripts/resolve-root.test.sh`, pins every layer and the precedence between them with temp dirs and controlled env; it exits nonzero on any failure so it can gate commits.
</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Read `$PWD` only when `CLAUDE_PROJECT_DIR` is unset, no walk-up: leaves subdirectory and worktree sessions broken. Rejected.
- Vault walk-up by heuristic markers: reintroduces magic resolution that ADR 0037 removed. Rejected.
- Have Claude Code always pass the project dir: not in the plugin's control and does not fix subdirectories. Rejected.
</details>

<details>
<summary><b>Risks</b></summary>
<br>

- A stray `.stardust/` in a parent (for example `$HOME`) would capture unrelated child sessions. Accepted: identical to git semantics, and `SOURCE=cwd` makes it visible; the fix is removing or not creating stray inits.
- Precedence disagreement between the shell position and the session start dir: the shell wins by design and the ADR documents it.
- Path spaces and symlinks: the repo paths in live use contain spaces and commas; quoting and physical resolution are covered by the test script.
</details>

<details>
<summary><b>Verification</b></summary>
<br>

`resolve-root.test.sh` MUST cover, each as an isolated temp-dir case:

1. Cwd inside a `.stardust` dir, `CLAUDE_PROJECT_DIR` unset: repo, SOURCE=cwd (the screenshot failure).
2. Cwd in a nested subdirectory of that dir: repo via walk-up.
3. Cwd elsewhere, `CLAUDE_PROJECT_DIR` set to a workspace: repo, SOURCE=project.
4. `STARDUST_VAULT` set: wins over both walks, SOURCE=env.
5. Vault map exact key hit for the project dir: vault, SOURCE=vault-map.
6. Vault map longest-prefix hit from a subdirectory of a mapped project: vault.
7. Legacy `vaultPath` with no `vaults` map: vault, SOURCE=legacy.
8. Unmapped dir, nothing to walk to: none (ADR 0037 isolation preserved).
9. Precedence: cwd walk beats project walk beats vault map.
10. Paths with spaces resolve and quote correctly.

Plus: the SessionStart hook and one plugin command run against the new resolver unchanged, and the live 0.5.0 plugin cache is synced so the fix is active immediately.
</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- Migrating existing `vaults` map entries; they keep working as layer 4.
- The stardust binary's `STARDUST_VAULT` handling (already correct).
</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. Rewrite `resolve-root.sh` with the six layers and the `SOURCE=` line; author `resolve-root.test.sh` pinning all ten cases.
2. Update `setup.md`'s resolution documentation, sync the resolver and setup into the active 0.5.0 plugin cache, regenerate the docs index.
3. Adversarial review: rerun the pinned cases fresh, reproduce both original failures against the new resolver, verify ADR 0037 isolation still holds, dash scan.
</details>

<details>
<summary><b>References</b></summary>
<br>

- plugin/claude/scripts/resolve-root.sh
- plugin/claude/commands/setup.md
- docs/adr/0038-resolve-workspace-from-where-you-stand.md
- docs/adr/0037-per-project-vault-resolution.md
</details>
