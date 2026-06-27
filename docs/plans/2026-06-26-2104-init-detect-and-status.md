---
title: Init auto-detect and a status command
type: plan
status: Draft
created: 2026-06-26
updated: 2026-06-26
related:
  - docs/specs/2026-06-26-2104-init-detect-and-status.md
  - docs/adr/0027-init-auto-detect-default-policy.md
  - docs/adr/0028-status-command-and-json-contract.md
---

Implement `stardust init` auto-detection of repo vs vault and a new `stardust status` command, TDD, one bite-sized task at a time.

# Header

- **Goal.** `stardust init` defaults the docs convention by sniffing the directory when neither `--docs` nor `--no-docs` is set, and `stardust status` reports init state, kind, collections with counts, index health, and freshness as a compact block or ANSI-free JSON.
- **Architecture.** Detection is a pure `convention.DetectKind`. Status data gathering is a package-level `service.GatherStatus` composing existing service reads. Both CLI surfaces stay thin.
- **Tech stack.** Go 1.26.1, module `github.com/alxxpersonal/stardust`. cobra CLI, `modernc.org/sqlite` (pure Go, never cgo), testify.
- **Global constraints.** Pure-Go, never cgo. NEVER panic; return `%w`-wrapped errors. `errors.Is` for sentinels. NO em dash or en dash anywhere in code, comments, strings, or docs. `// --- Section ---` separators. Doc comments on every export, third-person present tense. `gofmt` and `golangci-lint` clean. Do not commit, push, or `go install`.

# Context

Run all `go` commands from `~/Desktop/Stardust`. Existing behavior to preserve: `init --docs` still scaffolds; `service.Status` and the `rpc`/`mcp` "status" handler are untouched.

# Reuse map (read first)

- `internal/cli/init.go` - `newInitCmd`, `runInit`, `scaffoldVault(ctx, root, check, docs)`.
- `internal/cli/init_test.go` - test patterns (`t.Chdir`, `newInitCmd`, scaffold assertions).
- `internal/convention/convention.go` - where `DetectKind` lands; `DefaultDocCollections`.
- `internal/service/service.go` - `(*Service).Status`, `Status` struct, `ftsOnlyReason`, `Open`.
- `internal/service/records.go` - `(*Service).ListCollections`, `CollectionInfo`.
- `internal/service/bundle.go` - `(*Service).commitsBehindHead(ctx) (int, bool)`.
- `internal/config/config.go` - `FindRoot`, `ErrNoVault`, `Layout`, `DirName`.
- `internal/cli/context.go` - `openService`, `STARDUST_VAULT` precedence.
- `internal/cli/output.go` - `emitJSON`.
- `internal/cli/registry.go` - thin-command + `--output` pattern to mirror.
- `internal/cli/headless_ansi_test.go` - `TestPipedJSONOutputHasZeroANSI`, the ANSI-free JSON proof to mirror.
- `internal/cli/root.go` - `newRootCmd` `AddCommand` list to wire `newStatusCmd`.

Confirm every signature in source before calling it; do not trust this plan over the code.

---

## Task 1 - convention.DetectKind

Files
- Create: `internal/convention/detect.go`
- Test: `internal/convention/detect_test.go`

Interfaces
- Produces: `convention.Kind` (int) with `KindPlainVault`, `KindCodeRepo`; methods `WantsDocs() bool`, `Label() string`, `Describe() string`; `DetectKind(dir string) (Kind, error)`.
- Consumes: stdlib `os`, `path/filepath`, `strings`.

Steps
1. [ ] Write `detect_test.go` table test `TestDetectKind` with temp dirs built per case, asserting the returned `Kind`:
   - `go.mod` file + `.git` dir to `KindCodeRepo`.
   - only `a.md`, `b.md` to `KindPlainVault`.
   - an `.obsidian` dir (plus an `a.md`) to `KindPlainVault`.
   - empty dir to `KindPlainVault`.
   - `.git` dir only, no markdown, no source markers, plus one `notes.txt` to `KindCodeRepo`.
   - `main.go` present alongside two `*.md` files to `KindCodeRepo` (source marker beats markdown count).
   - `.obsidian` dir present alongside `main.go` to `KindPlainVault` (obsidian wins outright).
   Also assert `KindCodeRepo.WantsDocs()` is true, `KindPlainVault.WantsDocs()` is false, `KindCodeRepo.Label()` is `"code-repo-with-docs"`, `KindPlainVault.Label()` is `"plain-vault"`, and both `Describe()` strings contain the right override flag (`--no-docs` for code repo, `--docs` for plain vault).
