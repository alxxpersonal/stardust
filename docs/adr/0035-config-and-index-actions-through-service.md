---
title: Settings mutates config and runs index actions through additive service methods
status: Accepted
date: 2026-06-27
related:
  - docs/adr/0032-tui-reads-through-service-layer.md
  - docs/adr/0016-vectors-on-by-default-loud-degradation.md
  - docs/specs/2026-06-27-0230-settings-tab-and-drift-redesign.md
---

# Settings mutates config and runs index actions through additive service methods

## Context

ADR 0032 requires every TUI tab to read through `internal/service`, never the index, git, or disk directly. The Settings tab also writes: it persists config changes and triggers reindex, rebuild, and registry regeneration. The naive path is to call `config.Save` and `manifest.WriteRegistry` from the `tui` package. That breaks two ways. First, it bypasses the service-layer invariant for writes. Second, `Service` caches `embed` and `rerank` clients built from config at `Open`; saving `config.toml` from outside the service leaves the live service holding stale clients, so a changed `embed_model` or `ollama_url` does not take effect until the process restarts.

The registry regeneration sequence (query the docs collections, `manifest.WriteRegistry`, `RefreshManifest`) currently lives inline in `internal/cli/registry.go`. The TUI needs the same effect without importing `manifest` or duplicating the collection order in the `tui` package, and without changing the CLI.

## Decision

Extend the read-through-service rule to writes. Add two additive methods to `internal/service`:

- `SetConfig(cfg config.Config) error` persists `cfg` to `s.Layout.Config()`, sets `s.Config = cfg`, and rebuilds `s.embed` and `s.rerank` from the new values so later reads use the new model and endpoints live.
- `RegenerateRegistry(ctx context.Context) error` queries the docs collections in the fixed order, writes `docs/INDEX.md` via `manifest.WriteRegistry`, and calls `RefreshManifest`, mirroring `stardust registry`.

The Settings tab calls `SetConfig`, `RegenerateRegistry`, `Index(ctx, "")`, and `Rebuild(ctx)`. It never imports `config`, `manifest`, or `embed` to perform a write. The CLI is unchanged; the duplication of the three-step registry sequence between the new service method and `internal/cli/registry.go` is accepted to keep the CLI out of scope.

## Consequences

- The read-through-service invariant (ADR 0032) now covers config and index writes from the TUI.
- A config change takes effect on the live service immediately; no restart, no stale embed or rerank client.
- A saved bad `ollama_url` surfaces through the existing loud FTS-only degradation (ADR 0016), not a silent failure.
- One small, intentional duplication: the registry sequence exists in both the service method and the CLI command until a later ADR consolidates them.

## Alternatives considered

- Call `config.Save` and `manifest.WriteRegistry` from the `tui` package. Rejected: violates ADR 0032 and leaves stale cached clients.
- Reopen the whole service after each config save. Rejected: heavier, drops the open index handle, and races with other tabs; rebuilding only the two clients is the minimal correct fix.
- Refactor the CLI `registry` command to consume `RegenerateRegistry` now. Rejected: out of scope; the spec forbids CLI changes.

## References

- ADR 0032 TUI reads through service, ADR 0016 vectors loud degradation.
- `internal/service/service.go`, `internal/service/registry.go`, `internal/config/config.go`, `internal/cli/registry.go`.
- Spec: docs/specs/2026-06-27-0230-settings-tab-and-drift-redesign.md
</content>
