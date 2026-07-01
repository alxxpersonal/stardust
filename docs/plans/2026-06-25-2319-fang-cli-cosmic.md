---
title: Fang CLI cosmic rebuild plus informative errors - implementation plan
status: Done
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-2319-fang-cli-cosmic.md
  - docs/specs/2026-06-25-2319-informative-cli-errors.md
  - docs/adr/0009-fang-cobra-execute.md
  - docs/adr/0010-fang-headless-output-boundary.md
  - docs/adr/0011-stardust-cosmic-colorscheme.md
  - docs/adr/0012-cli-hint-error-type.md
  - docs/adr/0013-fang-error-handler-renders-suggestions.md
---

# Fang CLI cosmic rebuild plus informative errors - implementation plan

Wrap the stardust cobra tree with `fang.Execute`, theme its chrome with the cosmic colorscheme, add a `clierr.Hint` error type and a fang error handler that renders a `Run:` suggestion line, and prove the markdown-safe boundary with a hard zero-ANSI test on a piped JSON command.

## Header

- **Goal:** the CLI shell renders cosmic-violet help and errors via fang, gains `--version`/completions/manpage, surfaces actionable errors with a `Run:` line, and keeps every piped data path byte-for-byte raw.
- **Architecture:** export the cosmic palette to a leaf `internal/ui`; `cosmicColorScheme` maps it onto `fang.ColorScheme`; `cli.Execute()` swaps to `fang.Execute(ctx, root, opts...)`; a leaf `internal/clierr.Hint` carries actionable errors; a `fang.WithErrorHandler` renders message plus suggestion. Data writers (`emitMarkdown`/`emitJSON`) are untouched.
- **Tech stack:** Go 1.26.1, `charm.land/fang/v2`, `charm.land/lipgloss/v2`, cobra. One new external dep (fang).
- **Global constraints:** conventional commits, no co-author trailers, gofmt + `go test ./...` + `make lint` green, ZERO em or en dashes, no emoji, never panic, `%w` wrapping, doc comments on exports, `// --- Section ---` separators.

## Context

`internal/cli/root.go:21-26` runs `newRootCmd().Execute()` and prints a flat `error: %v`. The cosmic palette is duplicated unexported in `internal/tui/styles.go:13-21` and `internal/render/glamour.go:17-23`. Data output flows through `emitMarkdown`/`emitJSON` in `internal/cli/output.go`. Specs: `2026-06-25-2319-fang-cli-cosmic.md` and `2026-06-25-2319-informative-cli-errors.md`. ADRs 0009-0013.

## Reuse map (read first)

- `internal/cli/root.go` - `Execute`, `newRootCmd`, the `version` var, the flat error line.
- `internal/cli/output.go` - `emitMarkdown`, `emitJSON`, `isTTY` (the data writers; do NOT change behavior).
- `internal/tui/styles.go`, `internal/render/glamour.go` - the duplicated cosmic hexes to consolidate.
- `internal/config/config.go:111` - `ErrNoVault` sentinel.
- `internal/cli/context.go` - `resolveVault`, `openService` (where `ErrNoVault` is caught).
- `internal/cli/check.go:52`, `registry.go:65`, `hooks.go:28`, `sync.go` (95, 109, 176, 193) - error sites.

## Task 1: add fang, build

- Modify: `go.mod`, `go.sum`
- [x] `go get charm.land/fang/v2`.
- [x] `go build ./...` to confirm the vanity domain resolves like the other `charm.land/...v2` deps.
- [x] Commit `chore(cli): add charm.land/fang/v2`. (deferred: do-not-commit)

## Task 2: export the cosmic palette to a leaf package

- Create: `internal/ui/palette.go`
- Test: `internal/ui/palette_test.go`
- Produces: exported `lipgloss.Color` tokens `Primary`, `Secondary`, `Accent`, `Text`, `Muted`, `Border`, `Bg`, `CodeBg`.

