---
name: stardust-status
description: Show the resolved stardust mode, root, and index health for the current session. Use when the user asks for stardust status, whether stardust is set up here, which vault or repo root resolved, index health, error and warning counts, or where the plugin config lives.
---

# Stardust status

You are reporting the stardust-plugin status for this session. This is read-only. Keep the output to a few lines. Do not modify anything.

Run every date, stardust, git, and file command from the resolved ROOT.

## Steps

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and reading the `MODE` and `ROOT` lines. Report both. If `MODE` is `none`, say no workspace resolved and point the user to the stardust-setup skill for vault mode, then stop.
2. Report whether the `stardust` binary is on PATH. If it is not, say the plugin is idle and stop.
3. From `ROOT`, report index health. There is no `stardust status` verb, so synthesize it:
   - Run `stardust check --output json` and report the error and warning counts.
   - If `ROOT` is a git repo, report the short HEAD with `git -C "$ROOT" rev-parse --short HEAD`.
   - Report the note count from the first line of `${ROOT}/.stardust/INDEX.md` if it exists.
4. Print the resolved config path `${CLAUDE_PLUGIN_DATA}/config.json` and whether it exists, so the user knows where tunables live.

## Voice and formatting

Objective, dense, no fluff. No emoji. No em dash or en dash anywhere - use a hyphen with spaces or split the sentence. Report the facts in a few plain lines and stop.
