---
title: Init auto-detect and a status command
status: Implemented
version: 1
date: 2026-06-26
related:
  - internal/cli/init.go
  - internal/cli/root.go
  - internal/cli/context.go
  - internal/cli/output.go
  - internal/cli/registry.go
  - internal/convention/convention.go
  - internal/service/service.go
  - internal/service/records.go
  - internal/service/bundle.go
  - internal/config/config.go
  - internal/gitx/gitx.go
  - docs/adr/0027-init-auto-detect-default-policy.md
  - docs/adr/0028-status-command-and-json-contract.md
---

`stardust init` MUST pick a sensible docs default by sniffing the target directory when neither `--docs` nor `--no-docs` is given, and a new `stardust status` command MUST report initialization, detected kind, collections with counts, index health, and freshness in a compact human block or ANSI-free JSON.

<details>
<summary><b>Problem</b></summary>
<br>

Two gaps make first-run and state-checking manual and error prone.

1. `internal/cli/init.go` scaffolds the docs collections (specs, plans, adr, research) only when the operator remembers to pass `--docs`. It does zero detection. A code repo that wants the docs convention silently gets a bare vault, and there is no `--no-docs` to assert the opposite intent. The right default is knowable from the directory contents.
2. There is no `stardust status`. The only way to check whether a directory is initialized, what kind of vault it is, which collections are registered, how many records each holds, whether vectors are live, and how far behind HEAD the index sits, is to read files and run several subcommands by hand. Agents and the SDK have no single parseable state probe.

</details>

<details>
<summary><b>Context and background</b></summary>
<br>

Init today. `newInitCmd` binds one `--docs` bool and calls `runInit` then `scaffoldVault(ctx, cwd, "off", docs)`. `scaffoldVault` writes the `.stardust` layout and, when `docs` is true, calls `writeDocsCollections` over `convention.DefaultDocCollections()`. The detection logic must sit before `scaffoldVault` and resolve a final `docs` bool.

Detection home. `internal/convention` already centralizes docs convention rules (`DefaultDocCollections`, `DocCollection`, status sets). A kind-detection function belongs here: it is a convention question (is this directory a code repo that wants the docs convention, or a human markdown vault that does not). `convention` imports `config`, `collections`, `gitx`, `vault`, `agentsync`; it does not import `service`, so `service` may import `convention` without a cycle.

Service layer. `internal/service/service.go` already has a `Status` type and `(*Service).Status` that report index health: notes, chunks, last indexed sha, embed model, vectors on/off, reranker. `internal/service/records.go` has `(*Service).ListCollections` returning `[]CollectionInfo` (name, path, description, fields, live record count). `internal/service/bundle.go` has `(*Service).commitsBehindHead(ctx) (int, bool)`, which returns the commit distance from `last_indexed_sha` to HEAD, ok false outside git or when the cursor is unset. The new status method composes these three existing reads plus detection, and must additionally handle the uninitialized case, where `service.Open` fails with `config.ErrNoVault` before any of them can run.

Root resolution. `config.FindRoot(start)` walks up from `start` to the nearest `.stardust`, returning `config.ErrNoVault` when none is found. `cli/context.go` `openService` already resolves from cwd (or `STARDUST_VAULT`). The status data-gatherer must resolve the root itself so it can report the not-initialized state instead of erroring out.

JSON discipline. `cli/output.go` `emitJSON` encodes indented JSON straight to `cmd.OutOrStdout`, never through fang, so piped JSON carries zero ANSI bytes (proven by `TestPipedJSONOutputHasZeroANSI` in `cli/headless_ansi_test.go`). `registry`, `bundle`, and `query` all follow this. The status command MUST match it.

