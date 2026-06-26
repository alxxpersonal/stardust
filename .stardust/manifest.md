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
- Doc-code coherence engine `docs/specs/2026-06-26-0418-doc-code-coherence-engine.md` references `internal/convention/check.go` (1 commit), `internal/vault/vault.go` (1 commit), `internal/service/index.go` (1 commit); review

## Core conventions

- Files are source of truth; indexes and registries are derived.
- Docs use YAML frontmatter with type, status, related, and governs fields.
- Skills and agents may route with targets: claude, codex, gemini.
- Run `stardust check --strict` before committing convention docs.
