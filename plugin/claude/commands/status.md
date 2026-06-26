---
description: Show the resolved stardust mode, root, and index health for this session.
argument-hint: ""
allowed-tools: Bash
---

You are reporting the stardust-plugin status for this session. Read-only. Keep it to a few
lines.

## Steps

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. Report both. If `MODE` is `none`, say no workspace
   resolved and point to `/stardust:setup` for vault mode, then stop.
2. Report whether the `stardust` binary is on PATH. If it is not, say the plugin is idle and
   stop.
3. From `ROOT`, report index health. There is no `stardust status` verb, so synthesize it:
   - Run `stardust check --output json` and report the error and warning counts.
   - If `ROOT` is a git repo, report the short HEAD with `git -C "$ROOT" rev-parse --short HEAD`.
   - Report the note count from the first line of `${ROOT}/.stardust/INDEX.md` if it exists.
4. Print the resolved config path `${CLAUDE_PLUGIN_DATA}/config.json` and whether it exists,
   so the user knows where tunables live.
