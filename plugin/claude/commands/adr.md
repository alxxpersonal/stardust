---
description: Write one architectural decision record inline.
argument-hint: "[decision to document]"
allowed-tools: Bash, Read, Write
---

You are the ADR shorthand for the complete doc-forge workflow. Treat the doc type as `adr`
and write the ADR inline in this turn. Do not print a second slash command for the user to
run. Do not commit or push.

## Workflow

1. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace
   resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for
   a vault, point them to `/stardust:setup`.
2. If `$ARGUMENTS` is empty, ask the user to name the decision to record, then stop.
3. Use `$ARGUMENTS` as the decision topic. Run `stardust query "<decision>"` from `${ROOT}`
   when available and read any related ADRs.
4. Get the real timestamp with `date "+%Y-%m-%d-%H%M"` from `${ROOT}`. Use the date part for
   frontmatter.
5. Determine the next free zero-padded four-digit number by listing `docs/adr` and
   incrementing the highest leading number. Derive a kebab-case slug from the decision.
6. Write the ADR to `docs/adr/<NNNN>-<slug>.md`. Create `docs/adr` if needed.
7. Use YAML frontmatter:

   ```yaml
   ---
   title: <Title>
   status: Proposed
   date: <YYYY-MM-DD>
   related: [<paths>]
   ---
   ```

   Use `Accepted` only when the decision is already locked by the user or by existing
   implementation. Supersede older ADRs by referencing them in the new ADR, never by editing
   accepted records.
8. Put a one-line thesis after frontmatter. Wrap each section in collapsible markdown using
   `<details>`, `<summary><b>Section</b></summary>`, and `<br>`.
9. Include these sections: Context, Decision, Consequences, Alternatives considered,
   References.
10. Run `stardust registry` from `${ROOT}`. If Stardust is unavailable, report the skip.
11. Self-review for correct ADR number, closed-set status, no placeholders, no U+2014 or
    U+2013, no generated-by or co-author trailers, and no docs mirror folder.

Write the file directly with the available tools.
