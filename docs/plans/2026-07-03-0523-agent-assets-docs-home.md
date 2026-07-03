---
title: Agent assets docs home - implementation plan
status: Active
version: 1
date: 2026-07-03
related:
  - docs/specs/2026-07-03-0523-agent-assets-docs-home.md
  - docs/adr/0047-agent-assets-docs-home.md
---

Make `docs/agents/` the natively understood canonical home for shared agent assets, wire automatic sync through the hooks and a CI drift gate, and land the README story.

## Header

- **Goal:** one docs-homed cross-agent surface, shared by placement, synced without manual steps.
- **Architecture:** a convention-known `docs/agents/` area (skills, subagents, rules.md) feeding the existing agentsync pipeline; hook bodies gain a guarded sync line; CI gains `sync --check`.
- **Tech stack:** Go 1.26, existing agentsync/convention/hooks packages. No new dependencies.
- **Global constraints:** conventional commits, no trailers, zero em or en dashes, `go build ./...` + `go test ./...` + `make lint` (unmasked) green before every commit, `stardust check` stays 0 errors 0 warnings, index regenerated in the same commit as doc changes.

## Context

Read first: the spec and ADR 0047 (normative), `internal/agentsync/config.go` (DefaultConfig sources and targets, ImportOnly semantics), `internal/agentsync/inventory.go` (per-Kind discovery), `internal/convention/check.go` (checkStrayDoc:101, CheckSkills:129), `internal/hooks/hooks.go` (the guarded bodies, owned and composed), `internal/cli/init.go` (the docs scaffold), `.github/workflows/ci.yml`, and `internal/service/registry.go` (how registry sections render).

## Task 1: the docs/agents area, natively understood

Files:

- Modify: `internal/convention/check.go` (stray-doc exemption), `internal/agentsync/config.go` (source repoint + compat demotions), `internal/cli/init.go` (scaffold), `internal/service/registry.go` (Agents section)
- Test: matching `_test.go` files per package

Steps:

- [ ] Failing test: a `docs/agents/skills/demo/SKILL.md` in a docs-convention repo produces zero stray-doc issues; implement the `docs/agents/` exemption beside the `docs/templates/` one.
- [ ] Failing test: `agentsync.DefaultConfig` repo sources point at `docs/agents/skills`, `docs/agents/subagents`, `docs/agents/rules.md` at Priority 100, with `skills/`, `agents/`, `.stardust/rules.md` present as ImportOnly compat; implement the repoint. Pin collision resolution: the same item name in old and new sources resolves to the docs/agents copy.
- [ ] Failing test: `stardust init --docs` scaffolds `docs/agents/skills/`, `docs/agents/subagents/`, and a `docs/agents/rules.md` stub; implement.
- [ ] Failing test: the registry output contains an Agents section listing a seeded skill's name, description, and targets plus a seeded subagent; implement following the collection-section rendering pattern.
- [ ] Confirm `CheckSkills` validates the new path (extend only if folder-scoped) and that index/search cover the area (an integration assertion in the existing service test style).
- [ ] Full gate; commit `feat(agentsync): home shared agent assets in docs/agents`.

## Task 2: automatic sync

Files:

- Modify: `internal/hooks/hooks.go` (bodies), `.github/workflows/ci.yml` (drift gate)
- Test: `internal/hooks/hooks_test.go`

Steps:

- [ ] Failing test: the owned post-commit and post-merge bodies (and the composed sentinel block) contain the guarded `stardust sync` line after the registry line; implement in the shared body constants so owned and composed stay byte-consistent.
- [ ] Re-run the hooks idempotency tests: a reinstall replaces the sentinel block with the new body, exactly one block.
- [ ] Add the CI step: build the binary in-workflow and run `stardust sync --check` from the repo root (plus `stardust check --strict` if the workflow does not already gate it); keep the workflow's existing shape.
- [ ] Full gate; commit `feat(hooks): run agent-asset sync automatically with a ci drift gate`.

## Task 3: README

Files:

- Modify: `README.md`

Steps:

- [ ] Inventory every shipped post-v0.5.0 feature against source (git log v0.5.0..HEAD as the checklist): the JSON-RPC surface, MCP-through-registry, agent assets + rules sync, hooks compose, init auto-detect + status, the TUI, wiki mode (sibling autodetect, non-markdown pages), mount routing, contradiction candidates, endpoint-free reranking, the audit command, the plugin command set.
- [ ] Write the agent-assets section: the docs/agents layout, placement-as-sharing, auto-sync, the compat story, honestly scoped (reranking needs a local runtime, the TS client covers a subset, candidates are review-prompts).
- [ ] Refresh stale sections without bloating; verify every claim (command names, flags, config keys) in source; link docs/ for depth.
- [ ] Dash scan; full gate; commit `docs(readme): document the agent asset home and post-release surface`.

## Task 4: adversarial review

Steps:

- [ ] Fresh binary; temp repo end to end: init --docs scaffolds the area; a skill under docs/agents appears in check (clean), registry, and search; an installed-hooks commit materializes symlinks into `.claude/skills`, `.codex/skills`, `.gemini/skills` with no manual sync; breaking a symlink makes `sync --check` exit nonzero; a `.claude/skills`-only skill is never touched; a legacy root `skills/` repo still syncs via compat.
- [ ] The stardust repo itself: check still 0 errors 0 warnings, its CI workflow carries the gate, full go/lint/gofmt gate green, dash scan, conventional commits with zero trailers, `docs/INDEX.md` statuses match frontmatter.
- [ ] README: spot-verify every feature claim against source and the shipped binaries.
- [ ] Report defects; do not fix silently.

## Verification

The spec's Verification section, run live, plus the repo's own 0/0 and CI self-hosting. Flip this plan to Done and the spec to Implemented only when all four tasks are green.

## Self-review gate

- Placement semantics exact: docs/agents shared, tool dirs untouched, compat imports work.
- Owned and composed hook bodies stay byte-consistent; reinstall is idempotent.
- No checker weakening; the stray-doc exemption is the convention-known-area pattern, recorded in ADR 0047.
