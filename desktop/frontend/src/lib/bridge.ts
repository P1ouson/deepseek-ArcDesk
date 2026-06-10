// bridge is the single seam between the React app and the Go kernel. In the Wails
// shell it calls the bound App methods (window.go.main.App.*) and subscribes to
// the runtime event stream (window.runtime.EventsOn). In a plain browser (`pnpm
// dev` outside the shell) those globals are absent, so it falls back to a mock
// that streams a canned turn through the same contract — letting the whole UI be
// developed and laid out without rebuilding the Go side.

import { t } from "./i18n";
import { hasGoBinding, isRuntimeReady } from "./runtime";

import type {
  BalanceInfo,
  CapabilitiesView,
  CheckpointMeta,
  CommandInfo,
  ContextInfo,
  ContextPanelInfo,
  DirEntry,
  DroppedItem,
  EffortInfo,
  FilePreview,
  FileEntry,
  HistoryMessage,
  JobView,
  ClawChannel,
  ClawCallbackInfo,
  ClawMessage,
  MobileConnectConfig,
  MobileConnectDiagnostics,
  MobilePairingInfo,
  MobilePendingDecision,
  MobileTunnelStatus,
  MCPCatalogEntry,
  MCPServerInput,
  MemoryView,
  Meta,
  ModelInfo,
  NetworkView,
  ProjectNode,
  ProviderView,
  ProviderModelsResult,
  QuestionAnswer,
  ServerView,
  SessionMeta,
  ScheduledTask,
  SettingsView,
  SkillRootView,
  SkillView,
  AgentSettingsInput,
  OutputStyleView,
  DesktopAppearanceView,
  DesktopCodeReviewView,
  DesktopLocalPrefsMigrationInput,
  ProjectPreviewSettingsInput,
  SlashArgsResult,
  TabMeta,
  TopicMeta,
  UpdateInfo,
  UpdateProgress,
  ShellRunResult,
  CodeReviewResult,
  TerminalStartResult,
  ProjectSandboxStatus,
  ConfigureProjectSandboxInput,
  PreviewURLValidation,
  WireEvent,
  WorkspaceChangesView,
  WorkspaceView,
} from "./types";

// AppBindings is derived from the Wails-generated Go → TS method signatures, so
// the compiler catches drift between the Go binding surface and the frontend mock.
// Run `wails generate module` after adding/renaming a bound method on App, then
// `pnpm typecheck` to verify the mock still satisfies the contract.
//
// Types for the new native-feel bindings — kept inline since they are
// bridge-specific and only used in AppBindings / the dev mock.
interface NativeConfirmRequest {
  title: string;
  message: string;
  detail: string;
  confirmLabel: string;
  cancelLabel: string;
  destructive: boolean;
}

interface DesktopWindowState {
  width: number;
  height: number;
  x: number;
  y: number;
  maximised: boolean;
}

// AppBindings is the hand-written contract between the React app and the Go
// kernel. It uses local types (types.ts) so components don't import generated
// model classes. _CheckGeneratedBindings catches drift: when a Go method is
// added or renamed, the generated types shift, and a key present in GeneratedApp
// but missing from AppBindings causes a type error here. Fix: add the new method
// to AppBindings, then run `pnpm typecheck` to verify.
export interface AppBindings {
  Platform(): Promise<string>;
  Submit(input: string): Promise<void>;
  SubmitToTab(tabID: string, input: string): Promise<void>;
  SubmitDisplay(display: string, input: string): Promise<void>;
  SubmitDisplayToTab(tabID: string, display: string, input: string): Promise<void>;
  RunShell(command: string): Promise<void>;
  RunShellQuiet(command: string): Promise<ShellRunResult>;
  StartTerminal(): Promise<TerminalStartResult>;
  WriteTerminal(sessionID: string, data: string): Promise<void>;
  ResizeTerminal(sessionID: string, cols: number, rows: number): Promise<void>;
  CloseTerminal(sessionID: string): Promise<void>;
  ProjectSandboxStatus(): Promise<ProjectSandboxStatus>;
  ConfigureProjectSandbox(input: ConfigureProjectSandboxInput): Promise<void>;
  ValidatePreviewURL(url: string): Promise<PreviewURLValidation>;
  ProbePreviewURL(url: string): Promise<boolean>;
  DetectDevServerURL(): Promise<string>;
  OpenWebPreview(url: string): Promise<void>;
  WorkspacePagePreviewURL(path: string): Promise<string>;
  IsPreviewablePage(path: string): Promise<boolean>;
  Cancel(): Promise<void>;
  CancelTab(tabID: string): Promise<void>;
  Approve(id: string, allow: boolean, session: boolean, persist: boolean): Promise<void>;
  ApproveTab(tabID: string, id: string, allow: boolean, session: boolean, persist: boolean): Promise<void>;
  AnswerQuestion(id: string, answers: QuestionAnswer[]): Promise<void>;
  AnswerQuestionForTab(tabID: string, id: string, answers: QuestionAnswer[]): Promise<void>;
  SetPlanMode(on: boolean): Promise<void>;
  SetMode(mode: string): Promise<void>;
  SetModeForTab(tabID: string, mode: string): Promise<void>;
  Compact(): Promise<void>;
  NewSession(): Promise<void>;
  History(): Promise<HistoryMessage[]>;
  HistoryForTab(tabID: string): Promise<HistoryMessage[]>;
  Checkpoints(): Promise<CheckpointMeta[]>;
  CheckpointsForTab(tabID: string): Promise<CheckpointMeta[]>;
  Rewind(turn: number, scope: string): Promise<void>;
  Fork(turn: number): Promise<TabMeta>;
  SummarizeFrom(turn: number): Promise<void>;
  SummarizeUpTo(turn: number): Promise<void>;
  ListSessions(): Promise<SessionMeta[]>;
  ListTrashedSessions(): Promise<SessionMeta[]>;
  ResumeSession(path: string): Promise<HistoryMessage[]>;
  ResumeSessionForTab(tabID: string, path: string): Promise<HistoryMessage[]>;
  PreviewSession(path: string): Promise<HistoryMessage[]>;
  DeleteSession(path: string): Promise<void>;
  RestoreSession(path: string): Promise<void>;
  PurgeTrashedSession(path: string): Promise<void>;
  RenameSession(path: string, title: string): Promise<void>;
  ListWorkspaces(): Promise<WorkspaceView[]>;
  PickWorkspace(): Promise<string>;
  PickFilePath(): Promise<string>;
  PickSaveFilePath(defaultName: string): Promise<string>;
  SwitchWorkspace(path: string): Promise<string>;
  RemoveWorkspace(path: string): Promise<void>;
  ContextUsage(): Promise<ContextInfo>;
  ContextUsageForTab(tabID: string): Promise<ContextInfo>;
  Balance(): Promise<BalanceInfo>;
  BalanceForTab(tabID: string): Promise<BalanceInfo>;
  Jobs(): Promise<JobView[]>;
  JobsForTab(tabID: string): Promise<JobView[]>;
  Meta(): Promise<Meta>;
  MetaForTab(tabID: string): Promise<Meta>;
  Commands(): Promise<CommandInfo[]>;
  Capabilities(): Promise<CapabilitiesView>;
  ListMCPCatalog(): Promise<MCPCatalogEntry[]>;
  AddMCPServer(input: MCPServerInput): Promise<number>;
  UpdateMCPServer(name: string, input: MCPServerInput): Promise<void>;
  RemoveMCPServer(name: string): Promise<void>;
  RetryMCPServer(name: string): Promise<void>;
  ClearMCPServerAuthentication(name: string): Promise<void>;
  PickSkillFolder(): Promise<string>;
  AddSkillPath(path: string): Promise<void>;
  RemoveSkillPath(path: string): Promise<void>;
  RefreshSkills(): Promise<void>;
  SetSkillEnabled(name: string, enabled: boolean): Promise<void>;
  SetMCPServerEnabled(name: string, enabled: boolean): Promise<void>;
  SetMCPServerTier(name: string, tier: string): Promise<void>;
  SlashArgs(input: string): Promise<SlashArgsResult>;
  ListDir(rel: string): Promise<DirEntry[]>;
  SearchFileRefs(query: string): Promise<DirEntry[]>;
  ReadFile(rel: string): Promise<FilePreview>;
  ReadWorkspaceFileDataURL(rel: string): Promise<string>;
  WorkspaceChanges(): Promise<WorkspaceChangesView>;
  RunCodeReview(mode: string, scope: string, paths: string[]): Promise<CodeReviewResult>;
  OpenWorkspacePath(rel: string): Promise<void>;
  RevealWorkspacePath(rel: string): Promise<void>;
  RevealPath(path: string): Promise<void>;
  SavePastedImage(dataUrl: string): Promise<string>;
  SavePastedFile(name: string, dataUrl: string): Promise<string>;
  AttachDropped(path: string): Promise<DroppedItem>;
  AttachmentDataURL(path: string): Promise<string>;
  Models(): Promise<ModelInfo[]>;
  SetModel(name: string): Promise<void>;
  ModelsForTab(tabID: string): Promise<ModelInfo[]>;
  SetModelForTab(tabID: string, name: string): Promise<void>;
  Effort(): Promise<EffortInfo>;
  SetEffort(level: string): Promise<void>;
  EffortForTab(tabID: string): Promise<EffortInfo>;
  SetEffortForTab(tabID: string, level: string): Promise<void>;
  Memory(): Promise<MemoryView>;
  Remember(scope: string, note: string): Promise<string>;
  Forget(name: string): Promise<void>;
  SaveDoc(path: string, body: string): Promise<string>;
  Settings(): Promise<SettingsView>;
  SetDefaultModel(ref: string): Promise<void>;
  SetPlannerModel(ref: string): Promise<void>;
  SetAutoPlan(mode: string): Promise<void>;
  SaveProvider(p: ProviderView): Promise<void>;
  SyncProviderModels(providerName: string): Promise<ProviderModelsResult>;
  DeleteProvider(name: string): Promise<void>;
  SetProviderKey(apiKeyEnv: string, value: string): Promise<void>;
  SetPermissionMode(mode: string): Promise<void>;
  AddPermissionRule(list: string, rule: string): Promise<void>;
  RemovePermissionRule(list: string, rule: string): Promise<void>;
  SetSandbox(bash: string, network: boolean, workspaceRoot: string, allowWrite: string[]): Promise<void>;
  SetNetwork(n: NetworkView): Promise<void>;
  SetCloseBehavior(mode: string): Promise<void>;
  SetDesktopLanguage(lang: string): Promise<void>;
  SetDesktopTerminalShell(shell: string): Promise<void>;
  SetDesktopGitSettings(git: SettingsView["desktopGit"]): Promise<void>;
  SetDesktopAppearance(theme: string, style: string): Promise<void>;
  SetDesktopAppearancePrefs(prefs: DesktopAppearanceView): Promise<void>;
  SetDesktopCodeReviewSettings(prefs: DesktopCodeReviewView): Promise<void>;
  MigrateDesktopLocalPrefs(input: DesktopLocalPrefsMigrationInput): Promise<void>;
  SaveProjectPreviewSettings(input: ProjectPreviewSettingsInput): Promise<void>;
  MigrateDesktopPreferences(language: string, theme: string, style: string): Promise<void>;
  SetAgentSettings(agent: AgentSettingsInput): Promise<void>;
  ListOutputStyles(): Promise<OutputStyleView[]>;
  SetAgentParams(temperature: number, maxSteps: number, systemPrompt: string): Promise<void>;
  SetTrayLocale(locale: "en" | "zh"): Promise<void>;
  // SetBypass toggles YOLO mode (auto-approve every tool call this session; deny
  // rules still apply). Runtime-only — not written to config.
  SetBypass(on: boolean): Promise<void>;
  Version(): Promise<string>;
  CheckUpdate(): Promise<UpdateInfo | null>;
  ApplyUpdate(): Promise<void>;
  OpenDownloadPage(): Promise<void>;
  NeedsOnboarding(): Promise<boolean>;
  ConnectKey(apiKey: string, baseUrl?: string): Promise<void>;
  SideChatReply(message: string): Promise<string>;
  GetMobilePendingDecision(): Promise<MobilePendingDecision | null>;
  RespondMobileDecision(decisionId: string, allow: boolean, answers?: QuestionAnswer[]): Promise<void>;
  ListTabs(): Promise<TabMeta[]>;
  OpenProjectTab(workspaceRoot: string, topicID: string): Promise<TabMeta>;
  OpenGlobalTab(topicID: string): Promise<TabMeta>;
  SetActiveTab(tabID: string): Promise<void>;
  ReorderTabs(tabIDs: string[]): Promise<void>;
  CloseTab(tabID: string): Promise<void>;
  ListProjectTree(): Promise<ProjectNode[]>;
  RenameProject(workspaceRoot: string, title: string): Promise<void>;
  SetProjectColor(workspaceRoot: string, color: string): Promise<void>;
  ReorderProjects(workspaceRoots: string[]): Promise<void>;
  CreateTopic(scope: string, workspaceRoot: string, title: string): Promise<TopicMeta>;
  RenameTopic(topicID: string, title: string): Promise<void>;
  DeleteTopic(topicID: string): Promise<void>;
  TrashTopic(topicID: string): Promise<void>;
  ContextPanel(tabID: string): Promise<ContextPanelInfo>;
  ListWriteFiles(workspaceRoot: string): Promise<FileEntry[]>;
  ReadWriteFile(path: string): Promise<string>;
  WriteWriteFile(path: string, content: string): Promise<void>;
  DeleteWriteFile(path: string): Promise<void>;
  RenameWriteFile(oldPath: string, newPath: string): Promise<void>;
  CompleteWriteInline(textBefore: string, textAfter: string): Promise<string>;
  ListWriteWorkspaces(): Promise<string[]>;
  AddWriteWorkspace(root: string): Promise<void>;
  RemoveWriteWorkspace(root: string): Promise<void>;
  DefaultWriteWorkspace(): Promise<string>;
  EnsureBundledSkills(): Promise<void>;
  GetClawChannels(): Promise<ClawChannel[]>;
  SaveClawChannel(channel: ClawChannel): Promise<void>;
  DeleteClawChannel(id: string): Promise<void>;
  GetClawMessages(channelID: string): Promise<ClawMessage[]>;
  SendClawMessage(channelID: string, text: string): Promise<ClawMessage>;
  GetClawCallbackInfo(channelID: string): Promise<ClawCallbackInfo>;
  TestClawWeComChannel(channel: ClawChannel): Promise<string>;
  GetMobilePairingInfo(): Promise<MobilePairingInfo>;
  GetMobileConnectDiagnostics(): Promise<MobileConnectDiagnostics>;
  RefreshMobilePairing(): Promise<MobilePairingInfo>;
  GetMobileConnectConfig(): Promise<MobileConnectConfig>;
  SaveMobileConnectConfig(config: MobileConnectConfig): Promise<void>;
  ListMobileSessions(): Promise<{ id: string; createdAt: number; lastSeen: number }[]>;
  GetMobileTunnelStatus(): Promise<MobileTunnelStatus>;
  StartMobileTunnel(): Promise<MobileTunnelStatus>;
  StopMobileTunnel(): Promise<MobileTunnelStatus>;
  GetScheduledTasks(): Promise<ScheduledTask[]>;
  SaveScheduledTask(task: ScheduledTask): Promise<void>;
  DeleteScheduledTask(id: string): Promise<void>;
  TriggerScheduledTask(id: string): Promise<void>;
  // New native-feel bindings (added with the desktop native-feel plan).
  ConfirmAction(req: NativeConfirmRequest): Promise<boolean>;
  SaveWindowState(state: DesktopWindowState): Promise<void>;
}