- [x] Test: assert each token equals its locked hex (`Primary == lipgloss.Color("#a78bfa")`, ..., `CodeBg == lipgloss.Color("#16161e")`, `Bg == lipgloss.Color("#0a0a12")`).
- [x] Run, confirm fail (package missing).
- [x] Implement `internal/ui/palette.go` with the eight tokens and doc comments.
- [x] Point `internal/tui/styles.go` and `internal/render/glamour.go` at `internal/ui` (replace the local hexes; keep behavior identical). Verify no import cycle (`internal/ui` imports only lipgloss).
- [x] Run; `go build ./...`, `go test ./...` green.
- [x] Commit `refactor(ui): export the cosmic palette as a leaf package`. (deferred: do-not-commit)

## Task 3: the cosmic colorscheme

- Create: `internal/cli/color_scheme.go`
- Test: `internal/cli/color_scheme_test.go`
- Produces: `cosmicColorScheme(lipgloss.LightDarkFunc) fang.ColorScheme`.

- [x] Test: assert key field mappings (`Title == ui.Primary`, `Program == ui.Primary`, `Command == ui.Accent`, `Flag == ui.Accent`, `Base == ui.Text`, `FlagDefault == ui.Secondary`, `Dash == ui.Muted`, `Codeblock == ui.CodeBg`, `ErrorDetails == ui.Accent`).
- [x] Run, confirm fail.
- [x] Implement `cosmicColorScheme` mapping every `fang.ColorScheme` field from `internal/ui` tokens per ADR 0011; `ErrorHeader: [2]color.Color{ui.Bg, ErrorRed}` (cosmic-red #ff6b8a per the decision, NOT the pink accent). Confirmed the exact `fang.ColorScheme` field set against the adopted v2 source.
- [x] Run; loop to green.
- [x] Commit `feat(cli): map the cosmic palette onto fang ColorScheme`. (deferred: do-not-commit)

## Task 4: the clierr.Hint type

- Create: `internal/clierr/clierr.go`
- Test: `internal/clierr/clierr_test.go`
- Produces: `Hint{Message, Suggestion, cause}`, `New`, `Wrap`, `Error`, `Unwrap`.

- [x] Test: `New("x","y").Error() == "x (try: y)"`; `New("x","").Error() == "x"`; `errors.As` extracts a `*Hint` through a double `%w` wrap; `errors.Is(Wrap(config.ErrNoVault, ...), config.ErrNoVault)` is true.
- [x] Run, confirm fail.
- [x] Implement `internal/clierr/clierr.go` per ADR 0012 with doc comments; leaf package, no stardust imports.
- [x] Run; loop to green.
- [x] Commit `feat(clierr): add the Hint actionable-error type`. (deferred: do-not-commit)

## Task 5: swap Execute to fang with the colorscheme and error handler

- Modify: `internal/cli/root.go`
- Create: `internal/cli/error_handler.go`
- Test: `internal/cli/error_handler_test.go`

- [x] Add `commit` var and a `versionOpt()` helper in `root.go` (returns `fang.WithVersion(version)` when `version != "0.2.0-dev"`, else `fang.WithVersion("")` so fang falls back to build info).
- [x] Implement the `fang.WithErrorHandler` (`error_handler.go`) per ADR 0013: `errors.As` for `*clierr.Hint`, render `styles.ErrorText.Render(h.Message)` then a `Run:` line styled from `internal/ui` tokens when `Suggestion != ""`; else `fang.DefaultErrorHandler`. Confirmed `fang.Styles`/`DefaultErrorHandler` names against v2 source.
- [x] Test: render a `*clierr.Hint` with a suggestion to a `bytes.Buffer`, assert it contains the message AND a `Run:` line with the command; render a plain `errors.New("x")`, assert it matches the default handler output; render a hint with empty suggestion, assert no `Run:` line.
- [x] Swap `Execute()` body to `fang.Execute(ctx, root, versionOpt(), fang.WithCommit(commit), fang.WithColorSchemeFunc(cosmicColorScheme), fang.WithErrorHandler(...), fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM))`; create the signal context; remove the flat `error:` line; add `context`/`os/signal`/`syscall` imports.
- [x] Run; `go build ./...`, `go test ./...` green.
- [x] Commit `feat(cli): run the shell through fang with the cosmic theme and hint handler`. (deferred: do-not-commit)

## Task 6: convert the command-bearing error sites

- Modify: `internal/cli/context.go`, `internal/cli/check.go`, `internal/cli/registry.go`, `internal/cli/hooks.go`, `internal/cli/sync.go`
- Test: `internal/cli/errors_test.go`

- [x] Test (table): each command-bearing site returns a `*clierr.Hint` with a non-empty `Suggestion`: `ErrNoVault` wrapped at the CLI boundary -> `stardust init`; `check --strict` with errors -> `stardust check --fix`; `registry stale` with stale docs -> `stardust registry`; `hooks` non-git -> `git init`; `sync check failed` -> `stardust sync`.
- [x] Run, confirm fail.
- [x] Convert: in `resolveVault`/`openService`, catch `config.ErrNoVault` and return `clierr.Wrap(err, "no stardust vault found here", "stardust init")`. `check.go:52` -> `clierr.New(fmt.Sprintf("%d vault error(s)", res.Errors), "stardust check --fix")`. `registry.go:65` -> `clierr.New(fmt.Sprintf("%d stale docs", len(res.Docs)), "stardust registry")`. `hooks.go:28` -> `clierr.New(vc.Layout.Root+" is not a git repository", "git init")`. The `sync check failed` site -> `clierr.New(message, "stardust sync")`.
- [x] De-jargon the validation/flag sites: `new.go:80` drops the `new:` prefix (`%s already exists and is not empty`); `sync.go` 95/109/176/193 already state the value once with no package prefix. Left plain (no suggestion).
- [x] Run; loop to green. `make lint`.
- [x] Commit `feat(cli): give actionable errors a runnable suggestion`. (deferred: do-not-commit)

## Task 7: the hard zero-ANSI boundary test

- Create: `internal/cli/headless_ansi_test.go`

- [x] Test: build the root command, set args to `registry governs <path> --output json` against a temp indexed vault (the lightest data command with no git/Ollama dependency), redirect the command's out to an `os.Pipe` so `cmd.OutOrStdout()` is a non-tty, run it, read the captured bytes, and assert `bytes.Contains(out, []byte("\x1b[")) == false` AND that the bytes parse as JSON.
- [x] Run, confirm it passes with the fang swap in place (the boundary is structural).
- [x] Commit `test(cli): assert piped JSON output carries zero ANSI escapes`. (deferred: do-not-commit)

## Verification

- `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .` empty, `make lint` clean; zero em or en dashes, no emoji.
- Colorscheme unit test: key fields equal the cosmic tokens.
- clierr unit test: `Error()` one line, `errors.As` through double wrap, `errors.Is(ErrNoVault)` preserved.
- Error-handler buffer test: `Run:` line for hints, default for plain errors, no `Run:` for empty suggestion.
- Command-bearing table test: every actionable site carries a non-empty `Suggestion`.
- Zero-ANSI test: piped `--output json` has no `\x1b[` bytes and parses as JSON.
- Manual: `stardust --help` cosmic violet; `stardust bogus` styled ERROR; `stardust --version` prints version; `stardust completion bash` and `stardust man` emit; `stardust <data cmd> --output json | cat` raw; `NO_COLOR=1 stardust --help` plain; an uninitialised dir shows the `Run: stardust init` line.

## Self-review gate

- Every spec Work-breakdown item maps to a task across both specs.
- `emitMarkdown`/`emitJSON` behavior is byte-identical to today (data path untouched).
- The cosmic palette has exactly one source (`internal/ui`); no third copy.
- The `fang.ColorScheme` and `fang.Styles`/`DefaultErrorHandler` field/function names are confirmed against the adopted v2 source, not memory.
- `config.ErrNoVault` stays an `errors.Is`-checkable sentinel through the `clierr.Wrap`.
- No em or en dashes in any new file.