Degradation vocabulary. `service.go` defines `ftsOnlyReason`, the one-line cause surfaced when embeddings are unavailable. The status vectors reason reuses this exact string so the explanation is identical across surfaces.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. `stardust init` with neither flag detects the directory kind and scaffolds the docs convention for a code repo, or a plain vault otherwise, printing one line that states what was detected and the override flag.
2. `stardust init --docs` always scaffolds and `stardust init --no-docs` never scaffolds, both overriding auto-detection.
3. Detection lives in one small, pure, table-testable `convention` function with a documented precedence.
4. `stardust status` reports, for the resolved root: initialized or not, detected kind, registered collections with live counts, index health (note count, vectors on/off with reason, commits-behind-HEAD when git is available), and a clear init hint when not initialized.
5. `stardust status --output json` emits ANSI-free valid JSON matching the existing query/bundle/registry contract, with the data gathering in a service-layer function and the CLI thin.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No recursive directory walk for detection. Top-level sniff only, for speed and predictability.
- No change to `service.Status` (the index-health method) or the `rpc`/`mcp` "status" handler that wraps it. The new richer report is a separate type and call.
- No auto-init from `status`. When uninitialized it only reports and hints.
- No new git capability. Freshness reuses `gitx.CommitCountSince` via the existing `commitsBehindHead`.
- No `--docs`/`--no-docs` mutual-exclusion error beyond cobra defaults; if a caller sets both, `--docs` wins (explicit scaffold is the safe over-eager choice). Documented, not enforced as an error.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

Two units: a detection function in `convention`, and a status data-gatherer in `service`. Two thin CLI surfaces wire them.

### Detection: convention.DetectKind

```go
// Kind is the detected nature of a directory: a code repo that defaults to the
// docs convention, or a plain markdown vault that does not.
type Kind int

const (
	KindPlainVault Kind = iota
	KindCodeRepo
)

// WantsDocs reports whether init should scaffold the docs convention by default
// for this kind.
func (k Kind) WantsDocs() bool { return k == KindCodeRepo }

// Label returns the stable status string for the kind.
func (k Kind) Label() string // "code-repo-with-docs" | "plain-vault"

// Describe returns the one-line init detection sentence including the override.
func (k Kind) Describe() string

// DetectKind sniffs the top level of dir (non-recursive) and classifies it.
// Precedence, first match wins:
//   1. an .obsidian directory present        -> KindPlainVault
//   2. a source marker present               -> KindCodeRepo
//   3. markdown-dominant                     -> KindPlainVault
//   4. a .git directory present              -> KindCodeRepo
//   5. otherwise                             -> KindPlainVault
func DetectKind(dir string) (Kind, error)
```

Source markers (top level): the manifest files `go.mod`, `package.json`, `Cargo.toml`, and the presence of any `*.go`, `*.ts`, `*.py`, or `*.rs` file. Markdown-dominant: at least one `*.md` file and the `*.md` count is greater than or equal to the count of non-markdown regular files (dotfiles excluded from both sides). The `.git`-only rule sits below markdown-dominance on purpose: a Stardust vault is itself a git repo, so `.git` alone must not flip a markdown vault into a code repo; it only decides an otherwise ambiguous, non-markdown directory. `.obsidian` is the strongest human-vault signal and wins outright.

`Describe` returns, per kind:
- code repo: `detected a code repo, scaffolding the docs convention (use --no-docs to skip)`
- plain vault: `detected a plain vault, skipping the docs convention (use --docs to scaffold)`

### Init wiring

`newInitCmd` binds a second bool `--no-docs` and resolves the final scaffold decision from explicit flags or detection:

```go
var docs, noDocs bool
// ... in RunE:
scaffold, line, err := resolveInitDocs(cmd, cwd, docs, noDocs)
```

`resolveInitDocs` checks `cmd.Flags().Changed("docs")` and `Changed("no-docs")`:
- `--docs` set: scaffold true, no detection line.
- else `--no-docs` set: scaffold false, no detection line.
- else: `convention.DetectKind(cwd)`, scaffold = `kind.WantsDocs()`, line = `kind.Describe()`.

`runInit` prints `line` (when non-empty) before the `Initialised .stardust/ ...` line, then calls `scaffoldVault(ctx, cwd, "off", scaffold)`. A `DetectKind` error is non-fatal: log nothing, default to plain vault (scaffold false), so init never fails on an unreadable entry it could merely sniff.

### Status data-gatherer: service.GatherStatus