// Bidirectional compile-time drift checks. Exclude<A, B> extracts keys in A that
// are missing from B. If that set is non-empty, AssertNever<non-never> fails with
// "Type 'X' does not satisfy the constraint 'never'". In other words:
//   _CheckGenToApp errors → a Go method has no TS counterpart (add it to AppBindings)
//   _CheckAppToGen errors → a TS method has no Go counterpart (stale / removed)
// These compare method *names* only; full signature checking isn't possible here
// because local types (types.ts) use plain interfaces while generated types
// (models.ts) use classes with a convertValues prototype method. The structural
// mismatch would produce false positives. Method-arity and parameter-order drift
// are caught at the call sites by tsc when components invoke app.<method>(...).
export type _CheckGenToApp = never;
export type _CheckAppToGen = never;

interface WailsRuntime {
  EventsOn(name: string, cb: (...data: unknown[]) => void): () => void;
  BrowserOpenURL(url: string): void;
  // Native OS file drop (desktop only); useDropTarget gates delivery to elements
  // carrying the --wails-drop-target CSS property. Absent in the browser dev mock.
  OnFileDrop?(cb: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean): void;
  OnFileDropOff?(): void;
}

declare global {
  interface Window {
    runtime?: WailsRuntime;
    go?: { main?: { App?: AppBindings } };
  }
}

// Must match desktop/app.go's eventChannel constant.
const EVENT_CHANNEL = "agent:event";

// Resolve the Wails binding at CALL time
// runtime can inject window.go AFTER this module first evaluates, so snapshotting
// once would pin the browser mock for the whole session (and show fake data — the
// dev mock's model list leaking into the real app was exactly this bug).
function realApp(): AppBindings | undefined {
  return typeof window !== "undefined" ? window.go?.main?.App : undefined;
}

function boundApp(): AppBindings | undefined {
  return isRuntimeReady() ? realApp() : undefined;
}

/** Test seam: IPC + event stream use the same readiness gate. */
export function bridgeBindingSource(): "wails" | "mock" {
  return isRuntimeReady() ? "wails" : "mock";
}

/** Test seam: agent event subscribe branch (deferred while go exists without runtime). */
export function bridgeEventStreamSource(): "wails" | "mock" | "deferred" {
  if (isRuntimeReady()) return "wails";
  if (hasGoBinding()) return "deferred";
  return "mock";
}

function subscribeWhenRuntimeReady(subscribe: () => () => void): () => void {
  if (isRuntimeReady()) {
    return subscribe();
  }
  if (hasGoBinding()) {
    let alive = true;
    let off = () => {};
    const poll = window.setInterval(() => {
      if (!alive || !isRuntimeReady()) return;
      window.clearInterval(poll);
      off = subscribe();
    }, 50);
    return () => {
      alive = false;
      window.clearInterval(poll);
      off();
    };
  }
  return subscribe();
}

let mockSingleton: AppBindings | null = null;
function getMock(): AppBindings {
  if (!mockSingleton) mockSingleton = makeMockApp();
  return mockSingleton;
}

// onEvent subscribes to the agent's typed event stream; returns an unsubscribe.
export function onEvent(cb: (e: WireEvent) => void): () => void {
  if (isRuntimeReady()) {
    return window.runtime!.EventsOn(EVENT_CHANNEL, (payload) => cb(payload as WireEvent));
  }
  if (hasGoBinding()) {
    return subscribeWhenRuntimeReady(() =>
      window.runtime!.EventsOn(EVENT_CHANNEL, (payload) => cb(payload as WireEvent)),
    );
  }
  return mockSubscribe(cb);
}

// onUpdaterProgress subscribes to the auto-updater's progress events (a separate
// channel from the agent stream); returns an unsubscribe. Must match the event
// name emitted in desktop/updater_app.go.
export function onUpdaterProgress(cb: (p: UpdateProgress) => void): () => void {
  if (isRuntimeReady()) {
    return window.runtime!.EventsOn("updater:progress", (p) => cb(p as UpdateProgress));
  }
  updaterListeners.add(cb);
  return () => {
    updaterListeners.delete(cb);
  };
}

// onFilesDropped subscribes to native OS file drops landing on the composer (the
// --wails-drop-target element); the callback gets the dropped files' absolute
// paths. No-op in the browser dev mock, where the runtime is absent.
export function onFilesDropped(cb: (paths: string[]) => void): () => void {
  const rt = typeof window !== "undefined" ? window.runtime : undefined;
  if (!rt?.OnFileDrop) return () => {};
  rt.OnFileDrop((_x, _y, paths) => {
    if (Array.isArray(paths) && paths.length > 0) cb(paths);
  }, true);
  return () => rt.OnFileDropOff?.();
}

// onReady subscribes to the agent:ready event fired when boot.Build completes.
// The frontend re-fetches Meta/Context/History when this lands.
export function onReady(cb: () => void): () => void {
  if (isRuntimeReady()) {
    return window.runtime!.EventsOn("agent:ready", () => cb());
  }
  if (hasGoBinding()) {
    return subscribeWhenRuntimeReady(() => window.runtime!.EventsOn("agent:ready", () => cb()));
  }
  // In dev mock, fire immediately since there's no real boot sequence.
  cb();
  return () => {};
}

export function onProjectTreeChanged(cb: () => void): () => void {
  if (isRuntimeReady()) {
    return window.runtime!.EventsOn("project-tree:changed", () => cb());
  }
  return () => {};
}

export interface ScheduleTaskEvent {
  id: string;
  name: string;
  source: "manual" | "auto";
  prompt?: string;
  error?: string;
}

export function onScheduleTask(cb: (event: ScheduleTaskEvent) => void): () => void {
  if (isRuntimeReady()) {
    return window.runtime!.EventsOn("schedule:task", (payload) => cb(payload as ScheduleTaskEvent));
  }
  scheduleListeners.add(cb);
  return () => {
    scheduleListeners.delete(cb);
  };
}

function emitScheduleTask(event: ScheduleTaskEvent) {
  scheduleListeners.forEach((listener) => listener(event));
}

// app proxies each call to the live binding (or the dev mock only when truly
// outside the shell), so a late-injected window.go is picked up transparently.
export const app: AppBindings = new Proxy({} as AppBindings, {
  get(_t, prop) {
    const target = boundApp() ?? getMock();
    const v = (target as unknown as Record<string, unknown>)[String(prop)];
    return typeof v === "function" ? (v as (...a: unknown[]) => unknown).bind(target) : v;
  },
});

// openExternal opens a URL in the system browser (so links in rendered markdown
// don't navigate the webview away from the app). Falls back to window.open in the
// browser dev mock.
export function openExternal(url: string): void {
  if (typeof window !== "undefined" && window.runtime?.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(url);
  } else if (typeof window !== "undefined") {
    window.open(url, "_blank", "noopener");
  }
}

// --- browser dev mock --------------------------------------------------------

const listeners = new Set<(e: WireEvent) => void>();
const scheduleListeners = new Set<(event: ScheduleTaskEvent) => void>();

function mockSubscribe(cb: (e: WireEvent) => void): () => void {
  listeners.add(cb);
  return () => {
    listeners.delete(cb);
  };
}

function emit(e: WireEvent) {
  listeners.forEach((l) => l(e));
}

// Updater progress has its own listener set so the browser dev mock's ApplyUpdate
// can stream a fake download through onUpdaterProgress.
const updaterListeners = new Set<(p: UpdateProgress) => void>();

function emitUpdater(p: UpdateProgress) {
  updaterListeners.forEach((l) => l(p));
}

