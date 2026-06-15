import { useCallback } from "react";
import type { RightDockTab } from "../components/Topbar";
import type { ReviewMode, ReviewScope } from "./codeReview";
import { app } from "./bridge";
import { routeDesktopSend, type DesktopSendRoute } from "./desktopSendRouter";
import type { AppMode } from "./appMode";
import type { Translator } from "./i18n";
import type { Mode } from "./types";
import { applyThemeFromSettings } from "./applyThemeFromSettings";
import { getTheme, getThemeStyle, normalizeThemeStyleForTheme, themeForStyle } from "./theme";
import { toErrorMessage } from "./errors";
import { applyWriteModeSkill } from "./writeSkill";
import { enrichWriteModeSubmit } from "./writeAgentContext";

export type DesktopSendRouterDeps = {
  appMode: AppMode;
  mode: Mode;
  filePreviewComposerOpen: boolean;
  t: Translator;
  notice: (text: string, level?: "info" | "warn") => void;
  runShell: (command: string) => void;
  switchModel: (name: string) => void | Promise<void>;
  openMemory: () => void | Promise<void>;
  openKnowledge: () => void | Promise<void>;
  setGoalLabel: (label: string) => void;
  dispatchSideChat: (text: string) => void | Promise<void>;
  setAppMode: (mode: AppMode) => void;
  openDockTab: (tab: RightDockTab, options?: { toggle?: boolean }) => void;
  openWebPreview: (url?: string) => void;
  runCodeReview: (reviewMode: ReviewMode, scope: ReviewScope, paths: string[]) => void | Promise<void>;
  setSddOpen: (open: boolean) => void;
  syncModeToController: (mode: Mode) => void | Promise<void>;
  send: (displayText: string, submitText?: string) => void;
  enterPlanMode: (options?: { prefill?: boolean }) => void;
  exitExpandedPreviewComposer: () => void;
  writeSelectedFile?: string;
  writeWorkspaceRoot?: string;
};

async function executeDesktopSendRoute(route: DesktopSendRoute, deps: DesktopSendRouterDeps): Promise<void> {
  switch (route.action) {
    case "shellUsage":
      deps.notice("usage: !<command>  (e.g. !ls -la)");
      return;
    case "shell":
      deps.runShell(route.cmd);
      return;
    case "switchModel":
      void deps.switchModel(route.model);
      return;
    case "openMemory":
      void deps.openMemory();
      return;
    case "openKnowledge":
      void deps.openKnowledge();
      return;
    case "setGoal":
      deps.setGoalLabel(route.label);
      deps.notice(deps.t("goal.set", { label: route.label }));
      return;
    case "sideChat":
      if (route.text) void deps.dispatchSideChat(route.text);
      deps.notice(deps.t("sideChat.opened"));
      return;
    case "reviewOpen":
      deps.setAppMode("code");
      deps.openDockTab("changes", { toggle: false });
      deps.notice(deps.t("slash.reviewOpened"));
      return;
    case "reviewRun":
      deps.setAppMode("code");
      deps.openDockTab("changes", { toggle: false });
      void app
        .WorkspaceChanges()
        .then((view) => {
          const paths = view.files.map((file) => file.path);
          void deps.runCodeReview("standard", "all", paths);
        })
        .catch((err) => {
          deps.notice(deps.t("common.operationFailed", { msg: toErrorMessage(err) }), "warn");
        });
      return;
    case "openSdd":
      deps.setSddOpen(true);
      deps.notice(deps.t("slash.sddOpened"));
      return;
    case "openPreview":
      deps.setAppMode("code");
      deps.openWebPreview(route.url);
      deps.notice(deps.t("slash.previewOpened"));
      return;
    case "themeShowCurrent":
      deps.notice(deps.t("settings.themeCurrentSimple", { theme: getTheme() }));
      return;
    case "themeSet": {
      const nextStyle = normalizeThemeStyleForTheme(getThemeStyle(), route.theme);
      await app.SetDesktopAppearance(route.theme, nextStyle);
      applyThemeFromSettings(await app.Settings(), "slash");
      deps.notice(deps.t("settings.themeChangedSimple", { theme: route.theme }));
      return;
    }
    case "themeStyleSet": {
      const nextTheme = themeForStyle(route.style);
      await app.SetDesktopAppearance(nextTheme, route.style);
      applyThemeFromSettings(await app.Settings(), "slash");
      deps.notice(deps.t("settings.themeChangedSimple", { theme: route.style }));
      return;
    }
    case "themeUnknown":
      deps.notice(deps.t("settings.themeUnknown", { name: route.name }), "warn");
      return;
    case "planEnter":
      deps.setAppMode("code");
      deps.enterPlanMode();
      return;
    case "planSend":
      deps.setAppMode("code");
      deps.enterPlanMode({ prefill: false });
      await deps.syncModeToController("plan");
      deps.send(route.displayText, route.text);
      if (deps.filePreviewComposerOpen) deps.exitExpandedPreviewComposer();
      return;
    case "send":
      await deps.syncModeToController(deps.mode);
      {
        let displayText = route.displayText;
        let submitText = route.submitText;
        if (deps.appMode === "write") {
          const enriched = await enrichWriteModeSubmit(displayText, submitText, {
            writeFilePath: deps.writeSelectedFile,
            writeWorkspaceRoot: deps.writeWorkspaceRoot,
          });
          displayText = enriched.displayText;
          submitText = enriched.submitText;
          const outbound = applyWriteModeSkill(displayText, submitText);
          deps.send(outbound.displayText, outbound.submitText);
        } else {
          deps.send(displayText, submitText);
        }
      }
      if (deps.filePreviewComposerOpen) deps.exitExpandedPreviewComposer();
      return;
    default:
      return;
  }
}

export function useDesktopSendRouter(deps: DesktopSendRouterDeps) {
  return useCallback(
    async (displayText: string, submitText = displayText) => {
      const route = routeDesktopSend(displayText, submitText);
      await executeDesktopSendRoute(route, deps);
    },
    [deps],
  );
}
