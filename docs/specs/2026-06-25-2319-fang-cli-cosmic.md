---
title: Fang-styled stardust CLI with the cosmic colorscheme
status: Draft
version: 1
date: 2026-06-25
related:
  - docs/adr/0009-fang-cobra-execute.md
  - docs/adr/0010-fang-headless-output-boundary.md
  - docs/adr/0011-stardust-cosmic-colorscheme.md
  - docs/plans/2026-06-25-2319-fang-cli-cosmic.md
  - cmd/stardust/main.go
  - internal/cli/root.go
  - internal/tui/styles.go
  - internal/render/glamour.go
---

# Fang-styled stardust CLI with the cosmic colorscheme

Wrap the existing cobra tree with `fang.Execute` so help, usage, and errors render in the Stardust cosmic palette (violet accent, silver-white text) and the CLI gains auto `--version`, shell completions, and a manpage, while every headless data path (`--output md`, `--output json`, `--output plain`, query/bundle/graph/check/registry/digest/cron/sync output) stays raw and pipe-safe, proven by a test that asserts zero ANSI escape bytes in piped JSON output.

<details>
<summary><b>Problem</b></summary>
<br>

The stardust CLI prints errors as a flat `error: %v` line to stderr (`internal/cli/root.go:23`) and inherits cobra's unstyled default help and usage. There is no `--version` flag (only a manual `version` subcommand at `root.go:72-81`), no shell completions, and no manpage. The TUI is richly themed with the cosmic palette (`internal/tui/styles.go:13-21`) and the glamour renderer mirrors it (`internal/render/glamour.go:17-23`), but the command shell around the CLI is bare cobra default.

Adding styled help, a styled error block, a `--version` flag, completions, and a manpage by hand means reimplementing what `fang` already does, and any hand-rolled styling would drift from the cosmic palette that the TUI and glamour renderer already share.

The constraint that makes this non-trivial: stardust is an agent-facing tool whose primary output is markdown and JSON data. Its machine surfaces (`query --output json`, `bundle --output json`, `graph --output json`, `check --output json`, `digest --output json`, `cron list --output json`, `sync --output json`, `registry`, and the plain-markdown path of every `--output md`/`plain` command) MUST stay byte-for-byte raw when piped. The auto mode already glamour-renders on a TTY and emits plain markdown when piped (`internal/cli/output.go:24-30`). Any styling layer that leaks ANSI escape sequences into that data corrupts the agent contract and breaks downstream parsers. The fix MUST add chrome to the human surfaces (help, usage, errors) without touching the data surfaces.

</details>

<details>
<summary><b>Context and background</b></summary>
<br>

Entry point and command tree:

- `cmd/stardust/main.go:9-11` is a thin main that calls `cli.Execute()`.
- `internal/cli/root.go:21-26` is `Execute()`: it runs `newRootCmd().Execute()` and on error writes `fmt.Fprintln(os.Stderr, "error:", err)` then `os.Exit(1)`.
- `newRootCmd()` (`root.go:29-69`) builds the root with `SilenceUsage: true` and `SilenceErrors: true`, a no-subcommand `RunE` that launches the TUI on a TTY and prints help when piped, and adds 17 subcommands: init, index, query, graph, check, bundle, remember, registry, sync, digest, new, archive, serve, cron, hooks, rebuild, version. Several of those have children (registry stale/governs, sync init/report, new spec/plan/adr, cron list/run, hooks install/uninstall).
- Version lives as a package var `version = "0.2.0-dev"` (`root.go:18`), overridable via ldflags, surfaced only by the manual `version` subcommand.

Output mode is per-command, not global. There is no `--json`/`--md` global flag. Each data command takes `--output auto|md|json|plain` and routes through `emitMarkdown`/`emitJSON` in `internal/cli/output.go`:

- `emitMarkdown(w, md, mode)` (`output.go:24-30`): in `auto` mode on a TTY it glamour-renders via `render.GlamourRender`; otherwise (md, plain, or piped auto) it writes raw `strings.TrimRight(md, "\n")`.
- `emitJSON(w, v)` (`output.go:33-40`): indented JSON, always raw, no styling.
- `isTTY()` (`output.go:16-19`) checks `os.Stdout.Fd()` via `go-isatty`.

