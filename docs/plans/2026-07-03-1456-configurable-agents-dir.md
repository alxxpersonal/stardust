---
title: Configurable agents dir - implementation plan
status: Active
version: 1
date: 2026-07-03
related:
  - docs/specs/2026-07-03-1456-configurable-agents-dir.md
  - docs/adr/0048-configurable-agents-dir.md
---

Add the `agents_dir` config knob with a `docs/agents` default and thread it through all six convention seams, keeping the default byte-identical.

## Header

- **Goal:** the agent-assets home is configurable per workspace; no knob means exactly today's behavior.
- **Architecture:** one resolver method on `config.Config` owns default + normalization; six seams consume the resolved value as an explicit input; check validates the knob.
- **Tech stack:** Go 1.26, existing packages, no new dependencies.
- **Global constraints:** conventional commits, no trailers, zero em or en dashes, full gate green before every commit (`go build ./...`, `go test ./...`, `make lint` unmasked, `gofmt -l .` empty), `stardust check` 0 errors 0 warnings at every commit boundary, drifted docs re-sourced in the same commit, index regenerated with doc changes.

## Context

Read first: the spec and ADR 0048 (normative), `internal/config/config.go` (Config struct, SourceRoot precedent), `internal/agentsync/config.go` (DefaultConfig + its two callers `internal/cli/sync.go:93` and `internal/service/registry.go:117`), `internal/convention/check.go:110` (stray-doc exemption), `internal/service/check.go:62` (orphan exemption), `internal/cli/init.go:150` (scaffold), and how `service.Check` assembles issues for the `bad-agents-dir` validation.

## Task 1: knob, resolver, and all six seams

Files:

- Modify: `internal/config/config.go`, `internal/agentsync/config.go`, `internal/convention/check.go`, `internal/service/check.go`, `internal/service/registry.go`, `internal/cli/init.go`, `internal/cli/sync.go`
- Test: matching `_test.go` per package

Steps:

- [ ] Failing test: `Config.AgentsDir()` returns `docs/agents` when unset, normalizes `docs/skills/` to `docs/skills`, slash-normalizes; implement the field + resolver.
- [ ] Failing test: `agentsync.DefaultConfig` builds the three repo sources under a passed agents dir (pick the smallest API: an added parameter; update both production callers); default path unchanged byte-for-byte; implement.
- [ ] Failing test: stray-doc exemption follows a configured `docs/skills` (exempt) while `docs/agents` under that config is flagged again; thread the dir into `CheckDocs`; implement.
- [ ] Failing test: orphan exemption follows the configured dir; implement in `service.Check`.
- [ ] Failing test: the registry Agents section discovers under the configured dir; implement.
- [ ] Failing test: `init --docs` scaffolds the configured dir when config pre-exists, else the default; implement.
- [ ] Failing test: `bad-agents-dir` check error for absolute, `..`-escaping, and collection-colliding values; implement in `service.Check`.
- [ ] Full gate; re-source any drifted docs in the same commit; commit `feat(config): make the agent assets home configurable`.

## Task 2: docs and adversarial review

Files:

- Modify: `README.md`, `plugin/claude/commands/setup.md`
- Sync: setup.md into the active plugin cache version dir

Steps:

- [ ] Add the one-line `agents_dir` documentation to the README agent-assets section and setup.md's config shape; dash scan; sync the touched command file into the newest `~/.claude/plugins/cache/stardust-local/stardust/<version>/` dir.
- [ ] Live end-to-end in a temp repo with `agents_dir = "docs/skills"`: scaffold, check-clean skill, registry listing, hook-driven sync into all three tool dirs, `sync --check` drift gate; then the same flow with NO knob proving the default is byte-identical to v0.6.0 behavior; then the three invalid values each produce `bad-agents-dir`.
- [ ] Full gate; index regen; commit `docs(config): document the agents_dir knob`.
- [ ] Flip this plan to Done and the spec to Implemented in the same commit; verify repo check 0/0; report defects honestly instead of silencing.

## Verification

The spec's Verification section run live, the default-path byte-identical proof, the invalid-knob errors, and the stardust repo's own 0/0 hold.

## Self-review gate

- No seam retains the `docs/agents` literal outside the resolver's default.
- The default path produces identical plans, checks, registry output, and scaffolds to v0.6.0.
- sync.toml overrides still win over the knob-derived defaults.
