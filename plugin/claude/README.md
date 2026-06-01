# Stardust - Claude Code plugin

Exposes your local markdown vault to Claude Code as an MCP server, with four tools:

- `query` - hybrid keyword + semantic search over the vault
- `get_note` - read a note by path
- `status` - index health
- `graph` - link graph (orphans, broken links)

## Prerequisites

The `stardust` binary must be on your `PATH`:

```sh
go install github.com/alxxpersonal/stardust/cmd/stardust@latest
# then, in your vault:
stardust init && stardust index
```

The MCP server resolves the vault from the working directory, or from `STARDUST_VAULT` if set. To pin a specific vault regardless of where Claude Code launches, add an env to `.mcp.json`:

```json
{ "mcpServers": { "stardust": { "command": "stardust", "args": ["serve", "--mcp"], "env": { "STARDUST_VAULT": "/path/to/vault" } } } }
```

## Install (local development)

```sh
claude plugin marketplace add ./plugin/claude
claude plugin install stardust@stardust-local
```

Then in Claude Code, the `stardust` tools are available. Logging goes to stderr; stdout is reserved for the JSON-RPC protocol.
