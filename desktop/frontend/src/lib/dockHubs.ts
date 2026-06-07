import type { RightDockTab } from "../components/Topbar";

export type DockHub = "context" | "work" | "preview";
export type PreviewMode = "browser" | "terminal";

export interface PreviewPanelState {
  terminal: boolean;
  browser: boolean;
}

export interface DockHubDef {
  id: DockHub;
  defaultTab: RightDockTab;
  tabs: RightDockTab[];
  previewModes?: PreviewMode[];
}

export const DOCK_HUBS: DockHubDef[] = [
  { id: "context", defaultTab: "context", tabs: ["context"] },
  { id: "work", defaultTab: "changes", tabs: ["changes", "files", "todo", "git"] },
  { id: "preview", defaultTab: "browser", tabs: ["browser"], previewModes: ["browser", "terminal"] },
];

const HUB_BY_TAB = new Map<RightDockTab, DockHub>(
  DOCK_HUBS.flatMap((hub) => hub.tabs.map((tab) => [tab, hub.id] as const)),
);

const HUB_MAP = new Map(DOCK_HUBS.map((hub) => [hub.id, hub]));

const LAST_TAB_STORAGE_KEY = "reasonix.dock.hubLastTab.v1";
const PREVIEW_PANEL_STORAGE_KEY = "reasonix.dock.previewPanel.v1";
const PREVIEW_MODE_STORAGE_KEY = "reasonix.dock.previewMode.v1";

export function dockHubForTab(tab: RightDockTab): DockHub {
  return HUB_BY_TAB.get(tab) ?? "context";
}

export function dockHubDef(hub: DockHub): DockHubDef {
  return HUB_MAP.get(hub) ?? DOCK_HUBS[0];
}

export function dockTabInHub(tab: RightDockTab, hub: DockHub): boolean {
  return dockHubDef(hub).tabs.includes(tab);
}

export function previewModesForHub(hub: DockHub): PreviewMode[] {
  return dockHubDef(hub).previewModes ?? [];
}

export function loadHubLastTab(hub: DockHub): RightDockTab {
  if (hub === "preview") return "browser";
  if (typeof window === "undefined") return dockHubDef(hub).defaultTab;
  try {
    const raw = window.localStorage.getItem(LAST_TAB_STORAGE_KEY);
    if (!raw) return dockHubDef(hub).defaultTab;
    const parsed = JSON.parse(raw) as Partial<Record<DockHub, RightDockTab>>;
    const saved = parsed?.[hub] as RightDockTab | "plan" | undefined;
    if (saved === "plan") return "git";
    if (saved && dockTabInHub(saved, hub)) return saved;
    if (hub === "work" && parsed?.preview === "files") return "files";
  } catch {
    /* ignore */
  }
  return dockHubDef(hub).defaultTab;
}

export function saveHubLastTab(hub: DockHub, tab: RightDockTab): void {
  if (hub === "preview" || !dockTabInHub(tab, hub) || typeof window === "undefined") return;
  try {
    const raw = window.localStorage.getItem(LAST_TAB_STORAGE_KEY);
    const parsed = raw ? (JSON.parse(raw) as Partial<Record<DockHub, RightDockTab>>) : {};
    window.localStorage.setItem(LAST_TAB_STORAGE_KEY, JSON.stringify({ ...parsed, [hub]: tab }));
  } catch {
    /* ignore */
  }
}

export function loadPreviewPanelState(): PreviewPanelState {
  if (typeof window === "undefined") return { terminal: false, browser: true };
  try {
    const raw = window.localStorage.getItem(PREVIEW_PANEL_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<PreviewPanelState>;
      return {
        terminal: parsed.terminal === true,
        browser: parsed.browser !== false,
      };
    }
    const legacy = window.localStorage.getItem(PREVIEW_MODE_STORAGE_KEY);
    if (legacy === "terminal") return { terminal: true, browser: false };
    if (legacy === "browser") return { terminal: false, browser: true };
  } catch {
    /* ignore */
  }
  return { terminal: false, browser: true };
}

export function savePreviewPanelState(state: PreviewPanelState): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(PREVIEW_PANEL_STORAGE_KEY, JSON.stringify(state));
    window.localStorage.setItem(
      PREVIEW_MODE_STORAGE_KEY,
      state.terminal && !state.browser ? "terminal" : state.browser ? "browser" : "browser",
    );
  } catch {
    /* ignore */
  }
}

export function resolveHubTab(hub: DockHub, tab?: RightDockTab): RightDockTab {
  if (hub === "preview") return "browser";
  if (tab && dockTabInHub(tab, hub)) return tab;
  return loadHubLastTab(hub);
}

export function saveDockTabSelection(tab: RightDockTab): void {
  const hub = dockHubForTab(tab);
  if (hub === "preview") return;
  saveHubLastTab(hub, tab);
}

export function getPreviewPanelState(
  terminalOpen: boolean,
  dockOpen: boolean,
  activeTab: RightDockTab | null | undefined,
): PreviewPanelState {
  return {
    terminal: terminalOpen,
    browser: dockOpen && activeTab === "browser",
  };
}

export function isPreviewHubActive(
  terminalOpen: boolean,
  dockOpen: boolean,
  activeTab: RightDockTab | null | undefined,
): boolean {
  const state = getPreviewPanelState(terminalOpen, dockOpen, activeTab);
  return state.terminal || state.browser;
}

/** @deprecated use getPreviewPanelState */
export function getActivePreviewMode(
  terminalOpen: boolean,
  dockOpen: boolean,
  activeTab: RightDockTab | null | undefined,
): PreviewMode | null {
  const state = getPreviewPanelState(terminalOpen, dockOpen, activeTab);
  if (state.terminal) return "terminal";
  if (state.browser) return "browser";
  return null;
}

/** @deprecated use loadPreviewPanelState */
export function loadPreviewMode(): PreviewMode {
  const state = loadPreviewPanelState();
  if (state.terminal && !state.browser) return "terminal";
  return "browser";
}

/** @deprecated use savePreviewPanelState */
export function savePreviewMode(mode: PreviewMode): void {
  savePreviewPanelState({
    terminal: mode === "terminal",
    browser: mode === "browser",
  });
}
