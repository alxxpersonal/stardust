---
title: Codex plugin ships the slash commands directly, plus skills
status: Accepted
date: 2026-07-13
---

# 0050 - Codex plugin ships the slash commands directly, plus skills

## Context

The Claude plugin exposes its authoring workflows as slash commands (`commands/*.md`) with `description`, `argument-hint`, and `allowed-tools` frontmatter, invoked as `/stardust:spec "topic"` with `$ARGUMENTS`.

An initial reading of secondary web sources claimed Codex had removed user-defined slash commands in favor of skills, so the first draft of this decision converted the commands to Agent Skills only. Verifying against the installed `codex-cli 0.144.1` corrected that: only the standalone `~/.codex/prompts` mechanism was removed. Codex PLUGINS fully support slash commands. The official OpenAI `codex` plugin ships `commands/*.md` with `description`, `argument-hint`, `disable-model-invocation`, `allowed-tools`, and `$ARGUMENTS` substitution, auto-discovered from the plugin's `commands/` directory. Many installed community plugins do the same.

## Decision

Ship the slash commands directly in `plugin/codex/commands/<name>.md`, transferred near-verbatim from the Claude commands: keep `description`, `argument-hint`, and `$ARGUMENTS`; add `disable-model-invocation: false`; drop `allowed-tools` (its values are Claude tool names that do not map cleanly to Codex, and dropping it lets the command use Codex's default tools); and soften the few Claude-harness references in the body (ExitPlanMode, TodoWrite, native plan mode). Also ship the same workflows as Agent Skills (`skills/stardust-*/SKILL.md`) so the model can auto-invoke them by intent without the user typing a slash. Commands are the explicit user surface; skills are the automatic surface. Ship both for the core eight: `execute`, `spec`, `plan`, `doc`, `adr`, `audit`, `status`, `refresh`. Defer `setup` and `crons` (Claude-plugin-config and Claude-cron specific).

## Consequences

- Full slash-command parity with the Claude plugin: `/stardust:spec`, `/stardust:plan`, and the rest work in Codex with `$ARGUMENTS`.
- Commands and skills overlap for the same eight workflows. That is intentional: commands are explicit and typed, skills are intent-triggered. The cost is that the skills consume Codex's ~2% skills-description context budget (Codex shortens descriptions when the budget is tight); commands are lazy and cost nothing until invoked.
- `setup` and `crons` have no Codex command or skill in v1; the README covers their Codex-appropriate manual equivalents.
- Commands are auto-discovered from `commands/`, so the manifest needs no `commands` pointer.

## Alternatives considered

- Skills only (the first draft). Rejected after empirical verification: it needlessly dropped the slash-command surface Codex actually supports.
- Commands only, no skills. Reasonable and lighter on the context budget, but it loses the automatic intent-triggered invocation that skills give. Kept both.

## References

- docs/specs/2026-07-13-0102-codex-plugin.md
- docs/research/2026-07-13-0102-codex-cli-extensibility.md (see the empirical correction on plugin commands)