2. [ ] Run `go test ./internal/convention/ -run TestDetectKind`; confirm it fails (no symbol).
3. [ ] Implement `detect.go`:
   - `Kind` type, constants, `WantsDocs`, `Label`, `Describe` (exact strings from the spec Approach section).
   - `DetectKind`: read `os.ReadDir(dir)`; on error return `KindPlainVault, fmt.Errorf("detect kind in %s: %w", dir, err)`. Single pass collecting: `hasObsidian` (dir entry `.obsidian`, IsDir), `hasGit` (dir entry `.git`, IsDir), `hasSourceMarker`, `mdCount`, `nonMdCount`. Source markers: entry name in `{go.mod, package.json, Cargo.toml}`, or a regular file whose ext is in `{.go, .ts, .py, .rs}`. Markdown: regular file ext `.md` increments `mdCount`; any other regular file that is not a dotfile increments `nonMdCount`. Skip dotfiles from `nonMdCount`. Apply precedence: obsidian to plain; source marker to code; (`mdCount > 0 && mdCount >= nonMdCount`) to plain; git to code; else plain.
   - Doc comments on every export, `// --- ... ---` section header.
4. [ ] Run `go test ./internal/convention/ -run TestDetectKind`; loop until green.
5. [ ] `gofmt -l internal/convention/` and `golangci-lint run ./internal/convention/...`; fix any output.

Deliverable: pure detection function, table-tested, green.

---

## Task 2 - init wiring (--no-docs, detection line)

Files
- Modify: `internal/cli/init.go`
- Test: `internal/cli/init_test.go`

Interfaces
- Consumes: `convention.DetectKind`, `convention.Kind`, `cmd.Flags().Changed`.
- Produces: resolved scaffold bool + printed detection line.

Steps
1. [ ] Add tests to `init_test.go`:
   - `TestInitAutoDetectsCodeRepo`: temp dir with `go.mod` + `.git` dir, `t.Chdir`, run `newInitCmd()` with no args, capture stdout via `cmd.SetOut(buf)`; assert `specs/config.toml` exists under `.stardust/collections/` and stdout contains `detected a code repo`.
   - `TestInitAutoDetectsPlainVault`: temp dir with an `.obsidian` dir, no args; assert `specs/config.toml` does NOT exist and stdout contains `detected a plain vault`.
   - `TestInitNoDocsOverridesCodeRepo`: temp dir with `go.mod` + `.git`, args `--no-docs`; assert no `specs/config.toml`.
   - `TestInitDocsOverridesPlainVault`: temp dir with `.obsidian`, args `--docs`; assert `specs/config.toml` exists.
2. [ ] Run `go test ./internal/cli/ -run 'TestInit'`; confirm new cases fail.
3. [ ] Implement in `init.go`:
   - In `newInitCmd`, add `var noDocs bool` and `cmd.Flags().BoolVar(&noDocs, "no-docs", false, "skip the docs collections even if the directory looks like a code repo")`. Pass both into `runInit`.
   - Add `resolveInitDocs(cmd *cobra.Command, dir string, docs, noDocs bool) (scaffold bool, line string)`: if `cmd.Flags().Changed("docs")` return `true, ""`; else if `cmd.Flags().Changed("no-docs")` return `false, ""`; else `kind, err := convention.DetectKind(dir)`; on err return `false, ""`; else return `kind.WantsDocs(), kind.Describe()`.
   - In `runInit(cmd, docs, noDocs)`: compute `scaffold, line := resolveInitDocs(cmd, cwd, docs, noDocs)`; if `line != ""` print it to `cmd.OutOrStdout()` before the `Initialised` line; call `scaffoldVault(cmd.Context(), cwd, "off", scaffold)`. Update the `runInit` signature and its caller; doc comment stays accurate.
