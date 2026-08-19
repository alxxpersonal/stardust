---
name: stardust-refresh
description: Use when the user asks to refresh, re-index, reindex, or rebuild the stardust index, regenerate docs/INDEX.md, or run the manual stardust maintenance path after editing docs. Re-index the resolved stardust workspace and regenerate the docs registry.
---

# stardust-refresh

Refresh the stardust index and registry for the resolved workspace. This is the manual path that the maintenance cron and the commit hook automate. Keep it terse. Verification is mandatory: run each command from the resolved ROOT and confirm it succeeded before reporting.

## Steps

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for a vault, point them to the stardust-setup skill.
2. From `ROOT`, run `stardust index` to index changed notes.
3. From `ROOT`, run `stardust registry` to regenerate `docs/INDEX.md` from the collections.
4. Report one line: how many notes were touched and that the registry was regenerated. The next session will pick up the fresher state automatically at SessionStart.

Run every `date`, `stardust`, and file command from the resolved `ROOT`, not from your current directory.
