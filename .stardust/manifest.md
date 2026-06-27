# Stardust manifest

Vault: `Stardust`.

## Start here

- Search with `stardust query "<question>"` before assuming a note is missing.
- Read `.stardust/INDEX.md` for the vault index.
- Read `docs/INDEX.md` for docs registry status.

## Active plans

- Stardust v0.5.0 public release `docs/plans/2026-06-27-1813-stardust-v0-5-0-public-release.md`

## Stale implemented docs

- None.

## Docs referencing moved code

- Stardust hooks compose, never clobber `docs/specs/2026-06-25-0345-hooks-compose-not-clobber.md` references `internal/hooks/hooks.go` (3 commits), `internal/cli/hooks.go` (2 commits), `internal/cli/init.go` (3 commits); review
- Fang-styled stardust CLI with the cosmic colorscheme `docs/specs/2026-06-25-2319-fang-cli-cosmic.md` references `internal/cli/root.go` (1 commit), `internal/tui/styles.go` (3 commits), `internal/render/glamour.go` (1 commit); review
- Doc-code coherence engine `docs/specs/2026-06-26-0418-doc-code-coherence-engine.md` references `internal/convention/check.go` (2 commits), `internal/collections/collections.go` (1 commit), `internal/vault/vault.go` (4 commits) +4 more; review
- Stardust hardening for docs, index, links, and authoring commands `docs/specs/2026-06-26-1849-stardust-hardening.md` references `internal/service/registry.go` (1 commit), `internal/vault/vault.go` (3 commits), `internal/convention/check.go` (1 commit); review
- Init auto-detect and a status command `docs/specs/2026-06-26-2104-init-detect-and-status.md` references `internal/service/service.go` (3 commits), `internal/service/records.go` (1 commit), `internal/service/bundle.go` (1 commit); review

## Core conventions

- Files are source of truth; indexes and registries are derived.
- Docs use YAML frontmatter with type, status, related, and governs fields.
- Skills and agents may route with targets: claude, codex, gemini.
- Run `stardust check --strict` before committing convention docs.
