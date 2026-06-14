import type { Theme, ThemeStyle } from "./theme";
import { isThemeStyle } from "./theme";

/** Pure routing result for desktop-native send interception. Side effects run in the hook/App boundary. */
export type DesktopSendRoute =
  | { action: "shellUsage" }
  | { action: "shell"; cmd: string }
  | { action: "switchModel"; model: string }
  | { action: "openMemory" }
  | { action: "openKnowledge" }
  | { action: "setGoal"; label: string }
  | { action: "sideChat"; text: string }
  | { action: "reviewOpen" }
  | { action: "reviewRun" }
  | { action: "openSdd" }
  | { action: "openPreview"; url?: string }
  | { action: "themeShowCurrent" }
  | { action: "themeSet"; theme: Theme }
  | { action: "themeStyleSet"; style: ThemeStyle }
  | { action: "themeUnknown"; name: string }
  | { action: "send"; displayText: string; submitText: string };

function isThemeMode(value: string): value is Theme {
  return value === "auto" || value === "light" || value === "dark";
}

/** Intercept vs passthrough — deterministic, no I/O. Mirrors App handleSend guards. */
export function routeDesktopSend(displayText: string, submitText = displayText): DesktopSendRoute {
  const trimmed = displayText.trim();
  if (trimmed.startsWith("!")) {
    const cmd = trimmed.slice(1).trim();
    if (!cmd) return { action: "shellUsage" };
    return { action: "shell", cmd };
  }
  const model = /^\/model\s+(\S+)$/.exec(trimmed);
  if (model) return { action: "switchModel", model: model[1] };
  if (trimmed === "/memory") return { action: "openMemory" };
  if (trimmed === "/knowledge") return { action: "openKnowledge" };
  const goal = /^\/goal\s+(.+)$/.exec(trimmed);
  if (goal) return { action: "setGoal", label: goal[1].trim() };
  const btw = /^\/btw\s+(.+)$/.exec(trimmed);
  if (btw) return { action: "sideChat", text: btw[1].trim() };
  if (trimmed === "/review" || trimmed === "/review run") {
    return trimmed === "/review run" ? { action: "reviewRun" } : { action: "reviewOpen" };
  }
  if (trimmed === "/sdd") return { action: "openSdd" };
  const preview = /^\/preview(?:\s+(\S+))?$/.exec(trimmed);
  if (preview) return { action: "openPreview", url: preview[1] };
  const theme = /^\/theme(?:\s+(\S+))?$/.exec(trimmed);
  if (theme) {
    const arg = theme[1]?.toLowerCase();
    if (!arg) return { action: "themeShowCurrent" };
    if (isThemeMode(arg)) return { action: "themeSet", theme: arg };
    if (isThemeStyle(arg)) return { action: "themeStyleSet", style: arg };
    return { action: "themeUnknown", name: arg };
  }
  return { action: "send", displayText: trimmed, submitText: submitText.trim() };
}