4. [ ] Run `go test ./internal/cli/ -run 'TestInit'`; loop until green. Confirm `TestInitDocsScaffold` and `TestInitNoDocs` still pass (existing behavior preserved).
5. [ ] `gofmt -l internal/cli/init.go` and `golangci-lint run ./internal/cli/...`; fix any output.

Deliverable: `init` auto-detects with override flags and a printed line, existing tests intact.

---

## Task 3 - service.GatherStatus

Files
- Create: `internal/service/status_report.go`
- Test: `internal/service/status_report_test.go`

Interfaces
- Produces: `service.VaultStatus`, `service.IndexHealth` (both fully `json`-tagged per the spec), `service.GatherStatus(ctx context.Context, start string) (VaultStatus, error)`.
- Consumes: `config.FindRoot`, `config.ErrNoVault`, `Open`, `(*Service).Status`, `(*Service).ListCollections`, `(*Service).commitsBehindHead`, `convention.DetectKind`, `ftsOnlyReason`.

Steps
1. [ ] Write `status_report_test.go`:
   - `TestGatherStatusUninitialized`: `dir := t.TempDir()` with an `.obsidian` subdir; `st, err := GatherStatus(ctx, dir)`; require no error, `st.Initialized` false, `st.Kind == "plain-vault"`, `st.Hint` non-empty, `st.Collections` non-nil (len 0).
   - `TestGatherStatusInitialized`: build a temp vault: `scaffoldVault(ctx, dir, "off", true)` (reuse the cli scaffolder is not importable from service; instead create `.stardust` via `config` + write a collections config, OR call `service.Open` after running the real init path). Simpler: in the test, create the vault by writing `.stardust/config.toml` via `config.Save(config.Layout{Root: dir}.Config(), config.Default())` and a `go.mod` file so kind is code repo; then `GatherStatus`; require `st.Initialized` true, `st.Kind == "code-repo-with-docs"`, `st.Index.Notes == 0`. (Counts of records are exercised by the existing `ListCollections` tests; here assert the composition and kind.)
   Confirm the exact helper for creating a `.stardust` in tests by reading a sibling `service` test (for example `query_test.go` or `records_test.go`) and reuse its setup helper.
2. [ ] Run `go test ./internal/service/ -run TestGatherStatus`; confirm it fails (no symbol).
3. [ ] Implement `status_report.go`:
   - `VaultStatus` and `IndexHealth` structs exactly as in the spec Approach, every field `json`-tagged.
   - `GatherStatus`: `root, err := config.FindRoot(start)`; if `errors.Is(err, config.ErrNoVault)`: `kind, _ := convention.DetectKind(start)`; return `VaultStatus{Root: start, Initialized: false, Kind: kind.Label(), Collections: []CollectionInfo{}, Hint: "run stardust init to initialize this directory"}, nil`. If `err != nil` return `VaultStatus{}, err`. Else `svc, err := Open(ctx, root)`; on err return it; `defer func() { _ = svc.Close() }()`. Gather `st, err := svc.Status(ctx)` (return on err), `cols, err := svc.ListCollections(ctx)` (return on err), `behind, hasBehind := svc.commitsBehindHead(ctx)`, `kind, _ := convention.DetectKind(root)`. `reason := ""; if !st.Vectors { reason = ftsOnlyReason }`. If `cols == nil { cols = []CollectionInfo{} }`. Build and return `VaultStatus`.
   - Doc comments on every export, `// --- ... ---` section header, `%w` wrapping on any new error context.
4. [ ] Run `go test ./internal/service/ -run TestGatherStatus`; loop until green.
5. [ ] `gofmt -l internal/service/` and `golangci-lint run ./internal/service/...`; fix any output.

Deliverable: composed status report covering both states, service-tested, green.

---

## Task 4 - cli status command + ANSI JSON test

Files
- Create: `internal/cli/status.go`
- Modify: `internal/cli/root.go` (add `newStatusCmd()` to `AddCommand`)
- Test: `internal/cli/status_test.go`

Interfaces
- Produces: `newStatusCmd() *cobra.Command`, `runStatus(cmd, output)`.
- Consumes: `service.GatherStatus`, `emitJSON`, `STARDUST_VAULT`/cwd resolution.

