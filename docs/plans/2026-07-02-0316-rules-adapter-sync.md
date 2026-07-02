---
title: Rules adapter sync - implementation plan
status: Done
version: 1
date: 2026-07-02
related:
  - docs/specs/2026-07-02-0316-rules-adapter-sync.md
  - docs/adr/0039-rules-adapter-sync.md
  - internal/agentsync/config.go
  - internal/agentsync/inventory.go
  - internal/agentsync/plan.go
  - internal/agentsync/apply.go
  - internal/hooks/block.go
---

# Rules adapter sync - implementation plan

Add a third `KindRules` to `internal/agentsync` that renders a canonical `.stardust/rules.md` per tool and composes it into `CLAUDE.md`, `AGENTS.md`, and `GEMINI.md` through a sentinel-delimited block, riding the existing `stardust sync` flow.

## Header

- **Goal:** one canonical rules source, composed format-aware into the three root memory files, compose-never-clobber, drift reported by `sync --check`, healed by `sync`.
- **Architecture:** extend the existing sync pipeline (`Discover` -> `BuildPlan` -> `Apply`) with a `rules` kind, a `RulesPath` target field, a markdown sentinel-block composer mirroring `internal/hooks/block.go`, and a per-tool adapter map. No new command, flag, or config file.
- **Tech stack:** Go 1.26, `os` file IO, `gopkg.in/yaml.v3` (already a dep). No new dependencies.
- **Global constraints:** conventional commits, no co-author or generated-by trailers, `go build ./...` + `go test ./...` + `make lint` green, `gofmt -l .` empty, ZERO em or en dashes.

## Context

`internal/agentsync` syncs `KindSkill` and `KindAgent` as symlinks or copies into per-tool asset directories. Rules is different: one canonical body, composed into shared root files that humans and other tools also write. The design (spec `2026-07-02-0316-rules-adapter-sync.md`, ADR 0039) reuses the hooks compose-never-clobber pattern (ADRs 0007, 0008) with markdown-comment markers.

## Reuse map (read first)

- `internal/hooks/block.go` - `injectBlock` / `stripBlock` / `stripExistingBlock` / `collapseBlankRuns` / `ensureTrailingNewline`: the exact pattern to mirror for markdown markers.
- `internal/agentsync/inventory.go` - `Kind`, `Item`, `Discover`, `discoverSource`, `readItem`, `parseFrontmatter`, `ParseTargets`, `frontmatterString`, `itemName`.
- `internal/agentsync/config.go` - `Source`, `Target`, `Config`, `DefaultConfig`, `normalizeConfig`, `expandPath`, `validateTool`.
- `internal/agentsync/plan.go` - `BuildPlan`, `buildAction`, `itemTargetPath`, the status accounting.
- `internal/agentsync/apply.go` - `Apply`, `applyAction`, `createTarget`.
- `internal/config/config.go` - `Layout` and its path methods (add `Rules()`).

## Task 1: build the rules kind

- Modify: `internal/config/config.go`, `internal/agentsync/config.go`, `internal/agentsync/inventory.go`, `internal/agentsync/plan.go`, `internal/agentsync/apply.go`
- Create: `internal/agentsync/rules.go`

