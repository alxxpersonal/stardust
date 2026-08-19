---
description: Re-index the resolved stardust workspace and regenerate the docs registry.
argument-hint: ""
disable-model-invocation: false
---

You are refreshing the stardust index and registry for the resolved workspace. This is the
manual path that the maintenance cron and the commit hook automate. Keep it terse.

## Steps

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace
   resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for
   a vault, point them to `/stardust:setup` (the Stardust plugin's Codex command).
2. From `ROOT`, run `stardust index` to index changed notes.
3. From `ROOT`, run `stardust registry` to regenerate `docs/INDEX.md` from the collections.
4. Report one line: how many notes were touched and that the registry was regenerated. The
   next session will pick up the fresher state automatically at SessionStart.
