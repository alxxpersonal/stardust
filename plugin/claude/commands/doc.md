---
description: Add an ADR, research note, or runbook to docs/ via doc-forge.
argument-hint: "[adr|research|runbook] [topic]"
allowed-tools: Bash, Read
---

You are routing the user into the canonical doc-forge skill, which owns the single-doc
writing discipline and the index regeneration. This command parses, validates, surfaces a
hint, and hands off. It never writes a doc. Keep it terse.

## Steps

1. Parse `$ARGUMENTS`: the first token is the doc type, the rest is the topic.
2. Validate the type against the closed set `{adr, research, runbook}`. If it is anything
   else, reject it, print the valid set (`adr`, `research`, `runbook`), and stop. If the
   topic is empty, ask the user to name it and stop.
3. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace
   resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for
   a vault, point them to `/stardust:setup`.
4. If the type is `adr`, compute the next free zero-padded four-digit number as a convenience
   hint only. List `${ROOT}/docs/adr/`, take the highest leading number, and add one:
   `ls "${ROOT}/docs/adr/" 2>/dev/null | grep -Eo '^[0-9]{4}' | sort -n | tail -1`
   Print the result plus one, zero-padded to four digits, and state it is a hint: doc-forge
   assigns the final number at write time.
5. Delegate to doc-forge with the type and topic. State that doc-forge writes the one doc to
   the right `docs/` folder and regenerates the index. End with the exact handoff to run next:

   `/doc-forge <type> "<topic>"`

   If doc-forge is not installed, say so and point the user at the matching `docs/` convention
   folder (`docs/adr/`, `docs/research/`, or `docs/runbooks/`) so they can author by hand. Do
   not error.