- [x] Add `Layout.Rules()` returning `<.stardust>/rules.md`; unit test it like `TestLayoutSyncConfig`.
- [x] Add `KindRules Kind = "rules"`. Accept `"rules"` in `normalizeConfig`'s source-kind switch.
- [x] Add `RulesPath string` (`toml:"rules_path"`) to `Target`; expand it in `normalizeConfig` via `expandPath`.
- [x] Wire `DefaultConfig`: a `repo-rules` source at `.stardust/rules.md` (kind `rules`, priority 100), and `RulesPath` on the three repo targets (claude -> `CLAUDE.md`, codex -> `AGENTS.md`, gemini -> `GEMINI.md`). Leave global `RulesPath` empty.
- [x] Update `config_test.go` expectations for the new default source and target field. Run, loop to green.
- [x] Commit `feat(agentsync): add rules kind and canonical source config`.
- [x] `discoverRules(src, defaults)`: stat `src.Path` as a file; read it into one `Item{Kind: KindRules, Name: frontmatter name or "rules"}`; apply `ParseTargets`; a missing file returns nil. Add the `case "rules"` branch to `discoverSource`.
- [x] Test discover: one item from a seeded `.stardust/rules.md`, `targets` frontmatter honored, missing file yields none. Run, loop to green.
- [x] Commit `feat(agentsync): discover the canonical rules source`.
- [x] `internal/agentsync/rules.go`: the two markdown markers as constants; `rulesAdapter{fileName, render}`; `rulesAdapters` map for claude / codex / gemini; `render` strips frontmatter and normalizes; `injectRulesBlock` / `stripRulesBlock` mirroring `block.go` (create if absent, replace in place, preserve outside lines, collapse blank runs).
- [x] Test the block: user lines preserved, exactly one block, idempotent re-inject, strip removes only the block, CRLF target, no frontmatter fence in the rendered block, unknown tool errors. Run, loop to green.
- [x] Commit `feat(agentsync): render and compose rules blocks`.
- [x] `buildRulesAction(target, item)`: status `create` (missing file or absent block) / `ok` (block equals render) / `drift` (block differs), mode `compose`, never `conflict`. In `BuildPlan`, skip `KindRules` items when `RulesPath == ""`; branch `buildAction` to `buildRulesAction` for `KindRules`.
- [x] Test plan: create / ok / drift / skip-empty-path / no-conflict. Run, loop to green.
- [x] Commit `feat(agentsync): plan rules compose actions`.
- [x] `composeRules(action)`: read `action.Source`, render for `action.Tool`, `injectRulesBlock` into `action.Target`. Add `case "compose"` to `createTarget`; in `applyAction`, make `compose`-mode `drift` re-inject on a plain apply while leaving symlink and copy `--repair` guards unchanged.
- [x] Test apply: compose create writes a fresh file with the block; compose drift heals without `--repair`; symlink and copy repair behavior unchanged. Run, loop to green.
- [x] `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` green.
- [x] Commit `feat(agentsync): apply rules compose and self-heal drift`.

## Task 2: verify, document, and gate

- Modify: `SPEC.md`, `README.md`
- Verify: end-to-end round-trip and the full gate

- [x] Integration test: seed `.stardust/rules.md`, run sync through the service layer, assert `CLAUDE.md` / `AGENTS.md` / `GEMINI.md` each carry the block with pre-existing user lines intact.
- [x] Integration test: `stardust sync --check` exits 0 when in sync; edit the canonical file; assert `--check` exits non-zero; run `sync`; assert it heals and re-check is 0.
- [x] Update SPEC.md section 4.2 and README line ~214 to state rules sync ships (canonical source, sentinel-block compose, per-tool adapters), removing the deferral wording.
- [x] Set the spec `status` to `Implemented` and this plan `status` to `Done`.
- [x] Regenerate the docs index: `stardust index && stardust registry`.
- [x] Full gate: `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` empty, zero U+2014 / U+2013 in touched files, `stardust check` clean.
- [x] Commit `docs(agentsync): mark rules-adapter sync shipped`.

## Verification

- Discover yields exactly one rules item, `targets`-aware, missing-file-safe.
- Block inject is idempotent and preserves user lines; strip is surgical; CRLF handled; no frontmatter leaks.
- Plan maps target state to create / ok / drift, skips empty `RulesPath`, never conflicts.
- Apply composes on create, self-heals compose drift without `--repair`, leaves symlink and copy semantics unchanged.
- End-to-end: three root files carry the block, user content survives, `--check` and `sync` behave as specified.
- `go build ./...`, `go test ./...`, `make lint`, `gofmt -l .` green; zero em or en dashes.

## Self-review gate

- Every spec Work-breakdown item maps to a step here.
- Markers are single shared constants across inject and strip.
- `compose` is the only mode that self-heals drift; symlink and copy `--repair` is byte-identical to today.
- Skill and agent sync behavior is unchanged.
- No new command, flag, or config file was added.