A package-level function, not a method, because the uninitialized case cannot open a `Service`:

```go
// VaultStatus is the full state probe for one directory.
type VaultStatus struct {
	Root        string           `json:"root"`
	Initialized bool             `json:"initialized"`
	Kind        string           `json:"kind"`
	Collections []CollectionInfo `json:"collections"`
	Index       IndexHealth      `json:"index"`
	Hint        string           `json:"hint,omitempty"`
}

// IndexHealth is the derived-index portion of a status report.
type IndexHealth struct {
	Notes            int    `json:"notes"`
	Vectors          bool   `json:"vectors"`
	VectorsReason    string `json:"vectors_reason,omitempty"`
	CommitsBehind    int    `json:"commits_behind"`
	HasCommitsBehind bool   `json:"has_commits_behind"`
	LastIndexed      string `json:"last_indexed_sha,omitempty"`
	EmbedModel       string `json:"embed_model,omitempty"`
}

// GatherStatus resolves the vault root from start and reports full state. When
// no .stardust is found it returns an uninitialized report (with detected kind
// and an init hint) and a nil error. Otherwise it opens the service, composes
// the existing index-health, collections, and freshness reads, and closes.
func GatherStatus(ctx context.Context, start string) (VaultStatus, error)
```

Logic:
1. `root, err := config.FindRoot(start)`. On `errors.Is(err, config.ErrNoVault)`: detect kind from `start`, return `VaultStatus{Root: start, Initialized: false, Kind: kind.Label(), Collections: []CollectionInfo{}, Hint: "run stardust init to initialize this directory"}`, nil. On any other error, return it.
2. Open service: `svc, err := Open(ctx, root)`; defer `svc.Close()`.
3. `st, err := svc.Status(ctx)` for index health; `cols, err := svc.ListCollections(ctx)`; `behind, hasBehind := svc.commitsBehindHead(ctx)`; `kind, _ := convention.DetectKind(root)`.
4. Compose `VaultStatus{Initialized: true, Kind: kind.Label(), Collections: cols, Index: IndexHealth{Notes: st.Notes, Vectors: st.Vectors, VectorsReason: reason, CommitsBehind: behind, HasCommitsBehind: hasBehind, LastIndexed: st.LastIndexed, EmbedModel: st.EmbedModel}}`. `reason` is `ftsOnlyReason` when `st.Vectors` is false, else empty. `Collections` is never nil (use `cols` which `ListCollections` returns non-nil, or coalesce to `[]CollectionInfo{}`).

### Status CLI: cli/status.go

Thin, mirroring `registry`/`bundle`:

```go
func newStatusCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report vault initialization, kind, collections, and index health",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return runStatus(cmd, output) },
	}
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, json")
	return cmd
}
```

`runStatus` resolves `start` from `STARDUST_VAULT` or cwd (same precedence as `openService`), calls `service.GatherStatus`, and renders. `--output json` calls `emitJSON(cmd.OutOrStdout(), st)`. Otherwise a compact human block to `cmd.OutOrStdout`:

```
stardust status
  root:        /path/to/vault
  initialized: yes
  kind:        code-repo-with-docs
  collections:
    specs     3
    plans     2
    adr       28
    research  0
  index:
    notes:    142
    vectors:  on
    freshness: 4 commits behind HEAD
```

When not initialized:

```
stardust status
  root:        /path/here
  initialized: no
  kind:        plain-vault
  hint: run stardust init to initialize this directory
```

