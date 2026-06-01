import { App, ItemView, Plugin, PluginSettingTab, Setting, WorkspaceLeaf } from "obsidian";
import { StardustClient, Hit } from "../../../sdk/ts/stardust";

interface StardustSettings {
  daemonUrl: string;
}

const DEFAULT_SETTINGS: StardustSettings = { daemonUrl: "http://127.0.0.1:7777" };

const VIEW_TYPE = "stardust-search";

export default class StardustPlugin extends Plugin {
  settings: StardustSettings = DEFAULT_SETTINGS;
  client: StardustClient = new StardustClient(DEFAULT_SETTINGS.daemonUrl);

  async onload(): Promise<void> {
    await this.loadSettings();
    this.client = new StardustClient(this.settings.daemonUrl);

    this.registerView(VIEW_TYPE, (leaf) => new StardustSearchView(leaf, this));

    this.addRibbonIcon("search", "Stardust search", () => void this.activateView());
    this.addCommand({
      id: "stardust-search",
      name: "Search vault",
      callback: () => void this.activateView(),
    });

    this.addSettingTab(new StardustSettingTab(this.app, this));
  }

  onunload(): void {
    this.app.workspace.detachLeavesOfType(VIEW_TYPE);
  }

  async activateView(): Promise<void> {
    const { workspace } = this.app;
    let leaf: WorkspaceLeaf | null = workspace.getLeavesOfType(VIEW_TYPE)[0] ?? null;
    if (!leaf) {
      leaf = workspace.getRightLeaf(false);
      if (leaf) {
        await leaf.setViewState({ type: VIEW_TYPE, active: true });
      }
    }
    if (leaf) {
      workspace.revealLeaf(leaf);
    }
  }

  async loadSettings(): Promise<void> {
    this.settings = Object.assign({}, DEFAULT_SETTINGS, await this.loadData());
  }

  async saveSettings(): Promise<void> {
    await this.saveData(this.settings);
    this.client = new StardustClient(this.settings.daemonUrl);
  }
}

class StardustSearchView extends ItemView {
  private readonly plugin: StardustPlugin;

  constructor(leaf: WorkspaceLeaf, plugin: StardustPlugin) {
    super(leaf);
    this.plugin = plugin;
  }

  getViewType(): string {
    return VIEW_TYPE;
  }

  getDisplayText(): string {
    return "Stardust";
  }

  getIcon(): string {
    return "search";
  }

  async onOpen(): Promise<void> {
    const root = this.contentEl;
    root.empty();
    root.createEl("h4", { text: "Stardust" });

    const input = root.createEl("input", { type: "text", placeholder: "search the vault..." });
    input.style.width = "100%";
    const results = root.createEl("div");

    const run = async (): Promise<void> => {
      const q = input.value.trim();
      results.empty();
      if (!q) {
        return;
      }
      results.createEl("div", { text: "searching..." });
      try {
        const res = await this.plugin.client.query(q, 15);
        results.empty();
        results.createEl("div", { text: `${res.hits.length} results (${res.mode})` }).style.opacity = "0.6";
        for (const h of res.hits) {
          this.renderHit(results, h);
        }
      } catch (e) {
        results.empty();
        results.createEl("div", { text: "error: " + (e as Error).message });
      }
    };

    input.addEventListener("keydown", (ev: KeyboardEvent) => {
      if (ev.key === "Enter") {
        void run();
      }
    });
  }

  private renderHit(container: HTMLElement, h: Hit): void {
    const item = container.createEl("div");
    item.style.padding = "6px 0";
    item.style.cursor = "pointer";
    item.style.borderTop = "1px solid var(--background-modifier-border)";

    item.createEl("div", { text: h.title || h.path }).style.fontWeight = "600";
    item.createEl("div", { text: h.path }).style.opacity = "0.6";
    if (h.snippet) {
      item.createEl("div", { text: h.snippet });
    }

    item.addEventListener("click", () => {
      void this.app.workspace.openLinkText(h.path, "", false);
    });
  }

  async onClose(): Promise<void> {
    this.contentEl.empty();
  }
}

class StardustSettingTab extends PluginSettingTab {
  private readonly plugin: StardustPlugin;

  constructor(app: App, plugin: StardustPlugin) {
    super(app, plugin);
    this.plugin = plugin;
  }

  display(): void {
    const { containerEl } = this;
    containerEl.empty();
    new Setting(containerEl)
      .setName("Daemon URL")
      .setDesc("The Stardust HTTP API address (run `stardust serve`).")
      .addText((t) =>
        t.setValue(this.plugin.settings.daemonUrl).onChange(async (v) => {
          this.plugin.settings.daemonUrl = v.trim() || DEFAULT_SETTINGS.daemonUrl;
          await this.plugin.saveSettings();
        }),
      );
  }
}
