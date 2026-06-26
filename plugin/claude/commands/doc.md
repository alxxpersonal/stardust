---
description: Write one ADR, research note, or runbook inline.
argument-hint: "[adr|research|runbook] [topic]"
allowed-tools: Bash, Read, Write
---

You are running the complete doc-forge workflow inline for one convention-correct document.
Do not invoke the unrelated Microsoft `.docx` tooling. Do not print a second slash command
for the user to run. Author the doc in this turn, regenerate the registry, and stop
gracefully when the workspace cannot be resolved.

## Workflow

1. Parse `$ARGUMENTS`: the first token is the doc type, the rest is the topic. Valid types
   are `adr`, `research`, and `runbook`. If the type is invalid, print the valid set and
   stop. If the topic is empty, ask the user to name it and stop.
2. Resolve the workspace by running `sh "${CLAUDE_PLUGIN_ROOT}/scripts/resolve-root.sh"` and
   reading the `MODE` and `ROOT` lines. If `MODE` is `none`, report that no workspace
   resolved and stop. In a docs-convention repo the user can run `stardust init --docs`; for
   a vault, point them to `/stardust:setup`.
3. Explore before writing:
   - Run `stardust query "<topic>"` from `${ROOT}` when available.
   - For an ADR, inspect `docs/adr` and determine the next free zero-padded four-digit
     number.
   - Read related docs so the new document updates or supersedes rather than duplicates.
   - Get the real timestamp with `date "+%Y-%m-%d-%H%M"` from `${ROOT}`.
4. Choose the output path:
   - ADR: `docs/adr/<NNNN>-<slug>.md`
   - Research: `docs/research/<timestamp>-<slug>.md`
   - Runbook: `docs/runbooks/<slug>.md`
5. Write YAML frontmatter:

   ```yaml
   ---
   title: <Title>
   status: <status>
   date: <YYYY-MM-DD>
   related: [<paths>]
   ---
   ```

   ADR status is `Proposed` or `Accepted`. Research status is `Active`, `Archived`, or
   `Superseded`. Runbook status is `Active` or `Deprecated`.
6. Put a one-line thesis after frontmatter. Wrap every section in collapsible markdown using
   `<details>`, `<summary><b>Section</b></summary>`, and `<br>`.
7. Use the correct sections:
   - ADR: Context, Decision, Consequences, Alternatives considered, References.
   - Research: Question, Sources, Findings, Recommendation, Open questions, See also.
   - Runbook: Purpose, Prerequisites, Steps, Rollback, References.
8. Run `stardust registry` from `${ROOT}`. If Stardust is unavailable, report the skip.
9. Self-review:
   - correct type, folder, and filename
   - status from the closed set
   - no placeholders
   - no U+2014 or U+2013
   - no generated-by or co-author trailers
   - no docs mirror folder

Write the file directly with the available tools. Do not commit or push.
