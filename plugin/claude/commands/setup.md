---
description: Configure the stardust-plugin for an Obsidian vault or confirm repo mode, then run the first index.
argument-hint: "[vault|repo]"
allowed-tools: Bash, Read, Write
---

You are configuring the stardust-plugin. The goal is a valid `config.json` under the
plugin data directory and an initialized workspace, so that SessionStart boots loaded with
state. Be terse. Do not nag.

## Where config lives

Config is `${CLAUDE_PLUGIN_DATA}/config.json`. If `CLAUDE_PLUGIN_DATA` is unset in this
session, tell the user the plugin data directory is unavailable and stop.

## Pick the mode

Read `$ARGUMENTS`.

- `repo`, or empty when `${CLAUDE_PROJECT_DIR}/.stardust` is a directory: repo mode. Repo
  mode is zero-config. If `.stardust/` is missing in the project root, offer to run
  `stardust init --docs` there. Then write the default config below so the tunables exist.
- `vault`, or empty when no repo is detected: vault mode. Follow the vault steps.

## Vault mode steps

1. Resolve the vault path. Prefer the `STARDUST_VAULT` environment variable; if it is unset,
   ask the user for the absolute path. Do not assume a personal default.
2. Confirm the path is a directory on disk. If it is not (it may not be available yet),
   report it and stop. Do not write a config that points at a missing path.
3. If `<vault>/.stardust/` is absent, run `stardust init --docs` from the vault root.
4. Run the first index from the vault root: `stardust index`.
5. Write the config from the next section, with `vaultPath` set to the chosen path.

## Write config.json

Stardust keys each project directory to its own vault under `vaults`, so concurrent Claude
sessions in different roots never collide on one shared path. Merge an entry for the current
project, keyed by `${CLAUDE_PROJECT_DIR}`, without disturbing other projects' entries:

```sh
cfg="${CLAUDE_PLUGIN_DATA}/config.json"
[ -f "$cfg" ] || printf '{ "mode": "auto", "vaults": {} }' > "$cfg"
jq --arg p "$CLAUDE_PROJECT_DIR" --arg v "<absolute path to your vault>" \
  '.mode = (.mode // "auto") | .vaults[$p] = $v | del(.vaultPath)' \
  "$cfg" > "$cfg.tmp" && mv "$cfg.tmp" "$cfg"
```

Resulting shape:

```json
{
  "mode": "auto",
  "vaults": {
    "<this project dir>": "<absolute path to your vault>"
  },
  "digestHourLocal": 8,
  "maintenanceCron": "0 */2 * * *",
  "midConversationReminders": false
}
```

Field meanings:

- `mode`: `auto` resolves an initialized repo first, then this project's vault. Set `vault`
  to force vault mode, `repo` to ignore the vaults map.
- `vaults`: a map from project directory (`CLAUDE_PROJECT_DIR`) to the absolute vault path it
  uses. Each session resolves its own entry, so sessions never share one global path. A
  legacy top-level `vaultPath` is read only when no `vaults` map exists yet.
- `digestHourLocal`: local hour, 0 to 23, for the daily digest cron.
- `maintenanceCron`: five-field, local-time schedule for the maintenance cron.
- `midConversationReminders`: when true, the prompt-submit hook may emit one debounced
  retrieval reminder per window. Off by default.

## Confirm

Validate the file parses: `jq . "${CLAUDE_PLUGIN_DATA}/config.json"`. Tell the user a fresh
session will now boot loaded with workspace state. To arm the maintenance and digest crons,
point them to `/stardust:crons`. Cron creation is never automatic.
