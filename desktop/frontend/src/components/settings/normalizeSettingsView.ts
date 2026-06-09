import { asArray } from "../../lib/array";
import { normalizeLangPref } from "../../lib/i18n";
import { normalizeDesktopGit } from "../../lib/desktopGitPrefs";
import {
  normalizeThemePreference,
  normalizeThemeStyleForTheme,
} from "../../lib/theme";
import type { SettingsView } from "../../lib/types";

const AUTO_PLAN_MODES = ["off", "on"] as const;

type AutoPlanMode = (typeof AUTO_PLAN_MODES)[number];

function normalizeAutoPlan(mode: string | undefined): AutoPlanMode {
  return mode === "ask" || mode === "on" ? "on" : "off";
}

const TERMINAL_SHELL_PREFS = ["powershell", "cmd", "git-bash", "wsl"] as const;
export type TerminalShellPref = (typeof TERMINAL_SHELL_PREFS)[number];

export function normalizeTerminalShell(value: string | undefined): TerminalShellPref | "" {
  switch (value) {
    case "powershell":
    case "cmd":
    case "git-bash":
    case "wsl":
      return value;
    default:
      return "";
  }
}

type CloseBehavior = "background" | "quit";

function normalizeCloseBehavior(mode: string | undefined): CloseBehavior {
  return mode === "quit" ? "quit" : "background";
}

export function normalizeSettingsView(view: SettingsView | null | undefined): SettingsView | null {
  if (!view) return null;
  const permissions = view.permissions ?? { mode: "ask", allow: [], ask: [], deny: [] };
  const sandbox = view.sandbox ?? { bash: "enforce", network: false, workspaceRoot: "", allowWrite: [] };
  const network = view.network ?? {
    proxyMode: "auto",
    proxyUrl: "",
    noProxy: "",
    proxy: { type: "socks5", server: "", port: 0, username: "", password: "" },
  };
  const rawAgent = view.agent;
  const agent = {
    temperature: rawAgent?.temperature ?? 0,
    maxSteps: rawAgent?.maxSteps ?? 0,
    systemPrompt: rawAgent?.systemPrompt ?? "",
    systemPromptFile: rawAgent?.systemPromptFile ?? "",
    outputStyle: rawAgent?.outputStyle ?? "",
    autoPlan: normalizeAutoPlan(rawAgent?.autoPlan ?? view.autoPlan),
    autoPlanClassifier: rawAgent?.autoPlanClassifier ?? "",
    softCompactRatio: rawAgent?.softCompactRatio ?? 0.5,
    compactRatio: rawAgent?.compactRatio ?? 0.8,
    compactForceRatio: rawAgent?.compactForceRatio ?? 0.9,
    subagentModel: rawAgent?.subagentModel ?? "",
    subagentModels: { ...(rawAgent?.subagentModels ?? {}) },
    usesDefaultPrompt: rawAgent?.usesDefaultPrompt ?? false,
    defaultSystemPrompt: rawAgent?.defaultSystemPrompt ?? "",
  };
  return {
    ...view,
    providers: asArray(view.providers).map((p) => ({ ...p, models: asArray(p.models) })),
    providerKinds: asArray(view.providerKinds),
    permissions: {
      ...permissions,
      allow: asArray(permissions.allow),
      ask: asArray(permissions.ask),
      deny: asArray(permissions.deny),
    },
    sandbox: {
      ...sandbox,
      allowWrite: asArray(sandbox.allowWrite),
    },
    network: {
      ...network,
      proxy: network.proxy ?? { type: "socks5", server: "", port: 0, username: "", password: "" },
    },
    agent,
    autoPlan: agent.autoPlan,
    desktopLanguage: normalizeLangPref(view.desktopLanguage),
    desktopTheme: normalizeThemePreference(view.desktopTheme),
    desktopThemeStyle: normalizeThemeStyleForTheme(view.desktopThemeStyle, normalizeThemePreference(view.desktopTheme)),
    desktopTerminalShell: normalizeTerminalShell(view.desktopTerminalShell),
    desktopGit: normalizeDesktopGit(view.desktopGit),
    desktopAppearance: view.desktopAppearance ?? {
      backgroundPreset: "",
      foregroundPreset: "",
      textSize: "default",
      codeFontSize: "default",
      diffMarker: "background",
    },
    desktopCodeReview: view.desktopCodeReview ?? { defaultScope: "all", securityByDefault: false },
    closeBehavior: normalizeCloseBehavior(view.closeBehavior),
  };
}
