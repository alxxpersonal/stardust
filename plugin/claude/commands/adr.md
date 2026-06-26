---
description: Record an architectural decision as an ADR via doc-forge.
argument-hint: "[decision to document]"
allowed-tools: Bash, Read
---

You are the shorthand for recording one architectural decision. This is exactly
`/stardust:doc adr <decision>` with a shorter path for inline decision capture. It routes
into the canonical doc-forge skill and never writes a doc. Keep it terse.

## Steps

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace
   resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for
   a vault, point them to `/stardust:setup`.
2. If `$ARGUMENTS` is empty, ask the user to name the decision to record, then stop.
3. Compute the next free zero-padded four-digit ADR number as a convenience hint only. List
   `${ROOT}/docs/adr/`, take the highest leading number, and add one:
   `ls "${ROOT}/docs/adr/" 2>/dev/null | grep -Eo '^[0-9]{4}' | sort -n | tail -1`
   Print the result plus one, zero-padded to four digits, and state it is a hint: doc-forge
   assigns the final number at write time.
4. Delegate to doc-forge with the `adr` type and the decision. End with the exact handoff to
   run next:

   `/doc-forge adr "<decision>"`

   If doc-forge is not installed, say so and point the user at the `docs/adr/` convention
   folder so they can author by hand. Do not error.
