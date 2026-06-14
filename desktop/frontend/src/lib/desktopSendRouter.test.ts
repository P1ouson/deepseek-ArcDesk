import { describe, expect, it } from "vitest";
import { routeDesktopSend } from "./desktopSendRouter";

describe("routeDesktopSend", () => {
  it("routes shell commands", () => {
    expect(routeDesktopSend("!ls -la")).toEqual({ action: "shell", cmd: "ls -la" });
    expect(routeDesktopSend("!")).toEqual({ action: "shellUsage" });
    expect(routeDesktopSend("!   ")).toEqual({ action: "shellUsage" });
  });

  it("routes /model", () => {
    expect(routeDesktopSend("/model deepseek-chat")).toEqual({ action: "switchModel", model: "deepseek-chat" });
  });

  it("routes /memory", () => {
    expect(routeDesktopSend("/memory")).toEqual({ action: "openMemory" });
  });

  it("routes /knowledge", () => {
    expect(routeDesktopSend("/knowledge")).toEqual({ action: "openKnowledge" });
  });

  it("routes /goal", () => {
    expect(routeDesktopSend("/goal ship v1.3")).toEqual({ action: "setGoal", label: "ship v1.3" });
  });

  it("routes /btw", () => {
    expect(routeDesktopSend("/btw ping")).toEqual({ action: "sideChat", text: "ping" });
    expect(routeDesktopSend("/btw")).toEqual({ action: "send", displayText: "/btw", submitText: "/btw" });
  });

  it("routes /review", () => {
    expect(routeDesktopSend("/review")).toEqual({ action: "reviewOpen" });
    expect(routeDesktopSend("/review run")).toEqual({ action: "reviewRun" });
  });

  it("routes /sdd", () => {
    expect(routeDesktopSend("/sdd")).toEqual({ action: "openSdd" });
  });

  it("routes /theme", () => {
    expect(routeDesktopSend("/theme")).toEqual({ action: "themeShowCurrent" });
    expect(routeDesktopSend("/theme dark")).toEqual({ action: "themeSet", theme: "dark" });
    expect(routeDesktopSend("/theme DARK")).toEqual({ action: "themeSet", theme: "dark" });
    expect(routeDesktopSend("/theme graphite")).toEqual({ action: "themeStyleSet", style: "graphite" });
    expect(routeDesktopSend("/theme neon")).toEqual({ action: "themeUnknown", name: "neon" });
  });

  it("passthrough send for plain text and backend slash", () => {
    expect(routeDesktopSend("hello")).toEqual({ action: "send", displayText: "hello", submitText: "hello" });
    expect(routeDesktopSend("  hello  ", " submit ")).toEqual({
      action: "send",
      displayText: "hello",
      submitText: "submit",
    });
    expect(routeDesktopSend("/plan")).toEqual({ action: "send", displayText: "/plan", submitText: "/plan" });
    expect(routeDesktopSend("/skill")).toEqual({ action: "send", displayText: "/skill", submitText: "/skill" });
    expect(routeDesktopSend("/mcp show github")).toEqual({
      action: "send",
      displayText: "/mcp show github",
      submitText: "/mcp show github",
    });
    expect(routeDesktopSend("/auto-plan on")).toEqual({ action: "send", displayText: "/auto-plan on", submitText: "/auto-plan on" });
    expect(routeDesktopSend("/language zh")).toEqual({ action: "send", displayText: "/language zh", submitText: "/language zh" });
    expect(routeDesktopSend("/hooks trust")).toEqual({ action: "send", displayText: "/hooks trust", submitText: "/hooks trust" });
  });
});
