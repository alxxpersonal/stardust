---
title: Codex plugin ships as a dedicated plugin/codex bundle
status: Accepted
date: 2026-07-13
---

# 0049 - Codex plugin ships as a dedicated plugin/codex bundle

## Context

Stardust ships `plugin/claude/`, a Claude Code plugin. Codex CLI (verified against `codex-cli 0.144.1`) reads the same `.claude-plugin/marketplace.json`, `.claude-plugin/plugin.json`, `.mcp.json`, and `hooks/hooks.json` formats, so the existing Claude bundle is partly Codex-installable already. The question is whether to make the Codex plugin a separate bundle or extend the Claude one to serve both hosts.

Two host differences force divergence. Codex removed user-defined slash commands and uses Agent Skills, so Codex needs a `skills/` directory while Claude uses `commands/`. If both lived in one bundle, Claude would load the skills and the commands (duplicate surfaces). Codex also has a native `.codex-plugin/plugin.json` with an `interface` presentation block that has no meaning to Claude. The repo already splits per surface (`plugin/claude/`, `plugin/obsidian/`).

## Decision

Ship a dedicated `plugin/codex/` bundle mirroring `plugin/claude/`, with a native `.codex-plugin/plugin.json` manifest, a `skills/` directory instead of `commands/`, and its own copies of the shared hook and resolver scripts. Do not dual-purpose `plugin/claude/`.

## Consequences

- Clean separation: each host's bundle carries only what that host loads, and each can evolve independently.
- The three shared scripts (`resolve-root.sh`, `session-start.sh`, `prompt-submit.sh`) are duplicated as byte-for-byte copies, creating drift risk. Mitigated by a header note; a later `stardust` generate step can dedupe.
- Users install the Codex bundle explicitly; there is no accidental cross-host loading.

## Alternatives considered

- Extend `plugin/claude/` to serve both hosts. Rejected: duplicate command/skill surfaces on Claude, and mixed-host manifest metadata.
- A shared core with per-host thin wrappers. Rejected as premature for two bundles; revisit if a third host lands.

## References

- docs/specs/2026-07-13-0102-codex-plugin.md
- docs/research/2026-07-13-0102-codex-cli-extensibility.md
