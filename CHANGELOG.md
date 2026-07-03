# Changelog

All notable changes to this project will be documented in this file.

## [unreleased]

### Bug Fixes

- **conventions:** Keep directory links out of graph

- **conventions:** Ignore generated index link checks

- **plugin:** Resolve the vault per project dir to stop session collisions

- **plugin:** Resolve the workspace from the cwd with walk-up and env override

- **vault:** Scan .markdown files like the renderer

- **build:** Unmask golangci-lint failures and clear the two standing findings

- **check:** Keep the explicit-title rule markdown-only and refresh the index

- **mounts:** Keep name mentions from pruning metadata-less mounts and gate single-mount routing

- **examples:** Remove invalid codex model literal from cron examples

- **agentsync:** Keep legacy compat sources materializing and exempt docs/agents from orphans

- **check:** Exempt rules-compose instruction files from the orphan warning

- **agentsync:** Scope sources so repo assets never leak into global tool dirs


### Documentation

- **plugin:** Document layered workspace resolution

- **plan:** Tick the review checkboxes on the cwd resolution plan

- **spec:** Correct the shipped api surface claims

- **plans:** Close out the shipped vision plans

- **agentsync:** Spec the rules-adapter sync

- **agentsync:** Regenerate docs index for rules-adapter sync

- **convention:** Spec sibling source-root autodetection for wikis

- **vault:** Spec non-markdown wiki page indexing

- **mounts:** Spec query-aware mount routing

- **digest:** Spec cross-note contradiction candidates

- **rerank:** Spec endpoint-free reranking

- **rerank:** Correct survey errata and complete the queryresult schema

- **spec:** Correct full-sdk overclaim to a typed-client subset

- **examples:** Exclude cron-job teaching examples from the index

- **adr:** Renumber duplicate adr 0037 to 0045

- Re-source doc-code drift for the v0.5.0 release

- Resolve orphan and missing-title warnings for the release

- **plan:** Tick the completed v0.5.0 self-indexing tasks

- **agentsync:** Spec the docs/agents asset home with automatic sync

- **specs:** Re-source check.go line references after the duplicate-name gate

- Re-source check.go references after the duplicate-name gate

- **plan:** Backtick an index path reference

- **readme:** Document the agent asset home and post-release surface

- **plan:** Close the agent assets plan


### Features

- **wiki:** Cross-repo wiki-to-code drift via source_root

- **conventions:** Add managed directory indexes

- **plugin:** Add the stardust:audit command

- **rpc:** Extend the contract for the remaining mcp tools

- **agentsync:** Sync rules format-aware into agent instruction files

- **convention:** Autodetect a sibling source root for wiki checkouts

- **vault:** Index non-markdown wiki pages

- **mounts:** Route queries to relevant mounts

- **digest:** Surface cross-note contradiction candidates

- **rerank:** Rerank without a dedicated endpoint

- **check:** Warn duplicate names only on referenced ambiguity and close the v0.5.0 plan

- **agentsync:** Home shared agent assets in docs/agents

- **hooks:** Run agent-asset sync automatically with a ci drift gate


### Miscellaneous

- **stardust:** Stop tracking the derived index and manifest


### Refactoring

- **mcp:** Route all tools through the jrpc2 registry


### Testing

- **rpcserver:** Pin the openrpc document against the registry


## [0.5.0] - 2026-06-27

### Bug Fixes

- **manifest:** Title-case registry headings to match spec

- **hooks:** Uninstall strips only stardust lines

- **hooks:** Detect a pre-commit-only chain as compose

- **mcp:** Wrap mounts and list_collections tool outputs as objects

- **tui:** Left/right tab navigation and scrollable document views

- **tui:** Keep keycap headers on empty graph sections

- **tui:** Single headers, drift and settings layout, newest-first browse

- **tui:** Pad empty drift and graph sections to full box width

- **tui:** Single drift summary, full check findings, graph legends


### Documentation

- Document stardust registry

- Document agent infrastructure commands

- **contract:** Add jsonrpc typed-contract spec, adrs, plan; scaffold stardust docs vault

- **contract:** Record phase e/f tasks and the full-routes amendment

- **hooks:** Add compose-not-clobber spec, adrs, and plan

- Specs, plans, and ADRs for fang, the plugin, and the coherence engine

- Stardust hardening spec, plan, and ADRs 0024-0026

- Specs, plans, and ADRs for init-detect, status, and the TUI clone

- Settings tab and drift redesign spec, plan, and ADRs 0034-0036

- V0.5.0 public release spec and plan

- **readme:** Add brew install as the primary install method


### Features

