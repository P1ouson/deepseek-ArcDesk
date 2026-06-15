import type { PreviewMode } from "./dockHubs";
import type { RightDockTab } from "../components/Topbar";

/** Primary tabs shown in the unified sidebar icon bar. */
export type SidebarPrimaryTab = "changes" | "browser" | "terminal" | "files" | "git" | "context" | "todo";

export type SidebarProfile = "code" | "write";

const CODE_PANEL_TABS: SidebarPrimaryTab[] = ["changes", "files", "git", "context", "todo"];
const WRITE_PANEL_TABS: SidebarPrimaryTab[] = ["context", "todo"];

export function sidebarPanelTabsForProfile(profile: SidebarProfile): SidebarPrimaryTab[] {
  return profile === "write" ? WRITE_PANEL_TABS : CODE_PANEL_TABS;
}

export function sidebarAddActionsForProfile(profile: SidebarProfile): Array<"terminal" | "browser"> {
  return profile === "write" ? ["browser"] : ["terminal", "browser"];
}

export function isSidebarPanelTabAllowed(tab: SidebarPrimaryTab, profile: SidebarProfile): boolean {
  return sidebarPanelTabsForProfile(profile).includes(tab);
}

export function defaultSidebarPanelTab(profile: SidebarProfile): SidebarPrimaryTab {
  return profile === "write" ? "context" : "changes";
}

export function resolveSidebarPrimaryTab(
  rightDockMode: RightDockTab,
  previewColumnOpen: boolean,
  previewMode: PreviewMode,
): SidebarPrimaryTab {
  if (previewColumnOpen) {
    if (previewMode === "browser") return "browser";
    if (previewMode === "terminal") return "terminal";
    if (previewMode === "file" || previewMode === "page") return "files";
  }
  if (rightDockMode === "git") return "git";
  if (rightDockMode === "context") return "context";
  if (rightDockMode === "todo") return "todo";
  if (rightDockMode === "changes") return "changes";
  return "files";
}

export function sidebarShowsBranchBar(tab: SidebarPrimaryTab): boolean {
  return tab === "changes" || tab === "git";
}
