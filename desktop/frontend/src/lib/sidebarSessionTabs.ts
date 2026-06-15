import type { BrowserTab } from "./useBrowserPanel";
import type { TerminalTab } from "../components/TerminalPanel";

export type SidebarSessionKind = "browser" | "terminal";

export interface SidebarSessionTab {
  kind: SidebarSessionKind;
  id: string;
  clientKey: string;
  title: string;
}

export function buildSidebarSessionTabs(
  browserTabs: BrowserTab[],
  terminalTabs: TerminalTab[],
): SidebarSessionTab[] {
  const browser = browserTabs.map((tab) => ({
    kind: "browser" as const,
    id: tab.id,
    clientKey: tab.clientKey,
    title: tab.title,
  }));
  const terminal = terminalTabs.map((tab) => ({
    kind: "terminal" as const,
    id: tab.id,
    clientKey: tab.clientKey,
    title: tab.title,
  }));
  return [...browser, ...terminal];
}

export function resolveActiveSessionId(
  sidebarBodyTab: SidebarSessionKind | null,
  activeBrowserTabId: string | null,
  activeTerminalId: string | null,
): string | null {
  if (sidebarBodyTab === "browser") return activeBrowserTabId;
  if (sidebarBodyTab === "terminal") return activeTerminalId;
  return null;
}
