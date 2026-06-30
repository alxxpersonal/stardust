---
title: Vault resolution isolation per project
status: Implemented
version: 1
date: 2026-06-30
related:
  - docs/adr/0037-per-project-vault-resolution.md
  - plugin/claude/scripts/resolve-root.sh
  - plugin/claude/commands/setup.md
---

Stardust vault mode keys the vault by project directory, so concurrent Claude sessions in different roots never collide on one global path.

<details>
<summary><b>Problem</b></summary>
<br>

`resolve-root.sh` vault mode read a single `vaultPath` from the one global `config.json` under `CLAUDE_PLUGIN_DATA`. That data dir is shared by every Claude session. Any session that did not resolve a repo (a `$CLAUDE_PROJECT_DIR/.stardust` directory) fell back to that one `vaultPath`, so all such sessions resolved to the same vault, and any `/stardust:setup` clobbered it for every other session. The user saw a StrataForge session report the MonoDispatch vault.
</details>

<details>
<summary><b>Context</b></summary>
<br>

- Resolution order: repo mode first (a `.stardust` at the project root, already per-cwd), then vault mode (the configured path), then none.
- Repo mode was never the problem; it keys off `CLAUDE_PROJECT_DIR`. Vault mode existed so stardust can use a configured Obsidian vault from a directory that is not itself a stardust repo, and that is where the single global path collided.
- `CLAUDE_PLUGIN_DATA` is one directory per plugin install, not per session.
</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. Concurrent sessions in different project directories MUST resolve independent vaults.
2. A directory with no configured vault MUST resolve to `none`, not a stale global vault.
3. Pre-migration configs (a single `vaultPath`) MUST keep working until migrated.
4. The `MODE` and `ROOT` output contract MUST be unchanged so hooks and commands need no edits.
</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- Changing repo mode, the cron commands, or the SessionStart hook output.
- Auto-detecting vaults by walking the filesystem.
- A migration command. The live config is migrated in place and `/stardust:setup` handles the rest per project.
</details>

<details>
<summary><b>Approach</b></summary>
<br>

`config.json` gains a `vaults` object mapping each `CLAUDE_PROJECT_DIR` to its absolute vault path:

```json
{ "mode": "auto", "vaults": { "<project dir>": "<vault path>" } }
```

`resolve-root.sh` vault mode:

1. Look up `.vaults[$CLAUDE_PROJECT_DIR]`. If it is an existing directory, emit `vault` with it.
2. Else, only when the config has no `vaults` key at all, read the legacy top-level `.vaultPath` (backward compatibility).
3. Else, fall through to `none`.

Once a `vaults` map exists, the legacy `vaultPath` is never read, so a migrated config has no shared global path. `/stardust:setup` merges `.vaults[$CLAUDE_PROJECT_DIR]` with jq and deletes any `vaultPath`, preserving other projects' entries.
</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Global `vaultPath` default plus per-project overrides: rejected, the default reintroduces the collision for unmapped dirs.
- Walk-up auto-detection from cwd: rejected, vault mode is for using a vault from outside it.
</details>

<details>
<summary><b>Risks</b></summary>
<br>

- A session whose project dir is not yet mapped resolves to `none` instead of the old global vault. Mitigation: this is the correct behavior, and `/stardust:setup` maps it in one step.
- A path key with spaces or commas (the MonoDispatch path has both). Mitigation: jq `--arg` and `$p` index handle arbitrary string keys; verified.
</details>

<details>
<summary><b>Verification</b></summary>
<br>

Run `resolve-root.sh` with a fixed `CLAUDE_PLUGIN_DATA` config and varied `CLAUDE_PROJECT_DIR`:

- A mapped non-repo dir resolves `MODE=vault` to its mapped path.
- An unmapped dir resolves `MODE=none` (the collision is gone).
- Two distinct mapped project dirs resolve to two distinct roots.
- A `.stardust` repo dir still resolves `MODE=repo` to itself.
- A legacy config with only `vaultPath` and no `vaults` still resolves `MODE=vault`.

All five verified. The live config was migrated and the active 0.5.0 plugin cache synced so the fix is in effect immediately.
</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- Mapping every workspace the user runs. The user runs `/stardust:setup` per vault-mode workspace; repo-mode workspaces auto-resolve.
- A plugin version bump and reinstall. The source is patched; the cache was synced manually for the live fix.
</details>

<details>
<summary><b>References</b></summary>
<br>

- plugin/claude/scripts/resolve-root.sh
- plugin/claude/commands/setup.md
- docs/adr/0037-per-project-vault-resolution.md
</details>