Data writers (stdout, must stay raw): every `emitMarkdown(cmd.OutOrStdout(), ...)` and `emitJSON(cmd.OutOrStdout(), ...)` call across query, bundle, graph, check, digest, registry, cron list, sync; plus `new spec/plan/adr` printing the created path (`new.go`), and `registry` writing the INDEX file then a stderr confirmation.

Operational logging (stderr): index summary (`index.go:62`), rebuild progress, remember action, serve listen address, hooks install confirmation. These are user info, never parsed as data, and already go to stderr.

Cosmic palette (the brand identity, already defined twice and identical):

- `internal/tui/styles.go:13-21`: `colorPrimary #a78bfa` (violet), `colorSecondary #c4b5fd` (light violet), `colorAccent #f0abfc` (pink), `colorText #e9e7ff` (silver-white), `colorMuted #7c7ca0` (muted gray-violet), `colorBorder #4c4c6d` (dark violet border), `colorBg #0a0a12` (near-black).
- `internal/render/glamour.go:17-23`: the same hexes as glamour constants, plus `clrCodeBg #16161e` for code background.

Both are unexported and CLI-invisible. The CLI currently renders no color at all.

Fang facts (authoritative, from the charm-fang skill and `charm.land/fang/v2`):

- Import path is `charm.land/fang/v2` (the charm.land vanity domain, matching this codebase's `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/glamour/v2`, `charm.land/bubbles/v2`). Install with `go get charm.land/fang/v2`. NOT `github.com/charmbracelet/fang`.
- Fang WRAPS cobra. Every `*cobra.Command` stays exactly as written. The swap is one line: `newRootCmd().Execute()` becomes `fang.Execute(ctx, root, opts...)`.
- `func Execute(ctx context.Context, root *cobra.Command, options ...Option) error`. Inside, fang sets `root.SilenceUsage = true` and `root.SilenceErrors = true`, installs a styled help func and a styled error handler, and (unless `WithoutVersion`) sets `root.Version` from `WithVersion` or `debug.ReadBuildInfo`.
- Fang renders help and errors through `colorprofile.NewWriter(c.OutOrStdout()/ErrOrStderr(), os.Environ())`, which downgrades to no-color on a non-tty and honors `NO_COLOR`. lipgloss (`charm.land/lipgloss/v2`) does the same.
- Options: `WithVersion(v string)`, `WithCommit(sha string)`, `WithoutVersion()`, `WithoutCompletions()`, `WithoutManpage()`, `WithNotifySignal(sigs ...os.Signal)`, `WithColorSchemeFunc(fn ColorSchemeFunc)`, `WithTheme(theme ColorScheme)`, `WithErrorHandler(fn ErrorHandler)`.
- `WithColorSchemeFunc` takes `func(lipgloss.LightDarkFunc) fang.ColorScheme`. The `fang.ColorScheme` struct fields (to confirm against v2 source at adoption): `Base`, `Title`, `Description`, `Codeblock`, `Program`, `DimmedArgument`, `Comment`, `Flag`, `FlagDefault`, `Command`, `QuotedString`, `Argument`, `Help`, `Dash`, all `color.Color`; `ErrorHeader [2]color.Color` (0=fg, 1=bg); `ErrorDetails color.Color`. `lipgloss.Color` satisfies `color.Color`.

Markdown-safe boundary: fang only intercepts error and help/usage rendering. The data that each `RunE` writes to `cmd.OutOrStdout()` via `emitMarkdown`/`emitJSON` is written by the command itself and is never touched by fang.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. The stardust CLI shell renders help, usage, and errors through `fang.Execute` in the cosmic palette, with a styled `ERROR` header block instead of the flat `error: %v` line.
2. The CLI gains an auto `--version` flag (from `WithVersion` plus `WithCommit`, falling back to build info), a `completion` subcommand, and a hidden `man` subcommand, with signal handling wired through the context.
3. A single `fang.ColorScheme` derived from the cosmic palette is the source of truth for shell chrome colors, mapped field by field onto exported palette tokens. The accent identity is violet primary `#a78bfa` with silver-white text `#e9e7ff`, distinct from any monochrome scheme.
4. Every headless data surface (`--output json`, `--output md`, `--output plain`, and the piped-auto markdown path of query, bundle, graph, check, digest, registry, cron list, sync) stays byte-for-byte raw when piped, with zero ANSI escape bytes, proven by a test that captures stdout as a non-tty pipe.
5. Build, `go test ./...`, `go vet`, `gofmt`, and `make lint` stay green, with no em or en dashes, no emoji, and no panics.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No change to the command tree, the per-command `--output` flag set, the `emitMarkdown`/`emitJSON` logic, or any `RunE` body. Fang wraps, it does not rewrite.
- No restyling of the glamour-rendered or plain markdown data output. That is data, not chrome, and stays exactly as it is.
- No light-mode shell theme. The cosmic palette is dark only; the colorscheme func ignores its `ld` argument.
- No change to the TUI, the glamour renderer, or the serve/MCP surfaces.
- The manual `version` subcommand may stay or be removed; it is redundant once `--version` exists but harmless. The plan keeps it for backward compatibility and lets `--version` be the new path.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

Three units: export the palette, write the cosmic colorscheme, swap Execute. All shell wiring lives in `internal/cli`.

1. Export the cosmic palette (`internal/cli/palette.go`, new file, or promote from `internal/tui`). The palette hexes are currently duplicated unexported in `internal/tui/styles.go` and `internal/render/glamour.go`. To give the CLI a single typed source, add a small exported palette in a CLI-reachable spot. The cleanest option is a new leaf `internal/ui` (or `internal/palette`) package exporting `lipgloss.Color` tokens, which both the TUI and the new colorscheme import, collapsing the duplication. The minimal option is a private `palette.go` in `internal/cli` that re-declares the same hexes as exported-within-package `lipgloss.Color` vars. The plan picks the leaf-package option to kill the duplication, but the colorscheme mapping is identical either way.

   Tokens (exact hexes, the locked cosmic identity):

   ```
   Primary   #a78bfa  violet        (titles, program name)
   Secondary #c4b5fd  light violet  (flag defaults, help hint)
   Accent    #f0abfc  pink          (commands, flags)
   Text      #e9e7ff  silver-white  (body, args, descriptions)
   Muted     #7c7ca0  muted violet  (comments, dimmed args, dashes)
   Border    #4c4c6d  dark violet
   CodeBg    #16161e  code block bg
   Error     #f0abfc  pink-on-dark  (error header/details, see colorscheme)
   ```

2. Cosmic colorscheme (`internal/cli/color_scheme.go`, new file). One function maps the palette onto every `fang.ColorScheme` field. This is the KEY difference from a monochrome adoption: the title and program read violet, commands and flags read pink accent, body text reads silver-white, comments and dashes read muted violet.

   ```go
   package cli

   import (
       "image/color"

       fang "charm.land/fang/v2"
       "charm.land/lipgloss/v2"

       "github.com/alxxpersonal/stardust/internal/ui"
   )

   // cosmicColorScheme maps the Stardust cosmic palette onto fang's chrome.
   // The theme is dark only, so the lipgloss.LightDarkFunc argument is ignored.
   func cosmicColorScheme(lipgloss.LightDarkFunc) fang.ColorScheme {
       return fang.ColorScheme{
           Base:           ui.Text,      // silver-white body text
           Title:          ui.Primary,   // violet title
           Description:    ui.Text,      // flag/command descriptions
           Codeblock:      ui.CodeBg,    // code background
           Program:        ui.Primary,   // program name, violet
           Command:        ui.Accent,    // subcommand names, pink
           DimmedArgument: ui.Muted,     // dimmed args
           Comment:        ui.Muted,     // comments
           Flag:           ui.Accent,    // flag names, pink
           FlagDefault:    ui.Secondary, // flag defaults, light violet
           Argument:       ui.Text,      // positional args
           QuotedString:   ui.Secondary, // quoted strings
           Help:           ui.Secondary, // help hint
           Dash:           ui.Muted,     // flag dashes
           ErrorHeader:    [2]color.Color{ui.Bg, ui.Accent}, // near-black fg on pink bg
           ErrorDetails:   ui.Accent,    // error body, pink
       }
   }
   ```

   `lipgloss.Color` satisfies `color.Color`, so tokens slot straight in. The `ErrorHeader` is near-black text on a pink-accent background so a real error reads as a distinct cosmic alert, not the default red. This is a design choice flagged in Open questions; pink `#f0abfc` on near-black `#0a0a12` keeps the cosmic identity even in the error block. The exact field set MUST be confirmed against the pinned `charm.land/fang/v2` version at adoption; a field rename would fail to compile and be caught immediately.

3. Execute swap (`internal/cli/root.go`). Replace the body of `Execute()` and give it a context:

   ```go
   func Execute() {
       root := newRootCmd()
       ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
       defer stop()
       if err := fang.Execute(
           ctx,
           root,
           versionOpt(),
           fang.WithCommit(commit),
           fang.WithColorSchemeFunc(cosmicColorScheme),
           fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
       ); err != nil {
           os.Exit(1)
       }
   }
   ```

   Fang sets `SilenceUsage`/`SilenceErrors` itself, so the existing settings in `newRootCmd()` are redundant but harmless and stay. The flat `fmt.Fprintln(os.Stderr, "error:", err)` line is removed; fang owns error rendering now. `WithNotifySignal` plus `signal.NotifyContext` give a single cancellation path into every `RunE` via `cmd.Context()`, which the long-running `serve` command can select on. `versionOpt()` returns `fang.WithVersion(version)` when `version != "0.2.0-dev"`, else nothing, so `go install` builds fall back to `debug.ReadBuildInfo`.

Headless safety is structural, not added. Fang's writers wrap `c.OutOrStdout()` / `c.ErrOrStderr()` through `colorprofile.NewWriter`, which is only used for help and error rendering. The data path is `emitMarkdown`/`emitJSON` to `cmd.OutOrStdout()`, which fang never wraps. lipgloss and colorprofile both downgrade to no-color on a non-tty and honor `NO_COLOR`, so even the chrome is plain when piped. The test in the plan proves the data path specifically: capture `query <q> --output json` with stdout redirected to a pipe (non-tty) and assert the bytes contain no `\x1b[`.

Data flow:

```
fang.Execute(ctx, root, opts)
  | sets root.Version, help func, error handler (chrome only)
  | dispatches to cobra as before
  v
cobra runs the matched RunE
  |
  +-- human path: help/usage/error -> fang styled writer (colorprofile, NO_COLOR-aware)
  |
  +-- data path:  emitMarkdown/emitJSON(cmd.OutOrStdout(), ...) -> process stdout (fang never touches)
```

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Keep plain cobra and hand-roll lipgloss help and error templates. Rejected: reimplements fang, drifts from the cosmic palette, and still needs manual version/completion/manpage wiring.
- Reuse a monochrome scheme like the exo-jobs adoption. Rejected: stardust is the vault/stars engine and already owns a cosmic violet identity in its TUI and glamour renderer; the CLI shell SHOULD match that brand, not a gray one. The colorscheme is the deliberate point of divergence from the exo-jobs template.
- Use `WithTheme(fang.ColorScheme{...})` instead of `WithColorSchemeFunc`. Rejected for the primary path: `WithColorSchemeFunc` is the documented light/dark-adaptive entry point and matches the skill guidance; the func ignores `ld` here because the theme is dark only, which is explicit and fine. `WithTheme` works and is noted as a fallback if the func form causes friction.
- Leave the palette duplicated in `tui` and `render` and add a third copy in `cli`. Rejected: three copies of seven hexes drift. The plan promotes one exported `internal/ui` palette that all three import.
- Pin the version with `WithoutVersion()` and keep only the manual `version` subcommand. Rejected: loses fang free `--version` and the build-info fallback for no gain.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- Fang could wrap stdout in a way that colors data. Mitigation: the markdown-safe boundary is asserted by a hard test (zero `\x1b[` in piped `--output json`), not assumed. If the test fails, the boundary is broken and the build fails.
- `go get charm.land/fang/v2` must resolve through the charm.land vanity domain like the other charm v2 deps already do. Mitigation: the codebase already pulls four `charm.land/...v2` modules, so the vanity domain is proven here; the plan first task is the `go get` and a build.
- Fang `ColorScheme` field set could differ across versions. Mitigation: the mapping is pinned to the adopted v2 field set verified from source; a version bump that changes fields fails to compile and is caught immediately.
- Exporting the palette into a new `internal/ui` package could create an import cycle if `tui` and `render` are restructured carelessly. Mitigation: `internal/ui` is a leaf with only `charm.land/lipgloss/v2` as a dependency; `tui`, `render`, and `cli` import it, it imports none of them.
- Signal handling via `WithNotifySignal` plus `signal.NotifyContext` could double-register. Mitigation: both target the same signals and only cancel the shared context; `serve` already selects on context, so cancellation is idempotent.

</details>

<details>
<summary><b>Open questions</b></summary>
<br>

1. Error block background color. The colorscheme uses pink accent `#f0abfc` as the error header background (near-black text on pink), keeping the cosmic identity instead of the conventional red. ALTERNATIVE: a dedicated cosmic error red, for example `#ff6b8a` (pink-leaning red) so errors read clearly as failures while staying on-brand. DEFAULT: pink-accent background. FLAGGED FOR alxx to confirm: pink accent error block, or add a distinct error-red token.
2. Whether to keep the manual `version` subcommand once `--version` exists. Default keep, harmless. Remove later if desired.
3. Whether to pass `WithoutManpage()`/`WithoutCompletions()`. Default keep both on; they are free and useful.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- `go build ./...`
- `go test ./...` (race optional, the suite is not concurrency-heavy)
- `go vet ./...`
- `gofmt -l internal/cli` returns empty
- `make lint` clean
- The new headless-safety test passes: it runs a data command (`query` or `registry`) with `--output json` and stdout redirected to an `os.Pipe` (non-tty), reads the captured bytes, and asserts `bytes.Contains(out, []byte("\x1b[")) == false` plus that the JSON parses.
- A colorscheme unit test asserts key fields equal the palette tokens (Title == ui.Primary, Command == ui.Accent, Base == ui.Text).
- Manual, adversarial: `stardust --help` shows cosmic violet help; `stardust bogus` shows a styled `ERROR` block on stderr; `stardust --version` prints the version; `stardust completion bash` emits a script; `stardust man` emits roff; `stardust query foo --output json | cat` shows raw JSON with no escapes; `NO_COLOR=1 stardust --help` shows plain help.

</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

Restyling glamour or plain markdown data output, the TUI, the glamour renderer, the serve/MCP surfaces, a light-mode shell theme, and any change to the agent JSON shapes. The informative `Hint` error work is a sibling spec (`docs/specs/2026-06-25-2319-informative-cli-errors.md`).

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. Add `charm.land/fang/v2` to `go.mod` (`go get charm.land/fang/v2`), build.
2. Promote the cosmic palette to an exported leaf `internal/ui` package (`lipgloss.Color` tokens Primary/Secondary/Accent/Text/Muted/Border/Bg/CodeBg); point `internal/tui/styles.go` and `internal/render/glamour.go` at it to kill the duplication.
3. Write `internal/cli/color_scheme.go`: `cosmicColorScheme` mapping every `fang.ColorScheme` field from `internal/ui` tokens, plus a unit test asserting key fields equal the tokens.
4. Add `commit` var and a `versionOpt()` helper in `internal/cli/root.go`; wire the build ldflags for `version` and `commit`.
5. Swap `Execute()` to `fang.Execute(ctx, root, opts...)` with the colorscheme, version, commit, and signal options; remove the flat error line; add `context`/`signal`/`syscall` imports.
6. Write the headless-safety test: capture a data command `--output json` over an `os.Pipe`, assert zero `\x1b[` and valid JSON.
7. Run the full gate (build, test, vet, fmt, lint) and fix to green.

</details>

<details>
<summary><b>References</b></summary>
<br>

- `cmd/stardust/main.go` (thin entry point)
- `internal/cli/root.go` (Execute, command tree, manual version)
- `internal/cli/output.go` (`emitMarkdown`/`emitJSON`/`isTTY`, the data writers that must stay raw)
- `internal/tui/styles.go` and `internal/render/glamour.go` (the duplicated cosmic palette)
- charm-fang skill (authoritative option and ColorScheme reference)
- `charm.land/fang/v2` source (exact `ColorScheme` field set, `Execute` signature, `colorprofile` writer)
- The exo-jobs fang spec `docs/specs/2026-06-25-2220-fang-cli.md` (the proven template this adapts)
- ADR 0009 (Execute swap), ADR 0010 (headless boundary), ADR 0011 (cosmic colorscheme)

</details>
