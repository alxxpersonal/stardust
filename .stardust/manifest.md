# Stardust manifest

Vault: `Stardust`.

## Start here

- Search with `stardust query "<question>"` before assuming a note is missing.
- Read `.stardust/INDEX.md` for the vault index.
- Read `docs/INDEX.md` for docs registry status.

## Active plans

- None.

## Stale implemented docs

- None.

## Docs referencing moved code

- Stardust hooks compose, never clobber `docs/specs/2026-06-25-0345-hooks-compose-not-clobber.md` references `internal/hooks/hooks.go` (3 commits), `internal/cli/hooks.go` (2 commits), `internal/cli/init.go` (2 commits); review
- JSON-RPC 2.0 is the canonical contract transport `docs/adr/0001-jsonrpc-canonical-transport.md` references `docs/openapi.yaml` (1 commit), `internal/api/api.go` (3 commits); review
- One method registry served over multiple transports `docs/adr/0003-one-method-registry-multi-transport.md` references `internal/api/api.go` (3 commits); review
- Use creachadair/jrpc2 and the shared JSON-RPC conventions `docs/adr/0006-use-jrpc2-and-shared-conventions.md` references `rpc/contract.go` (2 commits), `docs/openapi.yaml` (1 commit); review
- Stardust composes hooks, never clobbers `docs/adr/0007-stardust-composes-hooks-never-clobbers.md` references `internal/hooks/hooks.go` (3 commits); review

## Core conventions

- Files are source of truth; indexes and registries are derived.
- Docs use YAML frontmatter with type, status, related, and governs fields.
- Skills and agents may route with targets: claude, codex, gemini.
- Run `stardust check --strict` before committing convention docs.