Rules for the human block: vectors prints `on` or `off (<reason>)`; freshness prints `N commits behind HEAD` when `HasCommitsBehind`, else `unknown (no git or unindexed)`; collections omits the sub-block when empty. Wire `newStatusCmd()` into `newRootCmd`'s `AddCommand` list.

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- **Detection in `service` or a new `detect` package.** Rejected. The question is a convention question and `convention` already owns the docs convention; a new package is overhead and `service` importing `convention` is cycle-free.
- **Extend `service.Status` instead of adding `GatherStatus`.** Rejected. `Status` is consumed by the `rpc`/`mcp` "status" handler with a fixed contract, and it is a method on an already-open `Service`, so it cannot express the uninitialized case. A separate package-level function keeps the existing contract intact.
- **`--docs` as a tri-state string flag (`auto|on|off`).** Rejected. Two bools with cobra `Changed()` is the idiomatic CLI shape, keeps `--docs` backward compatible, and reads clearly in help.
- **Treat `.git` as a primary code signal (above markdown-dominance).** Rejected. Stardust vaults are git-backed by design, so `.git` alone must not reclassify a human markdown vault. `.git` only breaks ties for non-markdown, non-obsidian, no-source-marker directories.
- **Recursive content sniff.** Rejected as premature. Top-level markers classify the real cases (repo roots, vault roots) and stay fast and predictable.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- A mixed repo (markdown docs site that is also a code repo) could be misclassified. Mitigated by the override flags and the printed detection line, which always names the escape hatch.
- `GatherStatus` probes Ollama via `svc.Status` (`embed.Available`), adding latency and a network touch to `status`. Acceptable: it mirrors existing `Status` behavior and is the honest vectors signal. JSON consumers that want a fast probe can be addressed later; not in scope.
- `convention` importing nothing new keeps the build clean, but `service` gains a `convention` import. Verified acyclic above.

</details>

<details>
<summary><b>Open questions</b></summary>
<br>

None blocking. The both-flags-set tiebreak (`--docs` wins) is decided and documented rather than left open.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- `convention` table tests: `go.mod`+`.git` dir to code repo; only `*.md` files to plain vault; an `.obsidian` dir to plain vault; an empty dir to plain vault; a `.git`-only non-markdown dir to code repo; markdown-dominant with a stray `.go` file resolves by precedence (source marker wins, code repo) and the inverse md-majority case is asserted.
- `cli/init` tests: with no flags in a `go.mod`+`.git` temp dir, the docs collections are scaffolded and the detection line is printed; in an `.obsidian` temp dir, they are not; `--docs` scaffolds regardless of a plain-vault dir; `--no-docs` skips regardless of a code-repo dir.
- `service.GatherStatus` tests: uninitialized temp dir returns `Initialized:false`, a kind, a non-empty hint, and nil error; an initialized temp vault returns `Initialized:true`, the expected kind, and collection counts that match scaffolded records.
- `cli/status` ANSI test: `status --output json` written to a real `os.Pipe` carries zero `\x1b[` bytes and unmarshals as JSON, following `TestPipedJSONOutputHasZeroANSI`.
- `go build ./...`, `go test ./...`, `gofmt -l` clean, `golangci-lint run` clean.
- Manual: `stardust status` in the Stardust repo prints `code-repo-with-docs` with the four collections and a freshness line.

</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- A `status --watch` or daemon mode.
- Exposing `GatherStatus` over `rpc`/`mcp`/HTTP. Structurally a thin later surface, deferred per the SPEC parity note.
- Detecting sub-kinds (monorepo, polyglot) or per-language docs presets.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. `convention.DetectKind` + `Kind` methods, with table tests.
2. Init wiring: `--no-docs` flag, `resolveInitDocs`, detection line, with CLI tests.
3. `service.GatherStatus` + `VaultStatus`/`IndexHealth` types, with service tests.
4. `cli/status.go` command, human + JSON render, root wiring, ANSI JSON test.

</details>

<details>
<summary><b>References</b></summary>
<br>

- `internal/cli/init.go`, `internal/cli/init_test.go`
- `internal/convention/convention.go`
- `internal/service/service.go` (`Status`, `ftsOnlyReason`), `internal/service/records.go` (`ListCollections`, `CollectionInfo`), `internal/service/bundle.go` (`commitsBehindHead`)
- `internal/config/config.go` (`FindRoot`, `ErrNoVault`, `Layout`)
- `internal/gitx/gitx.go` (`CommitCountSince`)
- `internal/cli/output.go` (`emitJSON`), `internal/cli/headless_ansi_test.go`
- `docs/adr/0027-init-auto-detect-default-policy.md`, `docs/adr/0028-status-command-and-json-contract.md`

</details>