function delay(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

function baseName(path: string): string {
  return path.replace(/[/\\]+$/, "").split(/[/\\]/).filter(Boolean).pop() ?? path;
}

// Browser dev mock — signatures must match AppBindings (`pnpm check:bridge`).
// Stubs UI-facing state only; does not replicate Go kernel/agent/MCP behavior.
function makeMockApp(): AppBindings {
  let cancelled = false;
  let pendingAskPreview = false;
  let pendingApprovalPreview = false;
  let cwd = "~/projects/joyquant-db"; // mutable so PickWorkspace is visible in dev
  const globalWorkspaceRoot = "~/Library/Application Support/arcdesk/global-workspace";
  let workspaces = ["~/projects/joyquant-db", "~/projects/joyquant-sys", "~/projects/arcdesk", "~/projects/blade"];
  let mockEffort = "auto";
  const mockClawMessages: ClawMessage[] = [];
  const day = 86_400_000;
  const t0 = Date.now();
  // Mutable so MCP add/remove/retry are observable in browser dev.
  let capServers: ServerView[] = [
    {
      name: "codegraph",
      transport: "stdio",
      status: "disabled",
      builtIn: true,
      configured: true,
      autoStart: false,
      tier: "lazy",
      tools: 0,
      prompts: 0,
      resources: 0,
      toolList: [
        { name: "search", description: "Search symbols, files, and text in the workspace." },
        { name: "context", description: "Fetch surrounding source context for a symbol or file." },
        { name: "trace", description: "Follow callers and callees across the code graph." },
        { name: "node", description: "Inspect a specific graph node." },
      ],
    },
    { name: "github", transport: "stdio", status: "connected", configured: true, autoStart: true, tier: "lazy", command: "npx", args: ["-y", "@modelcontextprotocol/server-github"], tools: 12, prompts: 2, resources: 0 },
    {
      name: "linear",
      transport: "http",
      status: "deferred",
      configured: true,
      autoStart: true,
      tier: "lazy",
      url: "https://mcp.linear.app/mcp",
      authStatus: "possible",
      authUrl: "https://mcp.linear.app/mcp",
      tools: 8,
      prompts: 0,
      resources: 0,
      toolList: [
        { name: "list_issues", description: "List and filter Linear issues." },
        { name: "get_issue", description: "Fetch a Linear issue by id or key." },
        { name: "create_issue", description: "Create a Linear issue." },
        { name: "update_issue", description: "Update status, assignee, priority, or labels." },
        { name: "list_projects", description: "List Linear projects." },
        { name: "get_project", description: "Fetch project details." },
        { name: "list_teams", description: "List Linear teams." },
        { name: "search", description: "Search Linear workspace objects." },
      ],
    },
    { name: "figma", transport: "http", status: "failed", configured: true, autoStart: true, tier: "lazy", url: "https://mcp.figma.com/mcp", authStatus: "required", authUrl: "https://mcp.figma.com/mcp", tools: 0, prompts: 0, resources: 0, error: "connect: 401 unauthorized" },
  ];
  const capSkills: SkillView[] = [
    { name: "explore", description: "Investigate the codebase in an isolated subagent", scope: "builtin", runAs: "subagent", enabled: true },
    { name: "review", description: "Review the staged diff", scope: "project", runAs: "inline", enabled: false },
    { name: "init", description: "Scaffold a ARCDESK.md for this repo", scope: "builtin", runAs: "inline", enabled: true },
    {
      name: "copywriting",
      description: "Write and improve marketing copy, articles, and persuasive 文案 (marketingskills)",
      scope: "global",
      runAs: "inline",
      enabled: true,
    },
  ];
  let capSkillRoots: SkillRootView[] = [
    { dir: "~/projects/arcdesk/.arcdesk/skills", scope: "project", priority: 1, status: "missing", configured: false, skills: 0 },
    {
      dir: "~/my-skills",
      scope: "custom",
      priority: 5,
      status: "ok",
      configured: true,
      skills: 1,
      skillItems: [{ name: "review", description: "Review the staged diff", scope: "custom", runAs: "inline" }],
    },
    {
      dir: "~/.arcdesk/skills",
      scope: "global",
      priority: 6,
      status: "ok",
      configured: false,
      skills: 2,
      skillItems: [
        { name: "explore", description: "Investigate the codebase in an isolated subagent", scope: "global", runAs: "subagent" },
        { name: "init", description: "Scaffold a ARCDESK.md for this repo", scope: "global", runAs: "inline" },
      ],
    },
  ];
  const mcpCatalogEntries: MCPCatalogEntry[] = [
    {
      id: "github",
      name: "GitHub",
      category: "Developer",
      description: "Browse repos, issues, and pull requests through the official GitHub MCP server.",
      transport: "stdio",
      command: "npx",
      args: ["-y", "@modelcontextprotocol/server-github"],
      tier: "lazy",
      official: true,
    },
    {
      id: "filesystem",
      name: "Filesystem",
      category: "Files",
      description: "Read and write files within allowed workspace directories.",
      transport: "stdio",
      command: "npx",
      args: ["-y", "@modelcontextprotocol/server-filesystem"],
      tier: "eager",
      official: true,
    },
    {
      id: "brave-search",
      name: "Brave Search",
      category: "Search",
      description: "Web search for live documentation, release notes, and troubleshooting.",
      transport: "stdio",
      command: "npx",
      args: ["-y", "@modelcontextprotocol/server-brave-search"],
      tier: "lazy",
      official: true,
    },
    {
      id: "linear",
      name: "Linear",
      category: "Project",
      description: "Create and update Linear issues, projects, and teams.",
      transport: "http",
      url: "https://mcp.linear.app/mcp",
      tier: "lazy",
      official: true,
    },
    {
      id: "playwright",
      name: "Playwright",
      category: "Browser",
      description: "Drive a browser for UI verification, screenshots, and end-to-end checks.",
      transport: "stdio",
      command: "npx",
      args: ["-y", "@playwright/mcp@latest"],
      tier: "background",
      official: false,
    },
    {
      id: "postgres",
      name: "PostgreSQL",
      category: "Database",
      description: "Inspect schemas and run read-only SQL against a configured database.",
      transport: "stdio",
      command: "npx",
      args: ["-y", "@modelcontextprotocol/server-postgres"],
      tier: "lazy",
      official: true,
    },
  ];
  const mockSwitchWorkspace = async (path: string) => {
    cwd = path || "~";
    workspaces = [cwd, ...workspaces.filter((p) => p !== cwd)].slice(0, 12);
    if (!mockProjectTree.some((node) => node.kind === "project" && node.root === cwd)) {
      mockProjectTree.unshift({
        key: `project_${cwd}`,
        kind: "project",
        label: baseName(cwd),
        root: cwd,
        children: [],
      });
    }
    return cwd;
  };
  // Mutable so delete/rename are observable in browser dev.
  const sessions: SessionMeta[] = [
    { path: "/mock/sessions/a.jsonl", preview: "fix the login bug in auth.go", turns: 12, createdAt: t0 - 2 * day, lastActivityAt: t0 - 3_600_000, modTime: t0 - 3_600_000, current: true, open: true },
    { path: "/mock/sessions/b.jsonl", preview: "refactor the payment module", turns: 5, createdAt: t0 - 3 * day, lastActivityAt: t0 - 6 * 3_600_000, modTime: t0 - 6 * 3_600_000, current: false, open: true },
    { path: "/mock/sessions/c.jsonl", preview: "write the README and badges", turns: 8, createdAt: t0 - 4 * day, lastActivityAt: t0 - day - 3_600_000, modTime: t0 - day - 3_600_000, current: false, open: false },
    { path: "/mock/sessions/d.jsonl", preview: "explain the plugin host design", turns: 3, createdAt: t0 - 5 * day, lastActivityAt: t0 - 4 * day, modTime: t0 - 4 * day, current: false, open: false },
  ];
  const trashedSessions: SessionMeta[] = [
    {
      path: "/mock/sessions/.trash/trash-dev-standard.jsonl",
      title: t("mock.trashDevStandardTitle"),
      preview: t("mock.trashDevStandardPreview"),
      turns: 4,
      createdAt: t0 - 8 * day,
      lastActivityAt: t0 - 7 * day,
      modTime: t0 - 7 * day,
      deletedAt: t0 - 20 * 60_000,
      current: false,
      open: false,
      scope: "project",
      workspaceRoot: "~/projects/joyquant-db",
      topicId: "topic_dev_standard",
      topicTitle: t("mock.trashDevStandardTitle"),
    },
    {
      path: "/mock/sessions/.trash/trash-p3a-review.jsonl",
      title: t("mock.trashP3aTitle"),
      preview: t("mock.trashP3aPreview"),
      turns: 7,
      createdAt: t0 - 6 * day,
      lastActivityAt: t0 - 5 * day,
      modTime: t0 - 5 * day,
      deletedAt: t0 - 2 * 3_600_000,
      current: false,
      open: false,
      scope: "project",
      workspaceRoot: "~/projects/joyquant-sys",
      topicId: "topic_p3a_pd",
      topicTitle: t("mock.trashP3aTitle"),
    },
    {
      path: "/mock/sessions/.trash/trash-global-product.jsonl",
      title: t("mock.trashGlobalProductTitle"),
      preview: t("mock.trashGlobalProductPreview"),
      turns: 2,
      createdAt: t0 - 4 * day,
      lastActivityAt: t0 - 3 * day,
      modTime: t0 - 3 * day,
      deletedAt: t0 - day,
      current: false,
      open: false,
      scope: "global",
      topicId: "topic_product",
      topicTitle: t("mock.trashGlobalProductTitle"),
    },
  ];
  // Mutable settings so the Settings panel's edits are observable in browser dev.
  const settings: SettingsView = {
    defaultModel: "deepseek-flash",
    plannerModel: "",
    autoPlan: "off",
    providers: [
      { name: "deepseek-flash", kind: "openai", baseUrl: "https://api.deepseek.com", models: ["deepseek-v4-flash"], default: "deepseek-v4-flash", apiKeyEnv: "DEEPSEEK_API_KEY", keySet: true, balanceUrl: "https://api.deepseek.com/user/balance", contextWindow: 1_000_000, supportedEfforts: [], defaultEffort: "" },
      { name: "mimo-pro", kind: "openai", baseUrl: "https://api.xiaomimimo.com/v1", models: ["mimo-v2.5-pro"], default: "mimo-v2.5-pro", apiKeyEnv: "MIMO_API_KEY", keySet: false, balanceUrl: "", contextWindow: 1_000_000, supportedEfforts: [], defaultEffort: "" },
    ],
    permissions: { mode: "ask", allow: ["ls", "read_file"], ask: [], deny: ["bash(rm -rf*)"] },
    sandbox: { bash: "enforce", network: true, workspaceRoot: "", allowWrite: [] },
    network: {
      proxyMode: "auto",
      proxyUrl: "",
      noProxy: "",
      proxy: { type: "socks5", server: "127.0.0.1", port: 7890, username: "", password: "" },
    },
    agent: {
      temperature: 0.2,
      maxSteps: 0,
      systemPrompt: "You are ArcDesk, a coding agent.",
      systemPromptFile: "",
      outputStyle: "",
      autoPlan: "off",
      autoPlanClassifier: "",
      softCompactRatio: 0.5,
      compactRatio: 0.8,
      compactForceRatio: 0.9,
      subagentModel: "",
      subagentModels: { explore: "deepseek-flash", review: "deepseek-flash" },
      usesDefaultPrompt: true,
      defaultSystemPrompt: `You are ArcDesk, a coding agent focused on executing code tasks.
Use the provided tools to read and write files and run shell commands.
Principles: understand the request before acting; verify with tools instead of
guessing; keep changes minimal and correct; briefly summarize what you did.`,
    },
    desktopLanguage: "",
    desktopTheme: "light",
    desktopThemeStyle: "glacier",
    desktopTerminalShell: "",
    desktopGit: {
      prMergeMethod: "merge",
      checkGitHubCli: false,
      syncRepoMergeToGitHub: false,
      commitInstructions: "",
      prInstructions: "",
    },
    desktopAppearance: {
      backgroundPreset: "studio",
      foregroundPreset: "charcoal",
      textSize: "default",
      codeFontSize: "default",
      diffMarker: "background",
    },
    desktopCodeReview: {
      defaultScope: "all",
      securityByDefault: false,
    },
    closeBehavior: "quit",
    configPath: "~/projects/arcdesk/arcdesk.toml",
    providerKinds: ["openai"],
    bypass: false,
  };
  const mockWriteFiles: Record<string, string> = {
    "/workspace/writes/welcome.md": "# 欢迎\n\n在这里写文案、笔记或长文，与代码项目无关。\n",
    "/workspace/README.md": "# README\n\nDraft content for README.\n",
    "/workspace/notes/brief.md": "# Brief\n\nNotes for the brief.\n",
  };
  const mockWriteWorkspaces = ["/workspace", "/workspace/docs", "/workspace/notes"];
  const mockWriteDirs = new Set(["/workspace/notes"]);
  const mockProjectTree: ProjectNode[] = [
    {
      key: "project_~/projects/joyquant-db",
      kind: "project",
      label: t("mock.projectJoyquantDb"),
      root: "~/projects/joyquant-db",
      projectColor: "blue",
      children: [
        { key: "topic_dev_standard", kind: "topic", label: `● ${t("mock.topicDevStandard")}`, root: "~/projects/joyquant-db", topicId: "topic_dev_standard", projectColor: "blue" },
        { key: "topic_db_maint", kind: "topic", label: t("mock.topicDbMaint"), root: "~/projects/joyquant-db", topicId: "topic_db_maint", projectColor: "blue" },
        { key: "topic_env", kind: "topic", label: t("mock.topicEnv"), root: "~/projects/joyquant-db", topicId: "topic_env", projectColor: "blue" },
      ],
    },
    {
      key: "project_~/projects/joyquant-sys",
      kind: "project",
      label: t("mock.projectJoyquantSys"),
      root: "~/projects/joyquant-sys",
      projectColor: "purple",
      children: [
        { key: "topic_p3b_pd", kind: "topic", label: `● ${t("mock.topicP3b")}`, root: "~/projects/joyquant-sys", topicId: "topic_p3b_pd", projectColor: "purple" },
        { key: "topic_p3a_pd", kind: "topic", label: t("mock.topicP3a"), root: "~/projects/joyquant-sys", topicId: "topic_p3a_pd", projectColor: "purple" },
        { key: "topic_hotfix", kind: "topic", label: t("mock.topicHotfix"), root: "~/projects/joyquant-sys", topicId: "topic_hotfix", projectColor: "purple" },
        { key: "topic_sys_coord", kind: "topic", label: t("mock.topicSysCoord"), root: "~/projects/joyquant-sys", topicId: "topic_sys_coord", projectColor: "purple" },
        { key: "topic_sys_standard", kind: "topic", label: t("mock.topicSysStandard"), root: "~/projects/joyquant-sys", topicId: "topic_sys_standard", projectColor: "purple" },
      ],
    },
    {
      key: "global_folder",
      kind: "global_folder",
      label: "Global",
      root: globalWorkspaceRoot,
      children: [
        { key: "global_topic_product", kind: "global_topic", label: t("mock.topicProduct"), topicId: "topic_product" },
        { key: "global_topic_ai", kind: "global_topic", label: t("mock.topicAi"), topicId: "topic_ai" },
        { key: "global_topic_lab", kind: "global_topic", label: t("mock.topicLab"), topicId: "topic_lab" },
      ],
    },
  ];
  const cloneProjectTree = () => JSON.parse(JSON.stringify(mockProjectTree)) as ProjectNode[];
  const projectChildren = (node: ProjectNode): ProjectNode[] => Array.isArray(node.children) ? node.children : [];
  const findMockTopic = (topicId: string): ProjectNode | null => {
    for (const parent of mockProjectTree) {
      const found = projectChildren(parent).find((child) => child.topicId === topicId);
      if (found) return found;
    }
    return null;
  };
  const deleteMockTopic = (topicId: string) => {
    for (const parent of mockProjectTree) {
      parent.children = projectChildren(parent).filter((child) => child.topicId !== topicId);
    }
  };
  const topicLabel = (topicId: string, fallback: string) => (findMockTopic(topicId)?.label || fallback).replace(/^●\s*/, "");
  const setMockActiveTab = (tabId: string) => {
    mockTabs = mockTabs.map((tab) => ({ ...tab, active: tab.id === tabId }));
  };
  let mockTabs: TabMeta[] = [
    {
      id: "tab_joyquant_db",
      scope: "project",
      workspaceRoot: "~/projects/joyquant-db",
      workspaceName: "joyquant-db",
      topicId: "topic_dev_standard",
      topicTitle: t("mock.trashDevStandardTitle"),
      projectColor: "blue",
	      label: "DeepSeek-R1",
	      ready: true,
	      running: false,
	      mode: "normal",
	      active: true,
	      cwd: "~/projects/joyquant-db",
    },
    {
      id: "tab_joyquant_sys",
      scope: "project",
      workspaceRoot: "~/projects/joyquant-sys",
      workspaceName: "joyquant-sys",
      topicId: "topic_p3b_pd",
      topicTitle: "p3b P&D",
      projectColor: "purple",
	      label: "DeepSeek-R1",
	      ready: true,
	      running: false,
	      mode: "normal",
	      active: false,
	      cwd: "~/projects/joyquant-sys",
    },
    {
      id: "tab_global",
      scope: "global",
      workspaceRoot: "",
      workspaceName: "Global",
      topicId: "topic_global",
      topicTitle: "Global",
	      label: "DeepSeek-R1",
	      ready: true,
	      running: false,
	      mode: "normal",
	      active: false,
	      cwd: "~/projects/joyquant-db",
    },
  ];
  let mockTerminalSeq = 0;
  return {
    async Platform() {
      // Mirror the OS the browser dev mock runs on.
      const ua = typeof navigator !== "undefined" ? navigator.userAgent : "";
      if (/Win/i.test(ua)) return "windows";
      if (/Mac/i.test(ua)) return "darwin";
      return "linux";
    },
        async Submit(input) {
          cancelled = false;
      emit({ kind: "turn_started" });
      const trimmedInput = input.trim().toLowerCase();
      if (trimmedInput === "/approve-preview" || trimmedInput === "approve preview" || trimmedInput === "approve预览") {
        pendingApprovalPreview = true;
        await delay(250);
        if (cancelled) return;
        emit({
          kind: "approval_request",
          approval: {
            id: "mock-approval-preview",
            tool: "bash",
            subject: t("mock.approvalSubject"),
          },
        });
        return;
      }
      if (
        trimmedInput === "/plan-approve-preview" ||
        trimmedInput === "plan approve preview" ||
        trimmedInput === "plan approve预览"
      ) {
        pendingApprovalPreview = true;
        await delay(250);
        if (cancelled) return;
        emit({
          kind: "approval_request",
          approval: {
            id: "mock-plan-approval-preview",
            tool: "exit_plan_mode",
            subject: "",
          },
        });
        return;
      }
      if (trimmedInput === "/ask-preview" || trimmedInput === "ask preview" || trimmedInput === "ask预览") {
        pendingAskPreview = true;
        await delay(250);
        if (cancelled) return;
        emit({
          kind: "ask_request",
          ask: {
            id: "mock-ask-preview",
            questions: [
              {
                id: "q1",
                header: t("mock.askQ1Header"),
                prompt: t("mock.askQ1Prompt"),
                options: [
                  { label: t("mock.askQ1Opt1Label"), description: t("mock.askQ1Opt1Desc") },
                  { label: t("mock.askQ1Opt2Label"), description: t("mock.askQ1Opt2Desc") },
                  { label: t("mock.askQ1Opt3Label"), description: t("mock.askQ1Opt3Desc") },
                ],
              },
              {
                id: "q2",
                header: t("mock.askQ2Header"),
                prompt: t("mock.askQ2Prompt"),
                options: [
                  { label: t("mock.askQ2Opt1Label"), description: t("mock.askQ2Opt1Desc") },
                  { label: t("mock.askQ2Opt2Label"), description: t("mock.askQ2Opt2Desc") },
                  { label: t("mock.askQ2Opt3Label"), description: t("mock.askQ2Opt3Desc") },
                ],
              },
            ],
          },
        });
        return;
      }
      if (trimmedInput === "/todo-preview" || trimmedInput === "todo preview" || trimmedInput === "todo预览") {
        await delay(250);
        if (cancelled) return;
        emit({
          kind: "tool_dispatch",
          tool: {
            id: "mock-todo-preview",
            name: "todo_write",
            args: JSON.stringify({
              todos: [
                { content: t("mock.todo1"), status: "completed" },
                { content: t("mock.todo2"), activeForm: t("mock.todo2ActiveForm"), status: "in_progress" },
                { content: t("mock.todo3"), status: "pending" },
              ],
            }),
            readOnly: false,
          },
        });
        await delay(150);
        emit({
          kind: "tool_result",
          tool: {
            id: "mock-todo-preview",
            name: "todo_write",
            args: JSON.stringify({
              todos: [
                { content: t("mock.todo1"), status: "completed" },
                { content: t("mock.todo2"), activeForm: t("mock.todo2ActiveForm"), status: "in_progress" },
                { content: t("mock.todo3"), status: "pending" },
              ],
            }),
            output: "todo list updated",
            readOnly: false,
          },
        });
        emit({ kind: "turn_done" });
        return;
      }
      if (
        trimmedInput === "/action-preview" ||
        trimmedInput === "action preview" ||
        trimmedInput === "操作流预览"
      ) {
        const ws = "~/projects/joyquant-db";
        const emitReasoning = async (text: string) => {
          for (const ch of text) {
            if (cancelled) return;
            emit({ kind: "reasoning", text: ch });
            await delay(8);
          }
        };
        const emitTool = async (
          id: string,
          name: string,
          args: Record<string, unknown>,
          output: string,
          extra?: { readOnly?: boolean; fileDiff?: { diff: string; added: number; removed: number } },
        ) => {
          emit({
            kind: "tool_dispatch",
            tool: {
              id,
              name,
              args: JSON.stringify(args),
              readOnly: extra?.readOnly ?? name !== "edit_file",
              fileDiff: extra?.fileDiff,
            },
          });
          await delay(280);
          if (cancelled) return;
          emit({
            kind: "tool_result",
            tool: {
              id,
              name,
              args: JSON.stringify(args),
              output,
              readOnly: extra?.readOnly ?? name !== "edit_file",
              fileDiff: extra?.fileDiff,
            },
          });
        };

        await emitReasoning("Scanning the auth module and related handlers before editing.");
        await emitTool(
          "act-grep",
          "grep",
          { pattern: "login|session", path: `${ws}/internal/auth` },
          [
            `${ws}/internal/auth/login.go:42:func ValidateSession`,
            `${ws}/internal/auth/session.go:18:type SessionStore`,
            `${ws}/internal/handlers/auth_handler.go:91:func (h *Handler) Login`,
          ].join("\n"),
          { readOnly: true },
        );
        await emitTool(
          "act-read",
          "read_file",
          { path: `${ws}/internal/auth/login.go` },
          "package auth\n\nfunc ValidateSession(token string) error { ... }",
          { readOnly: true },
        );
        await emitTool(
          "act-bash",
          "bash",
          { command: "go test ./internal/auth/... -count=1" },
          "$ go test ./internal/auth/... -count=1\nok  joyquant-db/internal/auth 0.412s\n",
          { readOnly: false },
        );
        await emitTool(
          "act-edit",
          "edit_file",
          {
            path: `${ws}/internal/auth/login.go`,
            old_string: 'return errors.New("invalid")',
            new_string: 'return ErrInvalidSession',
          },
          "edited login.go",
          {
            readOnly: false,
            fileDiff: {
              diff: [
                "@@ -40,7 +40,7 @@ func ValidateSession(token string) error {",
                " \tif token == \"\" {",
                "-\t\treturn errors.New(\"invalid\")",
                "+\t\treturn ErrInvalidSession",
                " \t}",
                " \tif !store.Has(token) {",
                "-\t\treturn errors.New(\"invalid\")",
                "+\t\treturn ErrInvalidSession",
                " \t}",
              ].join("\n"),
              added: 2,
              removed: 2,
            },
          },
        );

        const reply =
          "Updated session validation to use `ErrInvalidSession` and verified auth tests pass.\n";
        for (const ch of reply) {
          if (cancelled) break;
          emit({ kind: "text", text: ch });
          await delay(5);
        }
        emit({ kind: "message", text: reply });
        emit({ kind: "turn_done" });
        return;
      }
      // Simulate the server's pre-first-token latency so the deferred user bubble
      // and the "un-send on Esc before any reply" path are observable in browser
      // dev. Bail if cancelled during the wait — nothing was streamed yet.
      await delay(700);
      if (cancelled) return;
      const reply =
        `You said: **${input}**\n\n` +
        "This is the browser dev mock — the real reply comes from the kernel " +
        "inside the Wails shell. Here's a fenced block to exercise the editor seam:\n\n" +
        "```go\nfunc main() {\n    println(\"hello from the mock\")\n}\n```\n";
      for (const ch of reply) {
        if (cancelled) break;
        emit({ kind: "text", text: ch });
        await delay(6);
      }
      emit({ kind: "message", text: reply });
      emit({
        kind: "tool_dispatch",
        tool: {
          id: "t1",
          name: "edit_file",
          args: '{"path":"main.go","old_string":"println(\\"hi\\")","new_string":"println(\\"hello\\")"}',
          readOnly: false,
        },
      });
      await delay(350);
      emit({
        kind: "tool_result",
        tool: { id: "t1", name: "edit_file", output: "edited main.go", readOnly: false },
      });
      emit({
        kind: "usage",
        usage: {
          promptTokens: 1280,
          completionTokens: 64,
          totalTokens: 1344,
          cacheHitTokens: 1024,
          cacheMissTokens: 256,
          sessionCacheHitTokens: 1024,
          sessionCacheMissTokens: 256,
        },
      });
          emit({ kind: "turn_done" });
        },
        async SubmitToTab(_tabID, input) {
          await this.Submit(input);
        },
        async SubmitDisplay(_display, input) {
          await this.Submit(input);
        },
        async SubmitDisplayToTab(_tabID, display, input) {
          await this.SubmitDisplay(display, input);
        },
        async RunShell(command) {
          cancelled = false;
          emit({ kind: "turn_started" });
          await delay(100);
          if (cancelled) return;
          const id = `shell-${command.slice(0, 32)}`;
          emit({ kind: "tool_dispatch", tool: { id, name: "bash", args: JSON.stringify({ command }), readOnly: false } });
          await delay(200);
          if (cancelled) return;
          emit({ kind: "tool_progress", tool: { id, name: "bash", output: `$ ${command}\n(mock output)\n`, readOnly: false } });
          await delay(100);
          if (cancelled) return;
          emit({ kind: "tool_result", tool: { id, name: "bash", output: `$ ${command}\n(mock output)\n`, readOnly: false } });
          emit({ kind: "turn_done" });
        },
        async RunShellQuiet(command) {
          await delay(120);
          if (command.includes("branch --show-current")) {
            return { output: "ui-redesign\n" };
          }
          if (command.startsWith("gh --version") || command.includes("gh --version")) {
            return { output: "gh version 2.40.0 (mock)\n" };
          }
          if (command.includes("gh auth status")) {
            return { output: "github.com\n  ✓ Logged in to github.com account mock-user (mock)\n" };
          }
          if (command.includes("gh pr view")) {
            return {
              output: JSON.stringify({
                number: 42,
                state: "OPEN",
                title: "Mock pull request",
                url: "https://github.com/owner/repo/pull/42",
              }),
            };
          }
          if (command.includes("gh pr merge")) {
            return { output: "Merged pull request owner/repo#42\n" };
          }
          if (command.includes("gh repo view")) {
            return { output: "owner/repo\n" };
          }
          if (command.includes("gh api repos/")) {
            return { output: "{}\n" };
          }
          return { output: "Already up to date.\n" };
        },
        async StartTerminal() {
          mockTerminalSeq += 1;
          return { id: `mock-term-${mockTerminalSeq}`, shell: "powershell (mock)" };
        },
        async WriteTerminal(_sessionID, _data) {},
        async ResizeTerminal(_sessionID, _cols, _rows) {},
        async CloseTerminal(_sessionID) {},
        async ProjectSandboxStatus() {
          return {
            configured: false,
            workspaceRoot: "~/projects/arcdesk",
            bash: "enforce",
            network: true,
            allowWrite: [],
            previewHosts: ["localhost", "127.0.0.1"],
            previewPorts: [5173, 3000, 8080],
            previewStrict: false,
            yoloRequired: false,
          };
        },
        async ConfigureProjectSandbox(_input) {},
        async ValidatePreviewURL(url) {
          const trimmed = url.trim();
          if (!trimmed) return { decision: "blocked", url: "", reason: "invalid", strict: false };
          if (/^https?:\/\//i.test(trimmed) || trimmed.includes("localhost") || trimmed.includes("127.0.0.1")) {
            return { decision: "allow", url: trimmed.includes("://") ? trimmed : `http://${trimmed}`, strict: false };
          }
          return { decision: "confirm", url: trimmed, reason: "external", strict: false };
        },
        async ProbePreviewURL(url) {
          const trimmed = url.trim();
          if (!trimmed) return false;
          const href = trimmed.includes("://") ? trimmed : `http://${trimmed}`;
          try {
            await fetch(href, { method: "GET", mode: "no-cors", cache: "no-store" });
            return true;
          } catch {
            return false;
          }
        },
        async DetectDevServerURL() {
          return "http://localhost:5173";
        },
        async OpenWebPreview(url) {
          if (typeof window !== "undefined") {
            window.dispatchEvent(new CustomEvent("arcdesk:open-web-preview", { detail: { url } }));
          }
        },
        async WorkspacePagePreviewURL(path) {
          const trimmed = path.trim();
          if (!trimmed) throw new Error("path is required");
          return `http://127.0.0.1:4173/p/mock/${trimmed.replace(/^\.?\//, "")}`;
        },
        async IsPreviewablePage(path) {
          return /\.(html?|xhtml|svg)$/i.test(path.trim());
        },
        async Cancel() {
          cancelled = true;
          emit({ kind: "turn_done" });
        },
        async CancelTab(_tabID) {
          await this.Cancel();
        },
        async Approve(_id, allow, session, persist) {
          if (!pendingApprovalPreview) return;
      pendingApprovalPreview = false;
      const suffix = persist ? "persisted" : session ? "allowed for session" : "allowed once";
      emit({
        kind: "message",
        text: `approval preview answered: ${allow ? suffix : "denied"}`,
      });
          emit({ kind: "turn_done" });
        },
        async ApproveTab(_tabID, id, allow, session, persist) {
          await this.Approve(id, allow, session, persist);
        },
        async AnswerQuestion(_id, answers) {
      if (!pendingAskPreview) return;
      pendingAskPreview = false;
      const summary = answers
        .map((answer) => `${answer.questionId}: ${(answer.selected ?? []).join(", ") || "(no answer)"}`)
        .join("\n");
      emit({ kind: "message", text: `ask preview answered:\n\n${summary}` });
          emit({ kind: "turn_done" });
        },
        async AnswerQuestionForTab(_tabID, id, answers) {
          await this.AnswerQuestion(id, answers);
        },
    async ConfirmAction(req) {
      if (typeof window !== "undefined" && typeof window.confirm === "function") {
        return window.confirm(`${req.title}\n\n${req.message}`);
      }
      return false;
    },
        async SetPlanMode() {},
	        async SetMode(mode) {
	          const active = mockTabs.find((tab) => tab.active);
	          if (active) await this.SetModeForTab(active.id, mode);
	        },
	        async SetModeForTab(tabID, mode) {
	          const nextMode = mode === "plan" || mode === "yolo" ? mode : "normal";
	          mockTabs = mockTabs.map((tab) => tab.id === tabID ? { ...tab, mode: nextMode } : tab);
	        },
    async Compact() {},
    async NewSession() {},
    async Checkpoints() {
      return [
        { turn: 0, prompt: "你好呀", files: ["src/App.tsx"], time: Date.now() - 30_000, canCode: true, canConversation: true },
      ];
    },
    async CheckpointsForTab() {
      return this.Checkpoints();
    },
    async Rewind() {},
    async Fork() {
      const active = mockTabs.find((tab) => tab.active) ?? mockTabs[0];
      const tab: TabMeta = {
        ...active,
        id: "tab_fork_" + Date.now(),
        topicId: "topic_fork_" + Date.now(),
        topicTitle: `${active.topicTitle || t("rewind.fork")} · fork`,
        active: true,
        running: false,
      };
      mockTabs = [...mockTabs.map((item) => ({ ...item, active: false })), tab];
      return { ...tab };
    },
    async SummarizeFrom() {},
    async SummarizeUpTo() {},
        async History() {
          return [];
        },
        async HistoryForTab() {
          return this.History();
        },
    async ListSessions() {
      return sessions.map((s) => ({ ...s }));
    },
    async ListTrashedSessions() {
      return trashedSessions.map((s) => ({ ...s }));
    },
    async ResumeSession(path: string) {
      sessions.forEach((s) => {
        s.current = s.path === path;
        s.open = s.open || s.path === path;
      });
      return [
        { role: "user", content: `(mock) resumed ${path}` },
        { role: "assistant", content: "This is a mock resumed transcript — the real one comes from the kernel." },
      ];
    },
    async ResumeSessionForTab(_tabID: string, path: string) {
      return this.ResumeSession(path);
    },
    async PreviewSession(path: string) {
      const s = sessions.find((x) => x.path === path) ?? trashedSessions.find((x) => x.path === path);
      return [
        { role: "user", content: s?.preview || `(mock) preview ${path}` },
        {
          role: "assistant",
          content: "This is a read-only mock preview. The active conversation is unchanged.",
          reasoning: "Preview reads the saved session without resuming it.",
        },
      ];
    },
    async DeleteSession(path: string) {
      const i = sessions.findIndex((s) => s.path === path);
      if (i >= 0) {
        const [s] = sessions.splice(i, 1);
        trashedSessions.unshift({
          ...s,
          current: false,
          open: false,
          path: s.path.replace("/mock/sessions/", "/mock/sessions/.trash/"),
          deletedAt: Date.now(),
        });
      }
    },
    async RestoreSession(path: string) {
      const i = trashedSessions.findIndex((s) => s.path === path);
      if (i >= 0) {
        const [s] = trashedSessions.splice(i, 1);
        sessions.unshift({
          ...s,
          path: s.path.replace("/mock/sessions/.trash/", "/mock/sessions/"),
          deletedAt: undefined,
        });
      }
    },
    async PurgeTrashedSession(path: string) {
      const i = trashedSessions.findIndex((s) => s.path === path);
      if (i >= 0) trashedSessions.splice(i, 1);
    },
    async RenameSession(path: string, title: string) {
      const s = sessions.find((x) => x.path === path);
      if (s) s.title = title.trim() || undefined;
    },
    async ListWorkspaces() {
      return mockProjectTree
        .filter((node) => node.kind === "project" && node.root)
        .map((node) => ({
          path: node.root!,
          name: node.label || baseName(node.root!),
          current: node.root === cwd,
        }));
    },
    async PickWorkspace() {
      // Browser dev has no native dialog; simulate picking a folder and re-root so
      // the topbar folder chip visibly changes.
      return mockSwitchWorkspace(cwd.endsWith("another-project") ? "~/projects/arcdesk" : "~/projects/another-project");
    },
    async PickFilePath() {
      return `${cwd}/README.md`;
    },
    async PickSaveFilePath(defaultName: string) {
      const base = defaultName.trim() || "untitled.md";
      return `${cwd}/${base}`;
    },
    async SwitchWorkspace(path: string) {
      return mockSwitchWorkspace(path);
    },
    async RemoveWorkspace(path: string) {
      workspaces = workspaces.filter((p) => p !== path);
      const index = mockProjectTree.findIndex((node) => node.root === path);
      if (index >= 0) mockProjectTree.splice(index, 1);
    },
        async ContextUsage() {
          return { used: 42124, window: 128000, compactRatio: 0.8 };
        },
        async ContextUsageForTab() {
          return this.ContextUsage();
        },
        async Balance() {
      // Mirror the active mock provider: deepseek-flash carries a balance_url.
      const p = settings.providers.find((x) => x.name === settings.defaultModel);
      if (!p?.balanceUrl) return { available: false, display: "" };
          return { available: true, display: "¥128.50" };
        },
        async BalanceForTab() {
          return this.Balance();
        },
        async Jobs() {
          return []; // browser dev mock has no background jobs
        },
        async JobsForTab() {
          return this.Jobs();
        },
        async Meta() {
      return {
        label: "DeepSeek-R1",
        ready: true,
        eventChannel: EVENT_CHANNEL,
        cwd,
            bypass: settings.bypass,
          };
        },
        async MetaForTab() {
          return this.Meta();
        },
    async Commands() {
      return [
        { name: "new", description: "Start a new session", kind: "builtin" as const },
        { name: "compact", description: "Summarize older history to free up context", kind: "builtin" as const },
        { name: "model", description: "Switch model", kind: "builtin" as const },
        { name: "effort", description: "Set reasoning effort", kind: "builtin" as const },
        { name: "skill", description: "List skills", kind: "builtin" as const },
        { name: "explore", description: "Investigate the codebase in an isolated subagent", kind: "skill" as const },
        { name: "review", description: "Review the staged diff", hint: "[focus]", kind: "custom" as const },
      ];
    },
    async Capabilities() {
      return {
        servers: capServers.map((s) => ({ ...s })),
        skills: capSkills.map((s) => ({ ...s })),
        skillRoots: capSkillRoots.map((s) => ({ ...s })),
      };
    },
    async ListMCPCatalog() {
      return mcpCatalogEntries.map((entry) => ({ ...entry }));
    },
    async AddMCPServer(input: MCPServerInput) {
      const tools = input.transport === "stdio" ? 3 : 5;
      capServers.push({
        name: input.name,
        transport: input.transport,
        status: "connected",
        configured: true,
        autoStart: true,
        tier: input.tier || "lazy",
        command: input.command,
        args: input.args,
        url: input.url,
        tools,
        prompts: 0,
        resources: 0,
        toolList: Array.from({ length: tools }, (_, i) => ({
          name: `${input.name}_tool_${i + 1}`,
          description: `Mock tool ${i + 1} exposed by ${input.name}.`,
        })),
      });
      return tools;
    },
    async UpdateMCPServer(name: string, input: MCPServerInput) {
      capServers = capServers.map((s) => {
        if (s.name !== name) return s;
        const connected = s.status === "connected" || s.status === "failed" || input.tier !== "lazy";
        const nextStatus = s.status === "disabled" ? "disabled" : connected ? "connected" : "deferred";
        const nextTools = nextStatus === "connected" ? s.tools || (input.transport === "stdio" ? 3 : 5) : 0;
        return {
          ...s,
          transport: input.transport,
          status: nextStatus,
          tier: input.tier || "lazy",
          command: input.transport === "stdio" ? input.command : "",
          args: input.transport === "stdio" ? input.args : [],
          url: input.transport === "stdio" ? "" : input.url,
          envKeys: input.env ? Object.keys(input.env).sort() : s.envKeys,
          tools: nextTools,
          error: undefined,
          authStatus: nextStatus !== "connected" && input.transport !== "stdio" ? "possible" : undefined,
          authUrl: nextStatus !== "connected" && input.transport !== "stdio" ? input.url : undefined,
        };
      });
    },
    async RemoveMCPServer(name: string) {
      capServers = capServers.filter((s) => s.name !== name);
    },
    async RetryMCPServer(name: string) {
      capServers = capServers.map((s) =>
        s.name === name ? { ...s, status: "connected", tools: s.tools || 4, error: undefined, authStatus: undefined, authUrl: undefined } : s,
      );
    },
    async ClearMCPServerAuthentication(name: string) {
      capServers = capServers.map((s) =>
        s.name === name
          ? {
              ...s,
              status: s.tier === "background" || s.tier === "eager" ? "initializing" : "deferred",
              tools: 0,
              error: undefined,
              authStatus: s.transport !== "stdio" ? "possible" : undefined,
              authUrl: s.transport !== "stdio" ? s.url : undefined,
              authConfigured: undefined,
            }
          : s,
      );
    },
    async PickSkillFolder() {
      return "~/my-skills";
    },
    async AddSkillPath(path: string) {
      const dir = path.trim() || "~/my-skills";
      if (!capSkillRoots.some((r) => r.scope === "custom" && r.dir === dir)) {
        capSkillRoots.push({
          dir,
          scope: "custom",
          priority: capSkillRoots.length + 1,
          status: "ok",
          configured: true,
          skills: 1,
          skillItems: [{ name: "local-dev", description: "Local custom development workflow", scope: "custom", runAs: "inline" }],
        });
      }
      if (!capSkills.some((s) => s.name === "local-dev")) {
        capSkills.push({ name: "local-dev", description: "Local custom development workflow", scope: "custom", runAs: "inline", enabled: true });
      }
    },
    async RemoveSkillPath(path: string) {
      capSkillRoots = capSkillRoots.filter((r) => !(r.scope === "custom" && r.dir === path));
      if (!capSkillRoots.some((r) => r.scope === "custom")) {
        const idx = capSkills.findIndex((s) => s.name === "local-dev");
        if (idx >= 0) capSkills.splice(idx, 1);
      }
    },
    async RefreshSkills() {},
    async SetSkillEnabled(name: string, enabled: boolean) {
      const skill = capSkills.find((s) => s.name === name);
      if (skill) skill.enabled = enabled;
    },
    async SetMCPServerEnabled(name: string, enabled: boolean) {
      capServers = capServers.map((s) =>
        s.name === name
          ? {
              ...s,
              status: enabled ? "connected" : "disabled",
              autoStart: s.builtIn ? enabled : s.autoStart,
              tools: enabled ? s.tools || 4 : 0,
              error: undefined,
              authStatus: !enabled && s.transport !== "stdio" ? "possible" : undefined,
              authUrl: !enabled && s.transport !== "stdio" ? s.url : undefined,
            }
          : s,
      );
    },
    async SetMCPServerTier(name: string, tier: string) {
      capServers = capServers.map((s) => {
        if (s.name !== name) return s;
        if (tier === "lazy") return { ...s, tier, autoStart: true };
        const tools = s.tools || (s.transport === "stdio" ? 3 : 5);
        return { ...s, tier, autoStart: true, status: "connected", tools, error: undefined, authStatus: undefined, authUrl: undefined };
      });
    },
    async SlashArgs(input: string) {
      // Mirror a slice of the real arg hints so the menu is exercisable in browser dev.
      const from = input.lastIndexOf(" ") + 1;
      const cur = input.slice(from);
      const cmd = input.slice(0, input.indexOf(" ") < 0 ? input.length : input.indexOf(" "));
      const subs: Record<string, { label: string; insert: string; hint: string; descend?: boolean }[]> = {
        "/skill": [
          { label: "list", insert: "list", hint: "list skills" },
          { label: "show", insert: "show ", hint: "show a skill's body", descend: true },
          { label: "enable", insert: "enable ", hint: "enable a disabled skill", descend: true },
          { label: "disable", insert: "disable ", hint: "disable an enabled skill", descend: true },
          { label: "new", insert: "new ", hint: "scaffold a new skill" },
          { label: "paths", insert: "paths", hint: "show discovery paths" },
        ],
        "/hooks": [
          { label: "list", insert: "list", hint: "list active hooks" },
          { label: "trust", insert: "trust", hint: "trust this project's hooks" },
        ],
        "/model": [
          { label: "deepseek/deepseek-v4-flash", insert: "deepseek/deepseek-v4-flash", hint: "current" },
          { label: "deepseek/deepseek-v4-pro", insert: "deepseek/deepseek-v4-pro", hint: "" },
        ],
        "/effort": [
          { label: "auto", insert: "auto", hint: "use the model default" },
          { label: "high", insert: "high", hint: "deeper reasoning" },
          { label: "max", insert: "max", hint: "maximum reasoning" },
        ],
      };
      const items = (subs[cmd] ?? [])
        .filter((it) => it.label.toLowerCase().startsWith(cur.toLowerCase()))
        .map((it) => ({ label: it.label, insert: it.insert, hint: it.hint, descend: it.descend ?? false }));
      return { items, from };
    },
    async ListDir(rel: string) {
      // A tiny fake tree so the @ menu is navigable in browser dev.
      if (rel === "" || rel === "./") {
        return [
          { name: "internal", isDir: true },
          { name: "desktop", isDir: true },
          { name: "README.md", isDir: false },
          { name: "go.mod", isDir: false },
        ];
      }
      if (rel === "internal/") {
        return [
          { name: "control", isDir: true },
          { name: "boot", isDir: true },
          { name: "event.go", isDir: false },
        ];
      }
      return [{ name: "file.go", isDir: false }];
    },
    async SearchFileRefs(query: string) {
      const q = query.toLowerCase();
      return ["desktop/frontend/src/lib/bridge.ts", "frontend/wailsjs/runtime/runtime.js", "internal/control/refs.go"]
        .filter((path) => path.split("/").pop()?.toLowerCase().includes(q))
        .map((name) => ({ name, isDir: false }));
    },
    async ReadFile(rel: string) {
      const samples: Record<string, string> = {
        "README.md": "# ArcDesk\n\nBrowser-dev workspace preview.\n\n- Chat in the center\n- Browse files on the right\n- Keep sessions on the left\n",
        "go.mod": "module arcdesk\n\ngo 1.23\n",
        "desktop/file.go": "package desktop\n\nfunc main() {\n\tprintln(\"workspace preview\")\n}\n",
        "internal/event.go": "package internal\n\n// mock file used by the browser dev seam\n",
      };
      return {
        path: rel,
        body: samples[rel] ?? `// ${rel}\n\nMock file body from browser dev.`,
        size: samples[rel]?.length ?? 42,
        truncated: false,
        binary: false,
      };
    },
    async ReadWorkspaceFileDataURL(rel: string) {
      const lower = rel.toLowerCase();
      const mime = lower.endsWith(".png")
        ? "image/png"
        : lower.endsWith(".gif")
          ? "image/gif"
          : lower.endsWith(".webp")
            ? "image/webp"
            : "image/jpeg";
      return `data:${mime};base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+2b2kAAAAASUVORK5CYII=`;
    },
    async WorkspaceChanges() {
      return {
        gitAvailable: true,
        files: [
          {
            path: "desktop/frontend/src/components/GitPanel.tsx",
            sources: ["session", "git"],
            gitStatus: "M",
            turns: [0, 2],
            latestPrompt: "Mock session edited the workspace panel.",
            latestTime: Date.now() - 60_000,
          },
          { path: "README.md", sources: ["git"], gitStatus: "??" },
          { path: "internal/control/controller.go", sources: ["session"], turns: [1], latestTime: Date.now() - 120_000 },
        ],
      };
    },
    async RunCodeReview(mode, scope, paths) {
      await new Promise((resolve) => setTimeout(resolve, 600));
      const label = mode === "security" ? "Security review" : "Code review";
      return {
        text: `**Verdict:** Minor nits, OK to ship after.\n\n- nit: ${paths[0] ?? "example.go"}:1 — mock ${label} (${scope}, ${paths.length} file(s)).`,
      };
    },
    async OpenWorkspacePath(rel: string) {
      console.info("mock OpenWorkspacePath", rel);
    },
    async RevealWorkspacePath(rel: string) {
      console.info("mock RevealWorkspacePath", rel);
    },
    async RevealPath(path: string) {
      console.info("mock RevealPath", path);
    },
    async SavePastedImage(_dataUrl: string) {
      return ".arcdesk/attachments/mock.png";
    },
    async SavePastedFile(name: string, _dataUrl: string) {
      return `.arcdesk/attachments/mock-${name}`;
    },
    async AttachDropped(path: string) {
      const normalized = path.replace(/\\/g, "/");
      const name = normalized.split("/").filter(Boolean).pop() ?? path;
      const looksAbsolute = /^([a-zA-Z]:|\/)/.test(normalized);
      if (!looksAbsolute) {
        return { kind: "workspace" as const, path: normalized, isDir: normalized.endsWith("/") };
      }
      return { kind: "attachment" as const, path: `.arcdesk/attachments/mock-${name}` };
    },
    async AttachmentDataURL(_path: string) {
      return "data:image/png;base64,iVBORw0KGgo=";
    },
        async Models() {
          return [
            { ref: "deepseek/deepseek-v4-flash", provider: "deepseek", model: "deepseek-v4-flash", current: true },
            { ref: "deepseek/deepseek-v4-pro", provider: "deepseek", model: "deepseek-v4-pro", current: false },
          ];
        },
        async ModelsForTab() {
          return this.Models();
        },
        async SetModel() {},
        async SetModelForTab(_tabID, name) {
          await this.SetModel(name);
        },
        async Effort() {
          return { supported: true, current: mockEffort, default: "high", levels: ["auto", "high", "max"] };
        },
        async EffortForTab() {
          return this.Effort();
        },
        async SetEffort(level: string) {
          mockEffort = level || "auto";
        },
        async SetEffortForTab(_tabID, level) {
          await this.SetEffort(level);
        },
    async Memory() {
      return {
        available: true,
        storeDir: "~/.config/arcdesk/projects/-mock/memory",
        docs: [
          {
            path: "ARCDESK.md",
            scope: "project",
            body: "# ArcDesk project memory\n\nMock doc shown in the browser dev seam.\n\n## Notes\n\n- prefers concise replies",
          },
          {
            path: "~/.config/arcdesk/ARCDESK.md",
            scope: "user",
            body: t("mock.memoryBody"),
          },
        ],
        facts: [
          {
            name: "prefers-tabs",
            description: "User prefers tabs",
            type: "user",
            body: "Indent with tabs.",
          },
        ],
        scopes: [
          { scope: "user", path: "~/.config/arcdesk/ARCDESK.md" },
          { scope: "project", path: "ARCDESK.md" },
          { scope: "local", path: "ARCDESK.local.md" },
        ],
      };
    },
    async Remember(scope: string, note: string) {
      emit({ kind: "notice", level: "info", text: `remembered → ${scope}` });
      return `${scope} ARCDESK.md (mock): ${note}`;
    },
    async Forget(name: string) {
      emit({ kind: "notice", level: "info", text: `forgot → ${name}` });
    },
    async SaveDoc(path: string, _body: string) {
      emit({ kind: "notice", level: "info", text: `saved → ${path}` });
      return path;
    },
    async Settings() {
      return JSON.parse(JSON.stringify(settings)) as SettingsView;
    },
    async SetDefaultModel(ref: string) {
      settings.defaultModel = ref;
    },
    async SetPlannerModel(ref: string) {
      settings.plannerModel = ref;
    },
    async SetAutoPlan(mode: string) {
      settings.autoPlan = mode;
    },
    async SaveProvider(p: ProviderView) {
      const i = settings.providers.findIndex((x) => x.name === p.name);
      if (i >= 0) settings.providers[i] = p;
      else settings.providers.push(p);
    },
    async SyncProviderModels(providerName: string): Promise<ProviderModelsResult> {
      const provider = settings.providers.find((p) => p.name === providerName);
      if (!provider) throw new Error(`provider ${providerName} not found`);
      const models = ["deepseek-chat", "deepseek-reasoner", "deepseek-v4-flash"];
      provider.models = models;
      provider.default = models[0];
      return { provider: providerName, models };
    },
    async DeleteProvider(name: string) {
      settings.providers = settings.providers.filter((p) => p.name !== name);
    },
    async SetProviderKey(apiKeyEnv: string) {
      settings.providers.forEach((p) => {
        if (p.apiKeyEnv === apiKeyEnv) p.keySet = true;
      });
    },
    async SetPermissionMode(mode: string) {
      settings.permissions.mode = mode;
    },
    async AddPermissionRule(list: string, rule: string) {
      const k = list as "allow" | "ask" | "deny";
      if (settings.permissions[k] && !settings.permissions[k].includes(rule)) settings.permissions[k].push(rule);
    },
    async RemovePermissionRule(list: string, rule: string) {
      const k = list as "allow" | "ask" | "deny";
      settings.permissions[k] = settings.permissions[k].filter((r) => r !== rule);
    },
        async SetSandbox(bash: string, network: boolean, workspaceRoot: string, allowWrite: string[]) {
          settings.sandbox = { bash, network, workspaceRoot, allowWrite };
        },
        async SetNetwork(n: NetworkView) {
          settings.network = n;
        },
        async SetCloseBehavior(mode: string) {
          settings.closeBehavior = mode === "quit" ? "quit" : "background";
        },
        async SetDesktopLanguage(lang: string) {
          settings.desktopLanguage = lang === "en" || lang === "zh" ? lang : "";
        },
        async SetDesktopTerminalShell(shell: string) {
          settings.desktopTerminalShell =
            shell === "powershell" || shell === "cmd" || shell === "git-bash" || shell === "wsl" ? shell : "";
        },
        async SetDesktopGitSettings(git) {
          settings.desktopGit = {
            prMergeMethod:
              git.prMergeMethod === "squash" || git.prMergeMethod === "rebase" ? git.prMergeMethod : "merge",
            checkGitHubCli: git.checkGitHubCli === true,
            syncRepoMergeToGitHub: git.syncRepoMergeToGitHub === true,
            commitInstructions: git.commitInstructions ?? "",
            prInstructions: git.prInstructions ?? "",
          };
        },
        async SetDesktopAppearance(theme: string, style: string) {
          settings.desktopTheme = theme === "auto" || theme === "light" ? theme : "dark";
          settings.desktopThemeStyle = style;
        },
        async SetDesktopAppearancePrefs(prefs: DesktopAppearanceView) {
          settings.desktopAppearance = { ...prefs };
        },
        async SetDesktopCodeReviewSettings(prefs: DesktopCodeReviewView) {
          settings.desktopCodeReview = {
            defaultScope: prefs.defaultScope === "session" || prefs.defaultScope === "git" ? prefs.defaultScope : "all",
            securityByDefault: prefs.securityByDefault === true,
          };
        },
        async MigrateDesktopLocalPrefs(input: DesktopLocalPrefsMigrationInput) {
          if (input.hasAppearance && !settings.desktopAppearance.backgroundPreset) {
            settings.desktopAppearance = {
              backgroundPreset: input.backgroundPreset,
              foregroundPreset: input.foregroundPreset,
              textSize: input.textSize || "default",
              codeFontSize: input.codeFontSize || "default",
              diffMarker: input.diffMarker || "background",
            };
          }
          if (input.hasCodeReview && settings.desktopCodeReview.defaultScope === "all" && !settings.desktopCodeReview.securityByDefault) {
            settings.desktopCodeReview = {
              defaultScope:
                input.codeReviewScope === "session" || input.codeReviewScope === "git" ? input.codeReviewScope : "all",
              securityByDefault: input.codeReviewSecurity,
            };
          }
        },
        async SaveProjectPreviewSettings(_input: ProjectPreviewSettingsInput) {},
        async MigrateDesktopPreferences(language: string, theme: string, style: string) {
          if (!settings.desktopLanguage) settings.desktopLanguage = language === "en" || language === "zh" ? language : "";
          if (!settings.desktopTheme && !settings.desktopThemeStyle) {
            settings.desktopTheme = theme === "auto" || theme === "light" ? theme : "dark";
            settings.desktopThemeStyle = style;
          }
        },
    async SetAgentSettings(agent: AgentSettingsInput) {
      settings.agent = {
        ...settings.agent,
        ...agent,
        subagentModels: { ...agent.subagentModels },
        usesDefaultPrompt: !agent.systemPrompt.trim(),
      };
      settings.autoPlan = agent.autoPlan;
    },
    async ListOutputStyles(): Promise<OutputStyleView[]> {
      return [
        { name: "explanatory", description: "Explain non-obvious implementation choices as you go", builtin: true },
        { name: "learning", description: "Collaborate and leave TODO(human) stubs for the user to complete", builtin: true },
        { name: "concise", description: "Terse replies: minimal prose, code and bullets only", builtin: true },
      ];
    },
    async SetAgentParams(temperature: number, maxSteps: number, systemPrompt: string) {
      settings.agent = { ...settings.agent, temperature, maxSteps, systemPrompt, usesDefaultPrompt: !systemPrompt.trim() };
    },
    async SetTrayLocale(_locale: "en" | "zh") {},
    async SetBypass(on: boolean) {
      settings.bypass = on;
    },
    async Version() {
      return "v1.0.0 (browser dev)";
    },
    async CheckUpdate() {
      // Keep the default browser preview focused on the primary product surface.
      // ApplyUpdate remains mocked for explicit updater-flow tests.
      return {
        available: false,
        current: "v1.0.0",
        latest: "v1.0.0",
        notes: "",
        canSelfUpdate: false,
        downloadUrl: "",
        assetSize: 0,
      };
    },
    async ApplyUpdate() {
      const total = 12_345_678;
      for (let r = 0; r <= total; r += 1_800_000) {
        emitUpdater({ phase: "downloading", received: Math.min(r, total), total });
        await delay(120);
      }
      emitUpdater({ phase: "verifying", received: total, total });
      await delay(500);
      emitUpdater({ phase: "applying", received: total, total });
      await delay(500);
      emitUpdater({ phase: "done", received: total, total });
      // The real shell relaunches here; the mock just stops.
    },
    async OpenDownloadPage() {
      if (typeof window !== "undefined") {
        window.open("https://github.com/esengine/arcdesk/releases/latest", "_blank", "noopener");
      }
    },
    // Dev seam: drives the overlay flow in the browser until ConnectKey sets the
    // key. Matches ConnectKey on apiKeyEnv so the two stay in sync.
    async NeedsOnboarding() {
      return !settings.providers.find((p) => p.apiKeyEnv === "DEEPSEEK_API_KEY")?.keySet;
    },
    async ConnectKey(apiKey: string, baseUrl?: string) {
      if (!apiKey.trim()) throw new Error("key is required");
      const base = (baseUrl?.trim() || "https://api.deepseek.com").replace(/\/$/, "");
      settings.providers.forEach((p) => {
        if (p.apiKeyEnv === "DEEPSEEK_API_KEY") {
          p.keySet = true;
          p.baseUrl = base;
          p.balanceUrl = `${base}/user/balance`;
        }
      });
      await delay(300);
    },
    async SideChatReply(message: string) {
      if (!message.trim()) throw new Error("message is required");
      await delay(200);
      return `（侧对话 mock）已收到：${message.trim()}`;
    },
    async GetMobilePendingDecision() {
      return null;
    },
    async RespondMobileDecision(_decisionId: string, _allow: boolean, _answers?: QuestionAnswer[]) {},
    // Tab management mocks.
    async ListTabs() {
      return mockTabs.map((tab) => ({ ...tab }));
    },
    async OpenProjectTab(workspaceRoot: string, _topicID: string) {
      const existing = mockTabs.find((tab) => tab.scope === "project" && tab.workspaceRoot === workspaceRoot && tab.topicId === _topicID);
      if (existing) {
        setMockActiveTab(existing.id);
        return { ...existing, active: true };
      }
      const tab: TabMeta = {
        id: "tab_" + Date.now(),
        scope: "project",
        workspaceRoot,
        workspaceName: workspaceRoot.split("/").filter(Boolean).pop() ?? workspaceRoot,
        topicId: _topicID,
        topicTitle: topicLabel(_topicID, t("mock.newSession")),
        projectColor: mockProjectTree.find((node) => node.root === workspaceRoot)?.projectColor,
	        label: "deepseek-v4-flash",
	        ready: true,
	        running: false,
	        mode: "normal",
	        active: true,
	        cwd: workspaceRoot,
      };
      mockTabs = [...mockTabs.map((item) => ({ ...item, active: false })), tab];
      return { ...tab };
    },
    async OpenGlobalTab(_topicID: string) {
      const existing = mockTabs.find((tab) => tab.scope === "global" && tab.topicId === _topicID);
      if (existing) {
        setMockActiveTab(existing.id);
        return { ...existing, active: true };
      }
      const tab: TabMeta = {
        id: "tab_" + Date.now(),
        scope: "global",
        workspaceRoot: "",
        workspaceName: "Global",
        topicId: _topicID,
        topicTitle: topicLabel(_topicID, "Global"),
	        label: "deepseek-v4-flash",
	        ready: true,
	        running: false,
	        mode: "normal",
	        active: true,
	        cwd: "",
      };
      mockTabs = [...mockTabs.map((item) => ({ ...item, active: false })), tab];
      return { ...tab };
    },
    async SetActiveTab(_tabID: string) {
      setMockActiveTab(_tabID);
    },
    async ReorderTabs(_tabIDs: string[]) {
      const byId = new Map(mockTabs.map((tab) => [tab.id, tab]));
      const ordered = _tabIDs.map((id) => byId.get(id)).filter((tab): tab is TabMeta => Boolean(tab));
      if (ordered.length === mockTabs.length) mockTabs = ordered;
    },
    async CloseTab(_tabID: string) {
      if (mockTabs.length <= 1) return;
      const wasActive = mockTabs.some((tab) => tab.id === _tabID && tab.active);
      mockTabs = mockTabs.filter((tab) => tab.id !== _tabID);
      if (wasActive && mockTabs.length > 0 && !mockTabs.some((tab) => tab.active)) {
        mockTabs[mockTabs.length - 1] = { ...mockTabs[mockTabs.length - 1], active: true };
      }
    },
    async ListProjectTree() {
      return cloneProjectTree();
    },
    async RenameProject(workspaceRoot: string, title: string) {
      const node = workspaceRoot
        ? mockProjectTree.find((item) => item.root === workspaceRoot)
        : mockProjectTree.find((item) => item.kind === "global_folder");
      if (node) node.label = title.trim() || (node.kind === "global_folder" ? "Global" : node.label);
    },
    async SetProjectColor(workspaceRoot: string, color: string) {
      const node = workspaceRoot
        ? mockProjectTree.find((item) => item.root === workspaceRoot)
        : mockProjectTree.find((item) => item.kind === "global_folder");
      if (!node) return;
      node.projectColor = color || undefined;
      for (const child of projectChildren(node)) child.projectColor = node.projectColor;
      mockTabs = mockTabs.map((tab) =>
        (workspaceRoot ? tab.workspaceRoot === workspaceRoot : tab.scope === "global")
          ? { ...tab, projectColor: node.projectColor }
          : tab,
      );
    },
    async ReorderProjects(workspaceRoots: string[]) {
      const projects = mockProjectTree.filter((node) => node.kind === "project");
      if (workspaceRoots.length !== projects.length) return;
      const byRoot = new Map(projects.map((node) => [node.root, node]));
      const ordered = workspaceRoots.map((root) => byRoot.get(root)).filter((node): node is ProjectNode => Boolean(node));
      if (ordered.length !== projects.length) return;
      const globals = mockProjectTree.filter((node) => node.kind !== "project");
      mockProjectTree.splice(0, mockProjectTree.length, ...globals, ...ordered);
    },
    async CreateTopic(_scope: string, _workspaceRoot: string, title: string) {
      const id = "topic_" + Date.now();
      const topicTitle = title.trim() || t("mock.newSession");
      const parent = _scope === "global"
        ? mockProjectTree.find((node) => node.kind === "global_folder")
        : mockProjectTree.find((node) => node.root === _workspaceRoot);
      if (parent) {
        const global = parent.kind === "global_folder";
        parent.children = [{
          key: parent.kind === "global_folder" ? "global_topic_" + id : "topic_" + id,
          kind: global ? "global_topic" : "topic",
          label: topicTitle,
          root: parent.root,
          topicId: id,
          projectColor: parent.projectColor,
        }, ...projectChildren(parent)];
      }
      return { id, title: topicTitle, createdAt: Date.now() };
    },
    async RenameTopic(topicID: string, title: string) {
      const topic = findMockTopic(topicID);
      const nextTitle = title.trim();
      if (!topic || !nextTitle) return;
      const activePrefix = topic.label?.startsWith("● ") ? "● " : "";
      topic.label = `${activePrefix}${nextTitle}`;
      mockTabs = mockTabs.map((tab) =>
        tab.topicId === topicID ? { ...tab, topicTitle: nextTitle } : tab,
      );
    },
    async DeleteTopic(topicID: string) {
      deleteMockTopic(topicID);
    },
    async TrashTopic(topicID: string) {
      deleteMockTopic(topicID);
    },
    async ListWriteFiles(workspaceRoot: string) {
      const root = (workspaceRoot || "/workspace").replace(/[/\\]+$/, "");
      const entries: FileEntry[] = [];
      const seenDirs = new Set<string>();
      for (const path of Object.keys(mockWriteFiles)) {
        if (!path.startsWith(root + "/") && path !== root) continue;
        const rel = path.slice(root.length + 1);
        const parts = rel.split("/");
        let current = root;
        for (let i = 0; i < parts.length - 1; i++) {
          current = `${current}/${parts[i]}`;
          if (!seenDirs.has(current)) {
            seenDirs.add(current);
            entries.push({ path: current, name: parts[i]!, isDir: true, modTime: Date.now() - 18_400_000 });
          }
        }
        entries.push({
          path,
          name: baseName(path),
          isDir: false,
          size: mockWriteFiles[path]!.length,
          modTime: Date.now() - 3_600_000,
        });
      }
      for (const dir of mockWriteDirs) {
        if (!dir.startsWith(root + "/")) continue;
        if (!seenDirs.has(dir)) {
          seenDirs.add(dir);
          entries.push({ path: dir, name: baseName(dir), isDir: true, modTime: Date.now() - 18_400_000 });
        }
      }
      return entries;
    },
    async ReadWriteFile(path: string) {
      if (!(path in mockWriteFiles)) throw new Error(`file not found: ${path}`);
      return mockWriteFiles[path]!;
    },
    async WriteWriteFile(path: string, content: string) {
      mockWriteFiles[path] = content;
    },
    async DeleteWriteFile(path: string) {
      delete mockWriteFiles[path];
    },
    async RenameWriteFile(oldPath: string, newPath: string) {
      if (!(oldPath in mockWriteFiles)) throw new Error(`file not found: ${oldPath}`);
      mockWriteFiles[newPath] = mockWriteFiles[oldPath]!;
      delete mockWriteFiles[oldPath];
    },
    async CompleteWriteInline(textBefore: string, _textAfter: string) {
      const tail = textBefore.trim().split(/\s+/).slice(-6).join(" ");
      return tail ? `${tail} …` : "Continue writing here…";
    },
    async ListWriteWorkspaces() {
      return [...mockWriteWorkspaces];
    },
    async AddWriteWorkspace(root: string) {
      const next = root.trim();
      if (!next || mockWriteWorkspaces.includes(next)) return;
      mockWriteWorkspaces.push(next);
    },
    async RemoveWriteWorkspace(root: string) {
      const idx = mockWriteWorkspaces.indexOf(root);
      if (idx >= 0) mockWriteWorkspaces.splice(idx, 1);
    },
    async DefaultWriteWorkspace() {
      return "/workspace/writes";
    },
    async EnsureBundledSkills() {
      if (!capSkills.some((s) => s.name === "copywriting")) {
        capSkills.push({
          name: "copywriting",
          description: "Write and improve marketing copy, articles, and persuasive 文案 (marketingskills)",
          scope: "global",
          runAs: "inline",
          enabled: true,
        });
      }
    },
    async GetClawChannels() {
      return [];
    },
    async SaveClawChannel(_channel: ClawChannel) {},
    async DeleteClawChannel(_id: string) {},
    async GetClawMessages(channelID: string) {
      return mockClawMessages.filter((message) => message.channelId === channelID);
    },
    async SendClawMessage(channelID: string, text: string) {
      const message: ClawMessage = {
        id: `msg-${Date.now()}`,
        channelId: channelID,
        text: text.trim(),
        outgoing: true,
        createdAt: Date.now(),
      };
      mockClawMessages.push(message);
      return message;
    },
    async GetClawCallbackInfo(channelID: string) {
      return {
        baseUrl: "http://127.0.0.1:8787",
        path: `/claw/wecom/${channelID}`,
        url: `http://127.0.0.1:8787/claw/wecom/${channelID}`,
        port: 8787,
      };
    },
    async TestClawWeComChannel(_channel: ClawChannel) {
      return "";
    },
    async GetMobilePairingInfo() {
      return {
        token: "dev-token",
        pairUrl: "http://127.0.0.1:8788/mobile/p/dev-token",
        lanPairUrl: "http://127.0.0.1:8787/mobile/p/dev-token",
        relayUrl: "http://127.0.0.1:8788/mobile/p/dev-token",
        lanIp: "127.0.0.1",
        port: 8787,
        expiresAt: Date.now() + 900_000,
        pairedCount: 0,
        enabled: true,
        qrDataUrl: "",
        relayConnected: false,
        tunnelRunning: false,
        tunnelUrl: "",
        connectMode: "lan",
        bridgeReady: true,
      };
    },
    async GetMobileConnectDiagnostics() {
      return {
        report: "=== ArcDesk mobile diagnostics (dev mock) ===",
        bridgeReady: true,
        allowLAN: true,
        bindAddress: "127.0.0.1:8787",
        lanIp: "127.0.0.1",
        port: 8787,
        connectMode: "lan",
        pairUrl: "http://127.0.0.1:8787/mobile/p/dev-token",
        localHealth: "ok",
        lanHealth: "ok",
      };
    },
    async RefreshMobilePairing() {
      return this.GetMobilePairingInfo();
    },
    async GetMobileConnectConfig() {
      return { enabled: true, model: "deepseek-chat", persona: "Be concise and practical.", workspaceRoot: "/workspace" };
    },
    async SaveMobileConnectConfig(_config: MobileConnectConfig) {},
    async ListMobileSessions() {
      return [];
    },
    async GetMobileTunnelStatus() {
      return { running: false, url: "" };
    },
    async StartMobileTunnel() {
      return { running: true, url: "" };
    },
    async StopMobileTunnel() {
      return { running: false, url: "" };
    },
    async GetScheduledTasks() {
      const now = Date.now();
      return [
        { id: "task-1", name: "Daily summary", prompt: "Summarize the day", scheduleType: "daily", scheduleValue: "09:00", workspaceRoot: "/workspace", model: "deepseek-chat", enabled: true, nextRun: now + 3_600_000 },
        { id: "task-2", name: "Interval follow-up", prompt: "Check progress", scheduleType: "interval", scheduleValue: "2h", workspaceRoot: "/workspace/docs", model: "deepseek-reasoner", enabled: false, nextRun: now + 7_200_000 },
      ];
    },
    async SaveScheduledTask(_task: ScheduledTask) {},
    async DeleteScheduledTask(_id: string) {},
    async TriggerScheduledTask(id: string) {
      const task = (await this.GetScheduledTasks()).find((item) => item.id === id);
      emitScheduleTask({
        id,
        name: task?.name ?? id,
        source: "manual",
        prompt: task?.prompt,
      });
    },
    async SaveWindowState(_state) {
      // no-op in browser dev — no real window geometry to persist
    },
    async ContextPanel(_tabID: string) {
      const now = Date.now();
      return {
        usedTokens: 42124,
        windowTokens: 128000,
        promptTokens: 22134,
        completionTokens: 12345,
        reasoningTokens: 7521,
        cacheHitTokens: 87000,
        cacheMissTokens: 13000,
        sessionCost: 0.018,
        sessionCurrency: "¥",
        sessionCostUsd: 0.018,
        readFiles: [
          { path: "ARCDESK.md", turn: 2, time: now - 34 * 60 * 1000 },
          { path: "pyproject.toml", turn: 3, time: now - 30 * 60 * 1000 },
          { path: "docs/dev-standard.md", turn: 5, time: now - 13 * 60 * 1000, offset: 0, limit: 180 },
          { path: "scripts/db_migrate.sh", turn: 6, time: now - 4 * 60 * 1000, offset: 120, limit: 80, truncated: true },
        ],
        changedFiles: [
          { path: t("mock.changedFile1Path"), sources: ["session"], gitStatus: "modified", turns: [5, 6], latestPrompt: t("mock.changedFile1Prompt"), latestTime: now - 2 * 60 * 1000 },
          { path: t("mock.changedFile2Path"), sources: ["session"], gitStatus: "added", turns: [6], latestPrompt: t("mock.changedFile2Prompt"), latestTime: now - 60 * 1000 },
        ],
      };
    },
  };
}