- Local-first markdown context engine (v1)

- **rerank:** Optional cross-encoder reranking via configurable endpoint

- **api:** Service core seam + http/json api via stardust serve

- **mcp:** Mcp server + claude code plugin via stardust serve --mcp

- **mounts:** Mcp aggregator + cross-source rrf fusion

- **bundle:** Task-scoped context assembler (pagerank + hybrid + budget)

- **memory:** Agent write-back via the six-verb memory tool + dedup

- **temporal:** Git-as-event-stream digest, commitment surfacing

- **sdk:** Typed go + ts clients over the http api

- **plugin:** Obsidian plugin over the http api

- **check:** Vault integrity check, commit gate, and stardust new scaffold

- **api:** Add mounts listing, graph pagerank, and note link resolution

- **collections:** Typed collections over vault folders

- **api:** Collections and records HTTP, MCP, and SDK surfaces

- **cron:** Live scheduler + explicit runner field

- **api:** Record delete endpoint + SDK method

- **manifest:** Add grouped docs registry renderer

- **service:** Assemble docs registry groups from collections

- **cli:** Add stardust registry command

- **hooks:** Regenerate docs registry on commit

- **init:** Scaffold docs collections with --docs

- **sync:** Load agent sync configuration

- **sync:** Discover routed agent assets

- **sync:** Plan dry-run and drift checks

- **cli:** Add stardust sync

- **sync:** Scaffold agent infra migration config

- **service:** Query docs that govern code paths

- **cli:** Show governing docs for code paths

- **service:** Detect stale governed docs

- **cli:** Scaffold convention docs with stardust new

- **check:** Lint docs and agent conventions

- **manifest:** Refresh agent boot context from docs

- **registry:** Add stale subcommand listing docs whose governed code moved

- **manifest:** Re-source stale docs from registry stale scan for rich drift

- **check:** Add --fix to autofix mechanically-safe doc issues

- **rpc:** Add typed contract package for the record seam

- **rpcserver:** Add jrpc2 method registry

- **api:** Serve the jrpc2 registry over POST /rpc

- **rpc:** Add typed jrpc2 client

- **rpc:** Add contract types for the full operation set

- **rpcserver:** Register the full operation set

- **rpc:** Add client methods for the full operation set

- **rpcserver:** Add rpc.discover and the openrpc document

- **hooks:** Detect an existing hook chain

- **hooks:** Add idempotent sentinel-block helpers

- **hooks:** Compose into an existing chain instead of seizing core.hooksPath

- **hooks:** Report owned vs composed install mode

- **cli:** Adopt charm.land/fang/v2 for styled errors and help

- Doc-code coherence engine

- **plugin:** Stardust Claude Code plugin

- Harden the coherence engine (registry, wikilinks, prune, stray-doc)

- **plugin:** Bake spec-forge and doc-forge into the authoring commands

- **cli:** Init auto-detect and a styled status command

- **tui:** Clone the exo-jobs interactive TUI, recolored cosmic

- **service:** Add SetConfig and RegenerateRegistry

- **tui:** Clean-list redesign, search overhaul, drift fix, settings tab, nav

- **vault:** Exclude the generated registry from indexing

- **render:** Full cosmic glamour theme with clean headings

- **tui:** Open a search result in fullscreen

- **settings:** Edit collections through the TUI

- **settings:** Make the collections box directly actionable

- **plugin:** Bake spec-forge and doc-forge into the slash commands

- **wiki:** GitHub wiki compatibility

- **wiki:** Full GitHub wiki mode

- **tui:** Workspace status line


### Infrastructure

- Add MIT license, github actions ci, and changelog


### Miscellaneous

- Bump version to v0.2.0

- Regenerate drift manifest

- Scrub separate-product references and private paths for public


### Refactoring

- **convention:** Centralize docs frontmatter rules

- **mcp:** Map mcp framing onto the jrpc2 registry

- **sdk/ts:** Consume the jsonrpc /rpc endpoint

- **api:** Retire rest handlers superseded by the jsonrpc registry

- **agentsync:** Generic default migration profile


### Style

- **check:** Use unicode escapes for forbidden-dash literals


### Testing

- **convention:** Avoid gosec slice warning

- **rpc:** Pin method wire shapes with golden files

- **rpcserver:** Pin the method set

- **rpcserver:** Pin the error code band

- **rpcserver:** Pin stdio and http transport parity

- **rpc:** Pin the full operation set


### Ci

- Brew release pipeline via GoReleaser

- Configure a git identity for initial-commit tests


---
*Generated by [git-cliff](https://git-cliff.org)*
