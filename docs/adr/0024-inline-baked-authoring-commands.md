---
title: Plugin authoring commands bake forge workflows inline
status: Proposed
version: 1
date: 2026-06-26
supersedes:
  - docs/adr/0020-authoring-commands-reference-canonical-forge-skills.md
  - docs/adr/0021-authoring-commands-delegate-never-reimplement.md
related:
  - plugin/claude/commands/spec.md
  - plugin/claude/commands/plan.md
  - plugin/claude/commands/doc.md
  - plugin/claude/commands/adr.md
---

The plugin authoring commands now embed the complete forge workflows inline and no longer hand off to `/spec-forge` or `/doc-forge`.

<details>
<summary><b>Context</b></summary>
<br>

ADR 0020 chose command-to-skill delegation. ADR 0021 forbade inline reimplementation and removed `Write` from the commands. That made the commands thin routers, but it required a second user step and left the plugin unable to run the write workflow in a single turn.

The current requirement reverses that choice. The command files are now expected to author docs directly, including exploration, spec or doc creation, ADR writing, plan writing, registry regeneration, and self-review.

</details>

<details>
<summary><b>Decision</b></summary>
<br>

Supersede ADR 0020 and ADR 0021.

The four authoring commands embed the forge process inline:

- `/stardust:spec` and `/stardust:plan` embed the full spec-forge workflow.
- `/stardust:doc` and `/stardust:adr` embed the full doc-forge workflow.
- `/stardust:adr` defaults the doc type to `adr`.
- `allowed-tools` includes `Bash, Read, Write`.
- Command files contain no `/spec-forge` or `/doc-forge` handoff line.
- The unrelated Microsoft `.docx` `doc` skill is not referenced.

</details>

<details>
<summary><b>Consequences</b></summary>
<br>

- A command can complete the write workflow in the current turn.
- The plugin becomes responsible for keeping command bodies aligned with forge workflow changes.
- Public plugin consumers no longer need private forge skills for these commands to be useful.
- The command files are larger, but their behavior is explicit and testable with grep.

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Keep the router design from ADR 0020 and ADR 0021. Rejected because it preserves the second-step handoff.
- Invoke a skill tool from the command. Rejected because command portability depends on the command prompt itself, not a private installed skill.
- Create a separate plugin skill bundle. Rejected because the requested surface is the command files.

</details>

<details>
<summary><b>References</b></summary>
<br>

- `/Users/alxx/.claude/skills/spec-forge/SKILL.md`
- `/Users/alxx/.claude/skills/doc-forge/SKILL.md`
- `docs/specs/2026-06-26-1849-stardust-hardening.md`
- `docs/adr/0020-authoring-commands-reference-canonical-forge-skills.md`
- `docs/adr/0021-authoring-commands-delegate-never-reimplement.md`

</details>
