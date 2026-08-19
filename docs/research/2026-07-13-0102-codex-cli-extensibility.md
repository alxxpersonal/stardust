---
title: Codex CLI extensibility for a Stardust plugin
status: Active
version: 1
date: 2026-07-13
related:
  - docs/specs/2026-07-13-0102-codex-plugin.md
  - plugin/claude/
---

# Codex CLI extensibility for a Stardust plugin

Codex CLI (verified against the locally installed `codex-cli 0.144.1`) exposes first-class equivalents for every Claude Code extension point Stardust already uses, so a Codex plugin at near-full parity is buildable. The one headline gap is slash commands: Codex removed user-defined custom prompts and pivoted to Agent Skills, so the 10 `commands/*.md` become skills with no positional-argument templating.

## Method and provenance

Two evidence tiers. Primary is direct inspection of the installed `codex-cli 0.144.1` on this machine: `codex --help` and subcommand help, `codex features list`, `codex mcp list`, `codex plugin list`, `codex plugin marketplace list`, the on-disk manifests of installed plugins (`~/.codex/.tmp/**`), and the live `~/.codex/config.toml`. Secondary is a prior web-research pass over the `openai/codex` repo and `learn.chatgpt.com/docs/*`. Where the two disagree, the installed binary wins and is marked CONFIRMED. Web-only claims are marked WEB and were not all re-verified against the binary.

## Findings

### Plugin and marketplace system (CONFIRMED)