Steps
1. [ ] Write `status_test.go`:
   - `TestStatusJSONHasZeroANSI`: mirror `TestPipedJSONOutputHasZeroANSI`. Build an initialized temp vault, `t.Setenv("STARDUST_VAULT", root)`, `newRootCmd()`, `SetOut(pw)`/`SetErr(pw)` on a real `os.Pipe`, args `{"status", "--output", "json"}`, read concurrently, assert zero `\x1b[` bytes and `json.Unmarshal` succeeds into a `map[string]any` with an `initialized` key.
   - `TestStatusHumanUninitialized`: temp dir with no `.stardust`, `t.Setenv("STARDUST_VAULT", dir)`, run `newRootCmd()` with `{"status"}` into a `bytes.Buffer`; assert output contains `initialized: no` and `stardust init`.
   Reuse the vault setup helper found in step-1 reading.
2. [ ] Run `go test ./internal/cli/ -run TestStatus`; confirm it fails (no command).
3. [ ] Implement `status.go`:
   - `newStatusCmd`: `Use: "status"`, `Args: cobra.NoArgs`, `--output` string flag defaulting `"auto"` with help `"output mode: auto, json"`, `RunE` calling `runStatus(cmd, output)`. Doc comment on `newStatusCmd`.
   - `runStatus(cmd *cobra.Command, output string) error`: resolve `start` (`os.Getenv("STARDUST_VAULT")`, else `os.Getwd()` with `%w` wrap); `st, err := service.GatherStatus(cmd.Context(), start)`; on err return it. If `output == "json"` return `emitJSON(cmd.OutOrStdout(), st)`. Else write the compact human block (spec Approach) to `cmd.OutOrStdout()`: header `stardust status`, `root`, `initialized` yes/no, `kind`; when initialized, a `collections:` sub-block (skip when empty) listing `name count`, and an `index:` block with `notes`, `vectors` (`on` or `off (<reason>)`), `freshness` (`N commits behind HEAD` when `HasCommitsBehind`, else `unknown (no git or unindexed)`); when not initialized, a `hint:` line. Use `fmt.Fprintf`/`Fprintln`, no ANSI.
4. [ ] Add `newStatusCmd(),` to the `root.AddCommand(...)` list in `root.go`.
5. [ ] Run `go test ./internal/cli/ -run TestStatus`; loop until green.
6. [ ] `gofmt -l internal/cli/` and `golangci-lint run ./internal/cli/...`; fix any output.

Deliverable: `stardust status` wired, human + ANSI-free JSON, tested, green.

---

# Verification

1. [ ] `go build ./...` clean.
2. [ ] `go test ./...` all green.
3. [ ] `gofmt -l .` prints nothing.
4. [ ] `golangci-lint run` clean.
5. [ ] Dash guard: no em or en dash in new files. Run `LC_ALL=C grep -rnP $'\xe2\x80\x94|\xe2\x80\x93'` over the new Go files and the four new docs; it MUST return nothing.
6. [ ] Manual: build to a temp path (not `go install`), run `stardust status` in the repo root, confirm `kind: code-repo-with-docs`, the four collections with counts, and a freshness line. Run `stardust status --output json | head` and confirm clean JSON.
7. [ ] Manual: `mkdir`-test an empty dir, `stardust init` in it, confirm the printed detection line and the resulting scaffold (or absence) match the kind.

# Self-review gate

- [ ] No placeholders or TBDs; every new export has a doc comment.
- [ ] Names and types match the spec and across tasks (`Kind`, `DetectKind`, `WantsDocs`, `Label`, `Describe`, `GatherStatus`, `VaultStatus`, `IndexHealth`).
- [ ] `service.Status` and the `rpc`/`mcp` "status" handler are untouched.
- [ ] `init --docs` and plain `init` behavior preserved (`TestInitDocsScaffold`, `TestInitNoDocs` still pass).
- [ ] JSON output is ANSI-free and `Collections` serializes as `[]`, never `null`.
- [ ] Every spec requirement maps to a task: detection (T1), init flags + line (T2), service gatherer both states (T3), CLI human + JSON + wiring (T4).
