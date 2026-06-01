# Stardust - Obsidian plugin

Surfaces Stardust's hybrid search inside Obsidian. A thin client over the local Stardust HTTP API (`stardust serve`) - all logic lives in the daemon, the plugin is just UI.

## Prerequisites

Run the daemon against your vault:

```sh
stardust serve --addr 127.0.0.1:7777
```

## Build

```sh
npm install
npm run build   # typechecks, then bundles src/main.ts -> main.js
```

Copy `manifest.json` and `main.js` into `<vault>/.obsidian/plugins/stardust/`, then enable the plugin in Obsidian.

## Use

- Ribbon search icon or the "Stardust: Search vault" command opens a side panel.
- Type a query, press Enter; results are ranked notes with snippets. Click one to open it.
- Set the daemon URL in the plugin settings (defaults to `http://127.0.0.1:7777`).
