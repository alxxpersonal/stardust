---
title: Stardust hardening implementation plan
status: Done
version: 1
date: 2026-06-26
related:
  - docs/specs/2026-06-26-1849-stardust-hardening.md
  - docs/adr/0024-inline-baked-authoring-commands.md
  - docs/adr/0025-index-derived-outputs-reconcile-disk.md
  - docs/adr/0026-docs-root-stray-docs-are-errors.md
---

Implement five hardening fixes with red-green TDD, then pass the full Go and plugin gates.

## Header

- Goal: stop stale derived state, ignore markdown code links, prune renamed index rows, enforce canonical docs folders, and bake forge workflows into plugin commands.
- Architecture: keep files as truth, keep SQLite derived, keep CLI and service surfaces thin over shared internals.
- Tech Stack: Go 1.26.1, modernc.org/sqlite, cobra, stardust docs collections, Claude plugin command markdown.
- Global Constraints: no commit, no push, no `go install`, no panic, `%w` errors, `errors.Is` for sentinels, `// --- Section ---` separators, doc comments on exports, gofmt and golangci-lint clean, no U+2014 or U+2013 anywhere.

## Context

Read first:

- `internal/cli/registry.go`
- `internal/service/registry.go`
- `internal/service/index.go`
- `internal/index/upsert.go`
- `internal/vault/vault.go`
- `internal/convention/check.go`
- `plugin/claude/commands/spec.md`
- `plugin/claude/commands/plan.md`
- `plugin/claude/commands/doc.md`
- `plugin/claude/commands/adr.md`
- `plugin/claude/README.md`
- `plugin/claude/.claude-plugin/plugin.json`

## Task 1: stale registry guard

Files:

- Modify: `internal/service/registry.go`, `internal/cli/registry.go`
- Test: `internal/service/registry_test.go`

Interfaces:

- Consumes: `Service.Registry(order []string)`
- Produces: stale error containing `index looks empty or stale, run stardust index`

Steps:

- [x] Write failing test: create docs on disk, do not index, call `Registry`, expect stale error.
- [x] Run `go test ./internal/service -run TestRegistryFailsWhenIndexEmptyOrStale -count=1` and confirm red.
- [x] Implement disk/index collection reconciliation in service registry.
- [x] Run the focused test until green.
- [x] Run `go test ./internal/service -run TestRegistry -count=1`.

## Task 2: markdown code masking

Files:

- Modify: `internal/vault/vault.go`
- Test: `internal/vault/vault_test.go`

Interfaces:

- Consumes: `ExtractLinks(body string) []string`, `ExtractEdges(root string, note Note) []Edge`
- Produces: links and inline path refs only from prose

Steps:

- [x] Write failing tests for wikilinks in inline code, backtick fence, tilde fence, and prose.
- [x] Run `go test ./internal/vault -run TestExtractLinksIgnoresMarkdownCode -count=1` and confirm red.
- [x] Implement same-length markdown code masking and use it in link extraction.
- [x] Run focused vault tests until green.
- [x] Run `go test ./internal/vault -count=1`.

## Task 3: incremental rename prune

Files:

- Modify: `internal/service/index.go`
- Test: `internal/service/index_test.go`

Interfaces:

- Consumes: `Service.Index(ctx context.Context, since string) (IndexStats, error)`
- Produces: old path deleted, new path indexed

Steps:

- [x] Write failing test: index a note, rename file on disk, run incremental index, assert old path absent and new path present.
- [x] Run `go test ./internal/service -run TestIndexPrunesRenamedPathsIncrementally -count=1` and confirm red.
- [x] Add current markdown set reconciliation before per-path indexing.
- [x] Run focused service test until green.
- [x] Run `go test ./internal/service -count=1`.

## Task 4: stray docs enforcement

Files:

- Modify: `internal/convention/check.go`
- Test: `internal/convention/check_test.go`

Interfaces:

- Consumes: `CheckDocs(root string, ignore []string) ([]ConventionIssue, error)`
- Produces: `stray-doc` errors for markdown under `docs/` outside registered collections

Steps:

- [x] Write failing tests for `docs/superpowers/x.md`, `docs/loose.md`, `docs/INDEX.md`, and `docs/templates/x.md`.
- [x] Run `go test ./internal/convention -run TestCheckDocsReportsStrayDocs -count=1` and confirm red.
- [x] Implement allowed collection folder detection from collection configs.
- [x] Run focused convention tests until green.
- [x] Run `go test ./internal/convention -count=1`.

## Task 5: inline plugin command baking

Files:

- Modify: `plugin/claude/commands/spec.md`
- Modify: `plugin/claude/commands/plan.md`
- Modify: `plugin/claude/commands/doc.md`
- Modify: `plugin/claude/commands/adr.md`
- Modify: `plugin/claude/README.md`
- Modify: `plugin/claude/.claude-plugin/plugin.json`
- Test: shell grep and unicode dash scan

Interfaces:

- Consumes: canonical skill workflows from `the spec authoring workflow` and `the doc authoring workflow`
- Produces: commands with `allowed-tools: Bash, Read, Write` and no `/the spec workflow` or `/the doc workflow` handoff lines

Steps:

- [x] Write grep checks that fail on missing `Write` or any `/the spec workflow` or `/the doc workflow` in command files.
- [x] Run the grep checks and confirm red.
- [x] Rewrite command bodies to embed the full workflows inline.
- [x] Update README and plugin metadata descriptions.
- [x] Run grep checks until green.

## Full verification

- [x] Run `stardust registry`.
- [x] Run `go build ./...`.
- [x] Run `go test ./... -race -count=1`.
- [x] Run `go vet ./...`.
- [x] Run `gofmt -l .` and require empty output.
- [x] Run `golangci-lint run ./...`.
- [x] Run a unicode dash grep for U+2014 and U+2013 and require zero matches.
- [x] Run `stardust serve --mcp` and send an initialize request.
- [x] Inspect `git status --short`.