- `codex plugin` has `add`, `list`, `marketplace`, `remove`. `codex plugin marketplace` has `add`, `list`, `upgrade`, `remove`.
- `codex plugin marketplace add <SOURCE>` accepts a local path, `owner/repo[@ref]`, or an HTTPS/SSH Git URL (`--ref`, `--sparse`, `--json`). Example from help: `codex plugin marketplace add ./path/to/marketplace`.
- `codex plugin add <PLUGIN>@<MARKETPLACE>` or `codex plugin add <PLUGIN> --marketplace <M>`.
- State lives in `~/.codex/config.toml`: `[marketplaces.<name>] source = "..."` and `[plugins."<plugin>@<marketplace>"] enabled = true`.
- Marketplace descriptor is discovered at `.agents/plugins/marketplace.json` (native, used by `openai-bundled`) or the legacy `.claude-plugin/marketplace.json` (used by `claude-code-warp`, and by alxx's own `exo-discord` local plugin). Both are accepted.
- A Claude-format plugin installs and enables in Codex today: `warp@claude-code-warp` is `installed, enabled` from a `.claude-plugin/marketplace.json` source. This is the cross-compat proof.
- Feature flags (`codex features list`): `plugins`, `plugin_sharing`, `remote_plugin`, `hooks`, `multi_agent`, `skill_mcp_dependency_install` are all `stable` and effective `true`. `plugin_hooks` shows `removed` (see Hooks: plugin hooks still load; the flag graduated, the capability did not vanish).

### Plugin manifest `.codex-plugin/plugin.json` (CONFIRMED)

From the `visualize` and `computer-use` bundled plugins. Identity fields match Claude's (`name`, `version`, `description`, `author {name}`, `homepage`, `license`, `keywords`). Component pointers:

- `"mcpServers": "./.mcp.json"` - relative path to a standard MCP config file.
- `"skills": "./skills/"` - relative path to a skills directory.
- Hooks are contributed by a `hooks/hooks.json` in the bundle (see Hooks).
- `"interface": { ... }` - Codex-specific presentation block: `displayName`, `shortDescription`, `longDescription`, `developerName`, `category`, `capabilities[]`, `composerIcon`, `logo`, `logoDark`, `defaultPrompt[]`, `brandColor`, `screenshots[]`, plus URL fields. Optional for function; drives the `/plugins` browser UI.

Legacy `.claude-plugin/plugin.json` is also accepted (the `warp` plugin uses it with only identity fields).

### MCP client (CONFIRMED)

- `.mcp.json` is byte-identical to Claude's: `{"mcpServers": {"<id>": {"command", "args", "cwd"?, "env"?, "url"?, "enabled"?}}}`. Confirmed from `computer-use/.mcp.json`.
- config.toml form: `[mcp_servers.<id>]` with `command`/`args`/`env`/`cwd`/`enabled` for stdio, or `url` (+ optional bearer-token env var, OAuth) for streamable HTTP.
- `codex mcp add <NAME> -- <COMMAND>...` writes the stdio block; `--env KEY=VALUE`, `--url`, `--bearer-token-env-var`, `--oauth-client-id`, `--oauth-resource` are supported. `codex mcp list/get/remove/login/logout` exist.
- `${CLAUDE_PLUGIN_ROOT}` is expanded inside MCP command/args (the installed `discord` and `telegram` plugins run `bun run --cwd ${CLAUDE_PLUGIN_ROOT} ...`).
- Tool names are prefixed by the server id (so `stardust` -> `stardust.bundle`, etc; exact separator is Codex-internal).
- Codex can also run AS an MCP server: `codex mcp-server`.

### Hooks (CONFIRMED, corrects the web research)

- Plugin `hooks/hooks.json` uses the same shape as Claude: `{"hooks": {"<Event>": [{"matcher"?: "...", "hooks": [{"type": "command", "command": "...", "timeout"?: N}]}]}}`. Confirmed from the installed `hookify` and `ralph-loop` plugins.
- Plugin-contributed hooks DO load. `~/.codex/config.toml` has a `[hooks.state]` table with live entries such as `"warp@claude-code-warp:hooks/hooks.json:session_start:0:0"`, `"warp@claude-code-warp:hooks/hooks.json:user_prompt_submit:0:0"`, and `"vercel@claude-plugins-official:hooks/hooks.json:session_start:0:0"`. This empirically refutes the earlier web-sourced worry that plugin hooks were gated off or removed.
- Event names normalize from Claude PascalCase to Codex snake_case: `SessionStart` -> `session_start`, `UserPromptSubmit` -> `user_prompt_submit`, `Stop` -> `stop`, `PostToolUse` -> `post_tool_use`, plus `permission_request`. A Claude-cased `hooks.json` is understood.
- `hooks` is a `stable`, default-on feature.
- Trust: each plugin hook gets a per-hook entry in `[hooks.state]` keyed by an identity that includes a content position; new or edited command hooks are subject to a `/hooks` trust review (WEB, consistent with the observed state table). Managed/enterprise configs can pre-trust.
- Hook stdout is injected into the model context as developer context; the `${CLAUDE_PLUGIN_ROOT}` and `${CLAUDE_PLUGIN_DATA}` env vars are exported to the hook process, so existing POSIX hook scripts run unmodified.

### Skills (CONFIRMED)

- `SKILL.md` is YAML frontmatter with `name` and `description`, then a markdown body, loaded with progressive disclosure. Confirmed from the official `visualize` skill.
- A plugin ships skills via the manifest `"skills": "./skills/"` pointer; each skill is `skills/<name>/SKILL.md`.
- User-global skills live at `~/.agents/skills/<name>/SKILL.md` (107 present on this machine).
- Per-skill enable/disable via config.toml `[[skills.config]]` array-of-tables (`path`, `enabled`).
- Custom prompts (`~/.codex/prompts`, `/prompts:<name>`) were REMOVED (WEB: `openai/codex` PR #16115). There is no `$ARGUMENTS` / `$1-$9` positional templating and no namespaced `/stardust:spec` invocation. Skills trigger by `description` match or an explicit skill invocation; arguments are natural language.

### Subagents (WEB, partially confirmed)

- `multi_agent` is a `stable`, default-on feature. Codex supports custom agents defined in TOML (`.codex/agents/*.toml` or `~/.codex/agents/*.toml`) with `name`, `description`, `developer_instructions`, and optional model/effort/sandbox/mcp fields. Not exercised on disk in this pass; treat the exact schema as WEB until a `.toml` agent is created and loaded.

### Shared instructions (WEB)

- `AGENTS.md` is concatenated global-then-repo-root-down to cwd, under a shared `project_doc_max_bytes` cap (~32 KiB across all `AGENTS.md`). Stardust's `internal/agentsync` already composes a sentinel-delimited managed block into `AGENTS.md`.

## Implications for the build

1. Stardust's existing `.mcp.json` and `hooks/hooks.json` are already Codex-compatible with zero edits; the hook scripts (`session-start.sh`, `prompt-submit.sh`, `resolve-root.sh`) run unmodified because Codex exports `${CLAUDE_PLUGIN_ROOT}`.
2. The command surface is the only real port: 10 `commands/*.md` -> `skills/stardust-<name>/SKILL.md`, dropping Claude-only frontmatter (`allowed-tools`, `argument-hint`) and softening Claude-harness references (ExitPlanMode, TodoWrite, native plan mode) to Codex-neutral phrasing.
3. `policy.txt` works but opens with a Claude framing (`<user-instructions priority="override">`); on Codex, hook stdout is plain developer context, so the "override" weight does not carry. The deterministic steer survives; drop or neutralize the Claude-only wrapper for the Codex bundle.
4. A native `.codex-plugin/plugin.json` with an `interface` block plus `mcpServers`/`skills`/`hooks` pointers gives the best `/plugins` UX; a legacy `.claude-plugin` layout would also install.
5. Install is real and one-command per step: `codex plugin marketplace add <path>` then `codex plugin add stardust@<marketplace>`, or the MCP-only path `codex mcp add stardust -- stardust serve --mcp`.

## Open items to verify during the build

- Exact marketplace descriptor path Codex prefers for a single local plugin (`.agents/plugins/marketplace.json` vs `.claude-plugin/marketplace.json`) and whether a `.codex-plugin/plugin.json` manifest pairs with a `.claude-plugin/marketplace.json` source. Resolve by installing into the local Codex.
- Whether plugin-bundled hooks auto-trust on install or require a `/hooks` approval.
- Whether the `SessionStart` hook stdout lands as context in a non-interactive `codex exec` run or only in the interactive TUI.
- The Codex subagent TOML schema, if subagents are shipped.

## Sources

- Primary: installed `codex-cli 0.144.1` (`codex --help`, `codex features list`, `codex mcp list`, `codex plugin list`, `codex plugin marketplace list`), on-disk plugin manifests under `~/.codex/.tmp/**`, and `~/.codex/config.toml`.
- Secondary: `github.com/openai/codex` (repo docs, `mcp_cmd.rs`, `manifest.rs`), `learn.chatgpt.com/docs/{extend/mcp,hooks,build-skills,build-plugins,plugins}`, `openai/codex` PR #16115 (custom-prompts removal).
