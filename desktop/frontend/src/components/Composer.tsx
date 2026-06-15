import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties, ClipboardEvent, DragEvent, KeyboardEvent, PointerEvent as ReactPointerEvent, ReactNode } from "react";
import {
  AlertTriangle,
  ArrowUp,
  Check,
  ChevronDown,
  Eye,
  FileText,
  Folder,
  FolderGit2,
  FolderX,
  FolderPlus,
  Globe,
  List,
  Monitor,
  Square,
  SquareTerminal,
  TextQuote,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import { NO_WORKSPACE_VALUE } from "../lib/composerWorkspace";
import { asArray } from "../lib/array";
import { app, onFilesDropped } from "../lib/bridge";
import { logBridgeError } from "../lib/logBridgeError";
import { useI18n } from "../lib/i18n";
import { clearLayoutSize, loadOptionalLayoutSize, saveLayoutSize } from "../lib/layoutPreferences";
import { getRecentWorkspacePaths, recordRecentWorkspace, removeRecentWorkspace } from "../lib/workspaceRecents";
import type { CommandInfo, ComposerInsertRequest, ComposerWriteContext, DirEntry, EffortInfo, Mode, SlashArgItem, SlashArgsResult, WorkspaceView } from "../lib/types";
import {
  formatWorkspaceReference,
  hasComposerFileDrag,
  parseWorkspaceReference,
  readDroppedFileUriPaths,
  readWorkspaceReferenceDrag,
} from "../lib/workspaceDrag";
import { basename, parentPath } from "../lib/workspaceFilePreview";
import { SlashMenu } from "./SlashMenu";
import { ArgMenu } from "./ArgMenu";
import { FileMenu } from "./FileMenu";
import { EffortSwitcherMenu, EffortSwitcherTrigger } from "./EffortSwitcher";
import { ComposerModeBar } from "./ComposerModeBar";
import { ModelSwitcherMenu, ModelSwitcherTrigger } from "./ModelSwitcher";
import { modelLabelFromRef } from "../lib/modelLabel";
import { Tooltip } from "./Tooltip";
import { useDismissOverlay } from "../lib/useDismissOverlay";
import { AnchoredPopover } from "./AnchoredPopover";

export type ComposerSessionTag = { id: string; label: string };

function ComposerSessionChip({
  kind,
  icon,
  label,
  onOpen,
  onClose,
  closeLabel,
}: {
  kind: "terminal" | "browser" | "page";
  icon: ReactNode;
  label: string;
  onOpen?: () => void;
  onClose?: () => void;
  closeLabel: string;
}) {
  return (
    <div className={`composer-context__item composer-context__item--${kind} composer-context__session`}>
      {onOpen ? (
        <button type="button" className="composer-context__session-main" onClick={onOpen}>
          {icon}
          <span className="composer-context__text">
            <span className="composer-context__name">{label}</span>
          </span>
        </button>
      ) : (
        <span className="composer-context__session-main">
          {icon}
          <span className="composer-context__text">
            <span className="composer-context__name">{label}</span>
          </span>
        </span>
      )}
      {onClose ? (
        <button
          type="button"
          className="composer-context__session-close"
          onClick={(event) => {
            event.stopPropagation();
            onClose();
          }}
          aria-label={closeLabel}
        >
          <X size={12} />
        </button>
      ) : null}
    </div>
  );
}

function ComposerSessionGroup({
  kind,
  icon,
  sessions,
  groupLabel,
  activeFallback,
  closeTabLabel,
  onOpen,
  onClose,
}: {
  kind: "terminal" | "browser";
  icon: ReactNode;
  sessions: ComposerSessionTag[];
  groupLabel: (count: number) => string;
  activeFallback: string;
  closeTabLabel: (title: string) => string;
  onOpen?: (id: string) => void;
  onClose?: (id: string) => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  useDismissOverlay(menuOpen, () => setMenuOpen(false), { excludeRefs: [rootRef], closeOnEscape: true });

  if (sessions.length === 0) return null;

  if (sessions.length === 1) {
    const session = sessions[0]!;
    return (
      <ComposerSessionChip
        kind={kind}
        icon={icon}
        label={session.label.trim() || activeFallback}
        onOpen={onOpen ? () => onOpen(session.id) : undefined}
        onClose={onClose ? () => onClose(session.id) : undefined}
        closeLabel={closeTabLabel(session.label)}
      />
    );
  }

  return (
    <div className={`composer-context__group composer-context__group--${kind}`} ref={rootRef}>
      <button
        type="button"
        className={`composer-context__item composer-context__item--${kind} composer-context__group-toggle`}
        aria-expanded={menuOpen}
        onClick={() => setMenuOpen((open) => !open)}
      >
        {icon}
        <span className="composer-context__text">
          <span className="composer-context__name">{groupLabel(sessions.length)}</span>
        </span>
        <ChevronDown size={12} className={`composer-context__group-chevron${menuOpen ? " composer-context__group-chevron--open" : ""}`} />
      </button>
      {menuOpen ? (
        <div className="composer-context__group-menu" role="menu">
          {sessions.map((session) => (
            <div key={session.id} className="composer-context__group-row" role="presentation">
              <button
                type="button"
                className="composer-context__group-row-main"
                role="menuitem"
                onClick={() => {
                  onOpen?.(session.id);
                  setMenuOpen(false);
                }}
              >
                {icon}
                <span className="composer-context__name">{session.label.trim() || activeFallback}</span>
              </button>
              {onClose ? (
                <button
                  type="button"
                  className="composer-context__group-row-close"
                  aria-label={closeTabLabel(session.label)}
                  onClick={(event) => {
                    event.stopPropagation();
                    onClose(session.id);
                  }}
                >
                  <X size={12} />
                </button>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

interface Attachment {
  path: string;
  previewUrl?: string;
}

interface WorkspaceReference {
  path: string;
  isDir?: boolean;
}

const LONG_PASTE_MIN_CHARS = 2000;
const LONG_PASTE_MIN_LINES = 20;
const COMPOSER_MIN_HEIGHT = 86;
const COMPOSER_MAX_HEIGHT = 360;
const COMPOSER_MAX_VIEWPORT_RATIO = 0.4;
// Grace after compositionend to swallow a confirm-Enter that lands just after
// it; the real gap is a few ms, so keep it short or a deliberate quick second
// Enter (submit) gets eaten too.
const IME_CONFIRM_GRACE_MS = 100;

type PastedBlock = {
  label: string;
  text: string;
};

function lineCount(s: string): number {
  if (s === "") return 0;
  return s.split(/\r\n|\r|\n/).length;
}

function shouldFoldPaste(s: string): boolean {
  return s.length >= LONG_PASTE_MIN_CHARS || lineCount(s) >= LONG_PASTE_MIN_LINES;
}

function renderPastedBlock(block: PastedBlock): string {
  return `${block.label}\n\n--- Begin ${block.label} ---\n${block.text}\n--- End ${block.label} ---`;
}

function workspaceReferenceKey(ref: WorkspaceReference): string {
  return `${ref.isDir ? "dir" : "file"}:${ref.path}`;
}

function composerMaxHeight(): number {
  if (typeof window === "undefined") return COMPOSER_MAX_HEIGHT;
  return Math.max(COMPOSER_MIN_HEIGHT, Math.min(COMPOSER_MAX_HEIGHT, Math.floor(window.innerHeight * COMPOSER_MAX_VIEWPORT_RATIO)));
}

function clampComposerHeight(height: number): number {
  return Math.min(Math.max(Math.round(height), COMPOSER_MIN_HEIGHT), composerMaxHeight());
}

export interface ComposerSendState {
  disabled: boolean;
  onSend: () => void;
}

function loadComposerHeight(): number | null {
  return loadOptionalLayoutSize("composerHeight", clampComposerHeight);
}

function isImeKeyEvent(
  e: KeyboardEvent<HTMLTextAreaElement>,
  composing: boolean,
  lastCompositionEndAt: number,
): boolean {
  const native = e.nativeEvent as globalThis.KeyboardEvent & {
    isComposing?: boolean;
    keyCode?: number;
  };
  return (
    composing ||
    native.isComposing === true ||
    native.keyCode === 229 ||
    Date.now() - lastCompositionEndAt < IME_CONFIRM_GRACE_MS
  );
}

export type ComposerSurface = "code" | "write";

export function Composer({
  running,
  mode,
  cwd,
  modelLabel,
  tabId,
  effort,
  onSend,
  onLocalSlash,
  onCancel,
  onCycleMode,
  onSetMode,
  onSwitchModel,
  onSetEffort,
  onPickFolder,
  onRemoveWorkspace,
  insertRequest,
  disabled,
  decisionPending = false,
  ready,
  retry,
  workspaceRefreshSignal,
  hideModeBar = false,
  showWorkspaceSwitcher = false,
  workspaceNone = false,
  onUseNoWorkspace,
  terminalSessions = [],
  browserSessions = [],
  pagePreviewActive = false,
  pagePreviewLabel,
  onTerminalSessionOpen,
  onTerminalSessionClose,
  onBrowserSessionOpen,
  onBrowserSessionClose,
  onPageSessionOpen,
  onPageSessionClose,
  composerSurface = "code",
  sendExternally = false,
  onSendState,
}: {
  running: boolean;
  mode: Mode;
  cwd?: string;
  modelLabel: string;
  tabId?: string;
  effort?: EffortInfo;
  onSend: (displayText: string, submitText?: string) => void;
  /** Desktop-only slash verb that opens the memory drawer instead of submitting. */
  onLocalSlash?: (name: "memory") => void;
  // Returns the un-sent text when cancelling before the server replied (so it can
  // be restored to the input); undefined for a normal cancel.
  onCancel: () => string | undefined;
  onCycleMode: () => void;
  onSetMode: (mode: Mode) => void;
  onSwitchModel: (name: string) => void;
  onSetEffort: (level: string) => void;
  onPickFolder: (path?: string) => Promise<string>;
  onRemoveWorkspace: (path: string) => Promise<void>;
  insertRequest?: ComposerInsertRequest | null;
  disabled?: boolean;
  decisionPending?: boolean;
  // ready/cwd re-trigger the command fetch: Commands() returns only built-ins
  // until boot.Build finishes (the controller, hence skills/custom/MCP, is nil
  // before then), and the available set changes when the workspace switches.
  ready?: boolean;
  turnStartAt?: number;
  turnTokens?: number;
  retry?: { attempt: number; max: number };
  workspaceRefreshSignal?: number;
  hideModeBar?: boolean;
  showWorkspaceSwitcher?: boolean;
  workspaceNone?: boolean;
  onUseNoWorkspace?: () => void;
  terminalSessions?: ComposerSessionTag[];
  browserSessions?: ComposerSessionTag[];
  pagePreviewActive?: boolean;
  pagePreviewLabel?: string;
  onTerminalSessionOpen?: (id: string) => void;
  onTerminalSessionClose?: (id: string) => void;
  onBrowserSessionOpen?: (id: string) => void;
  onBrowserSessionClose?: (id: string) => void;
  onPageSessionOpen?: () => void;
  onPageSessionClose?: () => void;
  composerSurface?: ComposerSurface;
  sendExternally?: boolean;
  onSendState?: (state: ComposerSendState | null) => void;
}) {
  const { t } = useI18n();
  const [text, setText] = useState("");
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [workspaceRefs, setWorkspaceRefs] = useState<WorkspaceReference[]>([]);
  const [writeContext, setWriteContext] = useState<ComposerWriteContext | null>(null);
  const [pastedBlocks, setPastedBlocks] = useState<PastedBlock[]>([]);
  const [openPastedLabels, setOpenPastedLabels] = useState<string[]>([]);
  const [pendingPaste, setPendingPaste] = useState(0);
  const pastedBlocksRef = useRef<PastedBlock[]>([]);
  const nextPasteId = useRef(1);
  const [active, setActive] = useState(0);
  const [dismissed, setDismissed] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const dragDepthRef = useRef(0);
  const nativeDropAtRef = useRef(0);
  const attachDroppedPathsRef = useRef<(paths: string[]) => Promise<void>>(async () => {});
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const [workspaces, setWorkspaces] = useState<WorkspaceView[]>([]);
  const [composerHeight, setComposerHeight] = useState<number | null>(loadComposerHeight);
  const [composerResizing, setComposerResizing] = useState(false);
  const [paramMenu, setParamMenu] = useState<"model" | "effort" | null>(null);
  const taRef = useRef<HTMLTextAreaElement>(null);
  const composerCardRef = useRef<HTMLDivElement>(null);
  const workspaceAnchorRef = useRef<HTMLDivElement>(null);
  const modelAnchorRef = useRef<HTMLDivElement>(null);
  const effortAnchorRef = useRef<HTMLDivElement>(null);
  const wasRunning = useRef(running);
  const composingRef = useRef(false);
  const lastCompositionEndAt = useRef(0);
  const lastSelectionRef = useRef({ start: 0, end: 0 });
  const consumedInsertIdRef = useRef(0);
  const writeSurface = composerSurface === "write";
  const codeSurface = composerSurface === "code";
  const activeWriteContext = writeSurface ? writeContext : null;
  const showTerminalTags = codeSurface && terminalSessions.length > 0;
  const showBrowserTags = codeSurface && browserSessions.length > 0;
  const showPageTag = codeSurface && pagePreviewActive;
  const showCodeAttachments = codeSurface && attachments.length > 0;
  const showCodeWorkspaceRefs = codeSurface && workspaceRefs.length > 0;
  const hasContextTags =
    showTerminalTags || showBrowserTags || showPageTag || activeWriteContext || showCodeAttachments || showCodeWorkspaceRefs;

  useEffect(() => {
    if (writeSurface) {
      setWorkspaceRefs([]);
    } else {
      setWriteContext(null);
    }
  }, [composerSurface, writeSurface]);

  useEffect(() => {
    if (wasRunning.current && !running && text.trim() === "") {
      pastedBlocksRef.current = [];
      setPastedBlocks([]);
      setOpenPastedLabels([]);
    }
    wasRunning.current = running;
  }, [running, text]);

  // --- slash commands (whole-input "/token") ---
  const [commands, setCommands] = useState<CommandInfo[]>([]);
  useEffect(() => {
    app.Commands().then((next) => setCommands(asArray(next))).catch((err) => logBridgeError("Commands", err));
  }, [ready, cwd]);

  const slashQuery = useMemo(() => {
    if (!text.startsWith("/") || /\s/.test(text)) return null;
    return text.slice(1).toLowerCase();
  }, [text]);
  const slashMatches = useMemo(
    () => (slashQuery === null ? [] : commands.filter((c) => c.name.toLowerCase().includes(slashQuery)).slice(0, 8)),
    [slashQuery, commands],
  );

  // --- slash argument completion ("/cmd <args>") --- mirrors the CLI: once past
  // the command word, the backend suggests sub-commands (/skill → list/show/…,
  // /mcp → add/remove, /model → refs). Fetched from app.SlashArgs. Debounced
  // by 120ms so rapid typing doesn't flood the backend with IPC calls — the
  // menu only updates after the user pauses.
  const [argRes, setArgRes] = useState<SlashArgsResult | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();
  useEffect(() => {
    if (!text.startsWith("/") || !/\s/.test(text)) {
      setArgRes(null);
      return;
    }
    let live = true;
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      app
        .SlashArgs(text)
        .then((r) => {
          if (!live) return;
          // Drop suggestions that wouldn't change the input — the token is already
          // fully typed (e.g. "/skill list" offering "list"). Otherwise the menu
          // lingers on a complete command and Enter keeps "accepting" a no-op
          // instead of sending. (Defense-in-depth: the backend filters these too.)
          // r.items can arrive as null (an empty Go slice serializes to JSON null),
          // so guard before filtering — otherwise the throw is swallowed and the
          // stale menu from the previous keystroke lingers (the /skill list bug).
          const items = asArray(r?.items);
          const from = r?.from ?? 0;
          const useful = items.filter((it) => text.slice(0, from) + it.insert !== text);
          setArgRes(useful.length > 0 ? { items: useful, from } : null);
          setActive(0);
        })
        .catch((err) => logBridgeError("SlashArgs", err));
    }, 120);
    return () => {
      live = false;
      clearTimeout(debounceRef.current);
    };
  }, [text]);

  // --- @ file references (token at the end of the text) ---
  // atRaw is everything after a trailing "@token"; atDir is its path up to the
  // last "/", atFrag the part after. The menu lists one directory level (atDir)
  // and filters by atFrag — descending one level per pick.
  const atRaw = useMemo(() => {
    const m = /(?:^|\s)@([^\s]*)$/.exec(text);
    return m ? m[1] : null;
  }, [text]);
  const atDir = useMemo(() => {
    if (atRaw === null) return "";
    const slash = atRaw.lastIndexOf("/");
    return slash >= 0 ? atRaw.slice(0, slash + 1) : "";
  }, [atRaw]);
  const atFrag = useMemo(() => {
    if (atRaw === null) return "";
    const slash = atRaw.lastIndexOf("/");
    return (slash >= 0 ? atRaw.slice(slash + 1) : atRaw).toLowerCase();
  }, [atRaw]);

  const [entries, setEntries] = useState<DirEntry[]>([]);
  const [searchEntries, setSearchEntries] = useState<DirEntry[]>([]);
  const dirCache = useRef<Record<string, DirEntry[]>>({});
  const searchCache = useRef<Record<string, DirEntry[]>>({});
  useEffect(() => {
    if (atRaw === null) return;
    const cached = dirCache.current[atDir];
    if (cached) {
      setEntries(cached);
      return;
    }
    let live = true;
    app
      .ListDir(atDir)
      .then((es) => {
        const list = asArray(es);
        dirCache.current[atDir] = list;
        if (live) setEntries(list);
      })
      .catch((err) => logBridgeError("ListDir", err));
    return () => {
      live = false;
    };
    // re-fetch only when the menu opens or the directory level changes
  }, [atRaw === null, atDir]);
  useEffect(() => {
    if (atRaw === null || atDir !== "" || atFrag === "") {
      setSearchEntries([]);
      return;
    }
    const cached = searchCache.current[atFrag];
    if (cached) {
      setSearchEntries(cached);
      return;
    }
    setSearchEntries([]);
    let live = true;
    app
      .SearchFileRefs(atFrag)
      .then((es) => {
        const list = es ?? [];
        searchCache.current[atFrag] = list;
        if (live) setSearchEntries(list);
      })
      .catch((err) => logBridgeError("SearchFileRefs", err));
    return () => {
      live = false;
    };
  }, [atRaw === null, atDir, atFrag]);
  const atMatches = useMemo(
    () => {
      if (atRaw === null) return [];
      if (workspaceNone || !cwd) {
        return [{ name: t("composer.browseComputer"), isDir: false }];
      }
      const local = entries.filter((e) => e.name.toLowerCase().includes(atFrag));
      const seen = new Set(local.map((e) => e.name));
      const searched = searchEntries.filter((e) => {
        const basename = e.name.split("/").pop()?.toLowerCase() ?? "";
        return basename.includes(atFrag) && !seen.has(e.name);
      });
      return [...local, ...searched].slice(0, 10);
    },
    [atRaw, atFrag, cwd, entries, searchEntries, t, workspaceNone],
  );

  // --- which menu (if any) is open --- (slash command names win; then slash
  // arguments; then @-refs — they're rarely valid at once)
  const menuMode: "slash" | "slasharg" | "at" | null =
    slashMatches.length > 0 && !dismissed
      ? "slash"
      : argRes && argRes.items.length > 0 && !dismissed
        ? "slasharg"
        : atMatches.length > 0 && !dismissed
          ? "at"
          : null;
  const count =
    menuMode === "slash"
      ? slashMatches.length
      : menuMode === "slasharg"
        ? argRes!.items.length
        : menuMode === "at"
          ? atMatches.length
          : 0;

  useDismissOverlay(Boolean(menuMode), () => setDismissed(true), {
    excludeRefs: [taRef],
    excludeSelector: ".slashmenu",
  });

  // Reset highlight + un-dismiss whenever the active query changes.
  useEffect(() => {
    setActive(0);
    setDismissed(false);
  }, [slashQuery, atRaw]);

  const setTextCaretEnd = (next: string) => {
    setText(next);
    requestAnimationFrame(() => {
      const ta = taRef.current;
      if (ta) {
        ta.focus();
        ta.selectionStart = ta.selectionEnd = next.length;
      }
    });
  };

  const rememberCaret = () => {
    const ta = taRef.current;
    if (!ta) return;
    lastSelectionRef.current = { start: ta.selectionStart ?? text.length, end: ta.selectionEnd ?? text.length };
  };

  const insertTextAtCaret = (snippet: string) => {
    const ta = taRef.current;
    const start = ta ? (ta.selectionStart ?? text.length) : Math.min(lastSelectionRef.current.start, text.length);
    const end = ta ? (ta.selectionEnd ?? start) : Math.min(lastSelectionRef.current.end, text.length);
    const before = text.slice(0, start);
    const after = text.slice(end);
    const leading = before.length === 0 || before.endsWith("\n\n") ? "" : before.endsWith("\n") ? "\n" : "\n\n";
    const body = snippet.trimEnd();
    const trailing = after.length === 0 ? "\n" : after.startsWith("\n") ? "" : "\n\n";
    const inserted = leading + body + trailing;
    const next = before + inserted + after;
    const pos = before.length + inserted.length;
    setText(next);
    requestAnimationFrame(() => {
      const node = taRef.current;
      if (!node) return;
      node.focus();
      node.selectionStart = node.selectionEnd = pos;
      lastSelectionRef.current = { start: pos, end: pos };
    });
  };

  const addWorkspaceReference = (ref: WorkspaceReference) => {
    setWorkspaceRefs((prev) => {
      const key = workspaceReferenceKey(ref);
      if (prev.some((item) => workspaceReferenceKey(item) === key)) return prev;
      return [...prev, ref];
    });
    requestAnimationFrame(() => taRef.current?.focus());
  };

  useEffect(() => {
    if (!insertRequest || insertRequest.id === consumedInsertIdRef.current) return;
    consumedInsertIdRef.current = insertRequest.id;
    if (insertRequest.replace) {
      setText("");
      setWorkspaceRefs([]);
      setWriteContext(null);
    }
    if (insertRequest.writeContext && writeSurface) {
      setWriteContext(insertRequest.writeContext);
    }
    if (insertRequest.text) {
      const ref = parseWorkspaceReference(insertRequest.text);
      if (ref) {
        if (codeSurface) addWorkspaceReference(ref);
      } else if (!insertRequest.writeContext) {
        if (insertRequest.replace) {
          setText(insertRequest.text);
          requestAnimationFrame(() => {
            const node = taRef.current;
            if (!node) return;
            node.focus();
            const pos = insertRequest.text!.length;
            node.selectionStart = 0;
            node.selectionEnd = pos;
            lastSelectionRef.current = { start: 0, end: pos };
          });
        } else {
          insertTextAtCaret(insertRequest.text);
        }
      }
    }
    if (insertRequest.writeContext || insertRequest.text) {
      requestAnimationFrame(() => taRef.current?.focus());
    }
  }, [codeSurface, insertRequest, writeSurface]);

  const expandPastedBlocks = (displayText: string): string => {
    let expanded = displayText;
    for (const block of pastedBlocksRef.current) {
      if (expanded.includes(block.label)) {
        expanded = expanded.split(block.label).join(renderPastedBlock(block));
      }
    }
    return expanded;
  };

  const submit = useCallback(() => {
    if (disabled) return;
    const t = text.trim();
    const localSlash = t === "/memory" ? "memory" : null;
    if (localSlash) {
      if (onLocalSlash) {
        onLocalSlash(localSlash);
      } else {
        onSend(t);
      }
      setText("");
      setDismissed(true);
      return;
    }
    const pendingWriteContext = writeSurface ? writeContext : null;
    if (
      (!t && attachments.length === 0 && workspaceRefs.length === 0 && !pendingWriteContext) ||
      pendingPaste > 0
    ) {
      return;
    }
    const refs = codeSurface
      ? [
          ...workspaceRefs.map((ref) => formatWorkspaceReference(ref.path, ref.isDir)),
          ...attachments.map((a) => `@${a.path}`),
        ].join(" ")
      : "";

    if (pendingWriteContext) {
      const parts = [pendingWriteContext.instruction];
      if (t) parts.push(t);
      if (pendingWriteContext.selection) parts.push(`---\n${pendingWriteContext.selection}`);
      const submitBody = parts.join("\n\n");
      const writeFileRef =
        pendingWriteContext.scope === "document" && pendingWriteContext.filePath
          ? formatWorkspaceReference(pendingWriteContext.filePath)
          : "";
      const submitText = [submitBody, writeFileRef].filter(Boolean).join(" ");
      onSend(t || pendingWriteContext.actionLabel, submitText);
      setText("");
      setAttachments([]);
      setWorkspaceRefs([]);
      setWriteContext(null);
      return;
    }

    const displayText = [t, refs].filter(Boolean).join(t && refs ? " " : "");
    const submitText = [expandPastedBlocks(t), refs].filter(Boolean).join(t && refs ? " " : "");
    onSend(displayText, submitText);
    setText("");
    setAttachments([]);
    setWorkspaceRefs([]);
  }, [attachments, codeSurface, disabled, onLocalSlash, onSend, pendingPaste, text, workspaceRefs, writeContext, writeSurface]);

  const sendDisabled =
    pendingPaste > 0 ||
    (!text.trim() && attachments.length === 0 && workspaceRefs.length === 0 && !activeWriteContext) ||
    Boolean(disabled);

  useEffect(() => {
    if (!sendExternally || !onSendState) return;
    if (running) {
      onSendState({ disabled: false, onSend: submit });
      return;
    }
    onSendState({ disabled: sendDisabled, onSend: submit });
  }, [onSendState, running, sendDisabled, sendExternally, submit]);

  const readFileAsDataURL = (file: File) =>
    new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.onerror = () => reject(reader.error);
      reader.readAsDataURL(file);
    });

  const attachImageFiles = async (files: File[]) => {
    const images = files.filter((f) => f.type.startsWith("image/"));
    if (images.length === 0) return;
    for (const file of images) {
      setPendingPaste((n) => n + 1);
      try {
        const dataUrl = await readFileAsDataURL(file);
        const path = await app.SavePastedImage(dataUrl);
        const previewUrl = await app.AttachmentDataURL(path);
        setAttachments((prev) => [...prev, { path, previewUrl }]);
      } catch {
        // non-fatal: a failed image attach must not block normal text input
      } finally {
        setPendingPaste((n) => Math.max(0, n - 1));
      }
    }
  };

  // Non-image pastes (PDFs, docs): the clipboard hands us bytes, not a path, so
  // the kernel stores them and we reference the saved path — attached, not ignored.
  const attachOtherFiles = async (files: File[]) => {
    const others = files.filter((f) => !f.type.startsWith("image/"));
    if (others.length === 0) return;
    for (const file of others) {
      setPendingPaste((n) => n + 1);
      try {
        const dataUrl = await readFileAsDataURL(file);
        const path = await app.SavePastedFile(file.name, dataUrl);
        setAttachments((prev) => [...prev, { path }]);
      } catch {
        // non-fatal: a failed attach must not block normal text input
      } finally {
        setPendingPaste((n) => Math.max(0, n - 1));
      }
    }
  };

  const attachFiles = (files: File[]) => {
    void attachImageFiles(files);
    void attachOtherFiles(files);
  };

  // OS file drops arrive as absolute paths through the native bridge (the webview
  // withholds them from the HTML drop event); the kernel resolves each into a
  // workspace @reference or a stored attachment.
  const attachDroppedPaths = async (paths: string[]) => {
    setDragOver(false);
    dragDepthRef.current = 0;
    for (const path of paths) {
      setPendingPaste((n) => n + 1);
      try {
        const item = await app.AttachDropped(path);
        if (item.kind === "workspace") {
          addWorkspaceReference({ path: item.path, isDir: item.isDir });
        } else {
          setAttachments((prev) => [...prev, { path: item.path, previewUrl: item.previewUrl }]);
        }
      } catch {
        // non-fatal: a failed drop attach must not block normal text input
      } finally {
        setPendingPaste((n) => Math.max(0, n - 1));
      }
    }
  };

  attachDroppedPathsRef.current = attachDroppedPaths;

  useEffect(
    () =>
      onFilesDropped((paths) => {
        nativeDropAtRef.current = Date.now();
        void attachDroppedPathsRef.current(paths);
      }),
    [],
  );

  const onPaste = (e: ClipboardEvent<HTMLTextAreaElement>) => {
    const files = Array.from(e.clipboardData.files);
    if (files.length > 0) {
      e.preventDefault();
      attachFiles(files);
      return;
    }

    const pasted = e.clipboardData.getData("text");
    if (!shouldFoldPaste(pasted)) return;

    e.preventDefault();
    const ta = e.currentTarget;
    const start = ta.selectionStart ?? text.length;
    const end = ta.selectionEnd ?? text.length;
    const id = nextPasteId.current++;
    const lines = lineCount(pasted);
    const label = t("composer.pastedLabel", { id, lines });
    const block: PastedBlock = { label, text: pasted };
    const next = text.slice(0, start) + label + text.slice(end);

    pastedBlocksRef.current = [...pastedBlocksRef.current, block];
    setPastedBlocks((prev) => [...prev, block]);
    setText(next);
    requestAnimationFrame(() => {
      const node = taRef.current;
      if (!node) return;
      const pos = start + label.length;
      node.focus();
      node.selectionStart = node.selectionEnd = pos;
    });
  };

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    if (disabled) return;
    e.preventDefault();
    e.stopPropagation();
    dragDepthRef.current = 0;
    setDragOver(false);

    const droppedWorkspaceRef = readWorkspaceReferenceDrag(e.dataTransfer);
    if (droppedWorkspaceRef) {
      addWorkspaceReference(droppedWorkspaceRef);
      return;
    }

    const plain = e.dataTransfer.getData("text/plain").trim();
    const plainRef = plain ? parseWorkspaceReference(plain) : null;
    if (plainRef) {
      addWorkspaceReference(plainRef);
      return;
    }

    const uriPaths = readDroppedFileUriPaths(e.dataTransfer);
    if (uriPaths.length > 0) {
      void attachDroppedPaths(uriPaths);
      return;
    }

    const files = Array.from(e.dataTransfer.files);
    if (files.length > 0) {
      if (Date.now() - nativeDropAtRef.current < 250) return;
      attachFiles(files);
    }
  };

  const onDragOver = (e: DragEvent<HTMLDivElement>) => {
    if (disabled || !hasComposerFileDrag(e.dataTransfer)) return;
    e.preventDefault();
    e.stopPropagation();
    e.dataTransfer.dropEffect = "copy";
    setDragOver(true);
  };

  const onDragEnter = (e: DragEvent<HTMLDivElement>) => {
    if (disabled || !hasComposerFileDrag(e.dataTransfer)) return;
    e.preventDefault();
    e.stopPropagation();
    dragDepthRef.current += 1;
    setDragOver(true);
  };

  const onDragLeave = (e: DragEvent<HTMLDivElement>) => {
    if (disabled) return;
    e.stopPropagation();
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
    if (dragDepthRef.current === 0) setDragOver(false);
  };

  // handleCancel stops the in-flight turn; if it was cancelled before the server
  // replied, the just-sent text is handed back so we drop it back into the input.
  const handleCancel = () => {
    const restored = onCancel();
    if (typeof restored === "string") setTextCaretEnd(restored);
  };

  const pickCommand = (c: CommandInfo) => {
    if (c.name === "memory") {
      if (onLocalSlash) {
        onLocalSlash("memory");
      } else {
        onSend("/" + c.name);
      }
      setText("");
      setDismissed(true);
      return;
    }
    setTextCaretEnd("/" + c.name + " ");
  };

  const activePastedBlocks = pastedBlocks.filter((block) => text.includes(block.label));

  const removeWorkspaceReference = (target: WorkspaceReference) => {
    const key = workspaceReferenceKey(target);
    setWorkspaceRefs((prev) => prev.filter((ref) => workspaceReferenceKey(ref) !== key));
    requestAnimationFrame(() => taRef.current?.focus());
  };

  const togglePastedPreview = (label: string) => {
    setOpenPastedLabels((prev) => (prev.includes(label) ? prev.filter((x) => x !== label) : [...prev, label]));
  };

  const removePastedBlock = (block: PastedBlock) => {
    const next = text.split(block.label).join("");
    pastedBlocksRef.current = pastedBlocksRef.current.filter((x) => x.label !== block.label);
    setPastedBlocks((prev) => prev.filter((x) => x.label !== block.label));
    setOpenPastedLabels((prev) => prev.filter((x) => x !== block.label));
    setTextCaretEnd(next);
  };

  const expandPastedBlock = (block: PastedBlock) => {
    const next = text.split(block.label).join(block.text);
    pastedBlocksRef.current = pastedBlocksRef.current.filter((x) => x.label !== block.label);
    setPastedBlocks((prev) => prev.filter((x) => x.label !== block.label));
    setOpenPastedLabels((prev) => prev.filter((x) => x !== block.label));
    setTextCaretEnd(next);
  };

  const workspaceName = useMemo(() => {
    if (workspaceNone || !cwd) return t("composer.noWorkspace");
    const parts = cwd.split(/[/\\]/).filter(Boolean);
    return parts.length > 0 ? parts[parts.length - 1] : cwd;
  }, [cwd, t, workspaceNone]);

  const loadWorkspaces = () => {
    app.ListWorkspaces().then((next) => setWorkspaces(asArray(next))).catch(() => setWorkspaces([]));
  };

  useEffect(() => {
    if (workspaceMenuOpen) loadWorkspaces();
  }, [workspaceMenuOpen, cwd, workspaceRefreshSignal]);

  useEffect(() => {
    if (cwd) recordRecentWorkspace(cwd);
  }, [cwd]);

  const recentWorkspaces = useMemo(() => {
    const byPath = new Map(workspaces.map((workspace) => [workspace.path, workspace]));
    const ordered: WorkspaceView[] = [];
    const seen = new Set<string>();

    if (cwd && byPath.has(cwd)) {
      ordered.push(byPath.get(cwd)!);
      seen.add(cwd);
    }

    for (const path of getRecentWorkspacePaths()) {
      if (ordered.length >= 3) break;
      if (seen.has(path)) continue;
      const workspace = byPath.get(path);
      if (!workspace) continue;
      ordered.push(workspace);
      seen.add(path);
    }

    for (const workspace of workspaces) {
      if (ordered.length >= 3) break;
      if (seen.has(workspace.path)) continue;
      ordered.push(workspace);
      seen.add(workspace.path);
    }

    return ordered;
  }, [cwd, workspaces]);

  const chooseWorkspace = async (path?: string) => {
    const next = await onPickFolder(path);
    if (next && next !== NO_WORKSPACE_VALUE) {
      recordRecentWorkspace(next);
      setWorkspaceMenuOpen(false);
    }
  };

  const removeWorkspace = async (path: string) => {
    await onRemoveWorkspace(path);
    removeRecentWorkspace(path);
    setWorkspaces((prev) => prev.filter((w) => w.path !== path));
  };

  useEffect(() => {
    const onResize = () => setComposerHeight((height) => (height === null ? null : clampComposerHeight(height)));
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  const saveComposerHeight = (height: number) => {
    saveLayoutSize("composerHeight", height, clampComposerHeight);
  };

  const resetComposerHeight = () => {
    setComposerHeight(null);
    clearLayoutSize("composerHeight");
  };

  const onComposerResizeStart = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return;
    const card = composerCardRef.current;
    if (!card) return;

    e.preventDefault();
    const startY = e.clientY;
    const startHeight = composerHeight ?? card.getBoundingClientRect().height;
    let nextHeight = clampComposerHeight(startHeight);
    let moved = false;
    setComposerResizing(true);
    document.body.classList.add("composer-resizing");

    const onMove = (event: PointerEvent) => {
      moved = true;
      nextHeight = clampComposerHeight(startHeight + startY - event.clientY);
      setComposerHeight(nextHeight);
    };
    const onUp = () => {
      setComposerResizing(false);
      document.body.classList.remove("composer-resizing");
      if (moved) saveComposerHeight(nextHeight);
      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
      document.removeEventListener("pointercancel", onUp);
    };

    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
    document.addEventListener("pointercancel", onUp);
  };

  const pickEntry = (e: DirEntry) => {
    if (workspaceNone || !cwd) {
      void (async () => {
        try {
          const picked = await app.PickFilePath();
          if (!picked?.trim()) return;
          addWorkspaceReference({ path: picked.trim(), isDir: false });
          const atPos = text.length - (atRaw?.length ?? 0) - 1;
          setTextCaretEnd(text.slice(0, atPos));
          setDismissed(true);
        } catch {
          /* dialog cancelled */
        }
      })();
      return;
    }
    const atPos = text.length - (atRaw?.length ?? 0) - 1; // index of '@'
    const prefix = text.slice(0, atPos);
    // A directory keeps the menu open (trailing "/"); a file completes it (space).
    setTextCaretEnd(prefix + "@" + atDir + e.name + (e.isDir ? "/" : " "));
  };

  // pickArg replaces just the current token with the suggestion. A "descend" item
  // (e.g. "/skill show ") ends with a space, so the effect re-fetches the next
  // level; a terminal item leaves the menu (next fetch returns nothing).
  const pickArg = (it: SlashArgItem) => {
    if (!argRes) return;
    setTextCaretEnd(text.slice(0, argRes.from) + it.insert);
  };

  const pickActive = () => {
    if (menuMode === "slash") pickCommand(slashMatches[active]);
    else if (menuMode === "slasharg" && argRes) pickArg(argRes.items[active]);
    else if (menuMode === "at") pickEntry(atMatches[active]);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    const composing = isImeKeyEvent(e, composingRef.current, lastCompositionEndAt.current);
    if (e.key === "Enter" && composing) return;

    // Shift+Tab cycles the input mode (normal → plan → YOLO → normal). Handled
    // before the menus so it works even while one is open.
    if (e.key === "Tab" && e.shiftKey && !composing) {
      e.preventDefault();
      onCycleMode();
      return;
    }

    if (menuMode && !composing) {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setActive((i) => (i + 1) % count);
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setActive((i) => (i - 1 + count) % count);
        return;
      }
      if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault();
        pickActive();
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        setDismissed(true);
        return;
      }
    }

    // Enter sends; Shift+Enter newline. `composing` guards IME confirms.
    if (e.key === "Enter" && !e.shiftKey && !composing) {
      e.preventDefault();
      submit();
    }
    // Esc interrupts the in-flight turn (matches the Stop button's hint), and
    // restores the text if the server hadn't replied yet.
    if (e.key === "Escape" && running && !decisionPending) {
      e.preventDefault();
      handleCancel();
    }
  };

  const composerCardStyle = composerHeight === null ? undefined : ({ "--composer-height": `${composerHeight}px` } as CSSProperties);
  const hasWorkspace = Boolean(cwd) && !workspaceNone;
  const hasEffort = Boolean(effort?.supported);
  const composerMetaClass = [
    "composer-meta",
    hideModeBar ? "composer-meta--mode-inline" : "",
    hasWorkspace ? "composer-meta--has-workspace" : "composer-meta--no-workspace",
    hasEffort ? "composer-meta--has-effort" : "composer-meta--no-effort",
  ].join(" ");

  return (
    <div
      className={[
        "composer-wrap",
        decisionPending ? "composer-wrap--decision-pending" : "",
        dragOver ? "composer-wrap--dragover" : "",
        disabled ? "composer-wrap--disabled" : "",
      ]
        .filter(Boolean)
        .join(" ")}
      style={{ "--wails-drop-target": "drop" } as CSSProperties}
      onDrop={onDrop}
      onDragOver={onDragOver}
      onDragEnter={onDragEnter}
      onDragLeave={onDragLeave}
    >
      {dragOver ? (
        <div className="composer-drop-overlay" aria-hidden="true">
          <span>{t("composer.dropOverlay")}</span>
        </div>
      ) : null}
      <AnchoredPopover
        open={workspaceMenuOpen && showWorkspaceSwitcher}
        anchorRef={workspaceAnchorRef}
        onClose={() => setWorkspaceMenuOpen(false)}
        className="workspace-switcher workspace-switcher--portal"
      >
          <button
            type="button"
            className={`workspace-switcher__item workspace-switcher__item--solo${workspaceNone ? " workspace-switcher__item--current" : ""}`}
            onClick={() => {
              onUseNoWorkspace?.();
              setWorkspaceMenuOpen(false);
            }}
          >
            <FolderX size={15} />
            <span>{t("composer.useNoWorkspace")}</span>
            {workspaceNone ? <Check size={15} /> : null}
          </button>
          <div className="workspace-switcher__section-title">{t("composer.recentWorkspaces")}</div>
          <div className="workspace-switcher__list">
            {recentWorkspaces.map((w) => (
              <div className="workspace-switcher__row" key={w.path}>
                <button
                  className={`workspace-switcher__item${w.current && !workspaceNone ? " workspace-switcher__item--current" : ""}`}
                  title={w.path}
                  onClick={() => {
                    if (w.current && !workspaceNone) {
                      setWorkspaceMenuOpen(false);
                      return;
                    }
                    void chooseWorkspace(w.path);
                  }}
                >
                  <FolderGit2 size={15} />
                  <span>{w.name}</span>
                  {w.current && <Check size={15} />}
                </button>
                <button
                  className="workspace-switcher__remove"
                  type="button"
                  aria-label={t("composer.removeProject")}
                  title={t("composer.removeProject")}
                  disabled={running}
                  onClick={(event) => {
                    event.stopPropagation();
                    void removeWorkspace(w.path);
                  }}
                >
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
            {recentWorkspaces.length === 0 && (
              <div className="workspace-switcher__empty">{t("composer.noRecentWorkspaces")}</div>
            )}
          </div>
          <div className="workspace-switcher__actions">
            <button type="button" onClick={() => void chooseWorkspace()}>
              <FolderPlus size={15} />
              <span>{t("composer.addWorkspace")}</span>
            </button>
          </div>
      </AnchoredPopover>
      <AnchoredPopover
        open={paramMenu === "model"}
        anchorRef={modelAnchorRef}
        onClose={() => setParamMenu(null)}
        className="modelsw__menu modelsw__menu--portal"
        align="end"
      >
        <ModelSwitcherMenu
          tabId={tabId}
          onPick={(name) => {
            setParamMenu(null);
            onSwitchModel(name);
          }}
        />
      </AnchoredPopover>
      {effort?.supported && (
        <AnchoredPopover
          open={paramMenu === "effort" && !running}
          anchorRef={effortAnchorRef}
          onClose={() => setParamMenu(null)}
          className="modelsw__menu modelsw__menu--portal effortsw__menu"
          align="end"
        >
          <EffortSwitcherMenu
            effort={effort}
            onPick={(level) => {
              setParamMenu(null);
              if (level !== (effort.current || "auto")) onSetEffort(level);
            }}
          />
        </AnchoredPopover>
      )}
      {menuMode === "slash" && (
        <SlashMenu items={slashMatches} activeIndex={active} onPick={pickCommand} onHover={setActive} />
      )}
      {menuMode === "slasharg" && argRes && (
        <ArgMenu items={argRes.items} activeIndex={active} onPick={pickArg} onHover={setActive} />
      )}
      {menuMode === "at" && <FileMenu items={atMatches} activeIndex={active} onPick={pickEntry} onHover={setActive} />}
      {!hideModeBar ? (
      <div className="composer-toolbar">
        <div
          className="composer-modebar motion-segment"
          role="toolbar"
          aria-label={t("composer.modeTitle")}
          style={
            {
              "--motion-segment-index": mode === "normal" ? 0 : mode === "plan" ? 1 : 2,
              "--motion-segment-count": 3,
            } as CSSProperties
          }
        >
          {[
            { id: "normal" as Mode, label: "auto", icon: <Zap size={13} /> },
            { id: "plan" as Mode, label: "plan", icon: <List size={13} /> },
            { id: "yolo" as Mode, label: "yolo", icon: <AlertTriangle size={13} /> },
          ].map((option) => (
            <button
              key={option.id}
              type="button"
              className={`composer-modebar__item composer-modebar__item--${option.id}${mode === option.id ? " composer-modebar__item--active" : ""}`}
              onClick={() => onSetMode(option.id)}
              aria-pressed={mode === option.id}
            >
              {option.icon}
              <span>{option.label}</span>
            </button>
          ))}
        </div>
      </div>
      ) : null}
      {activePastedBlocks.length > 0 && (
        <div className="composer__pasted">
          {activePastedBlocks.map((block) => {
            const open = openPastedLabels.includes(block.label);
            return (
              <div className="composer__pasted-block" key={block.label}>
                <div className="composer__pasted-head">
                  <FileText size={15} aria-hidden="true" />
                  <span className="composer__pasted-label">{block.label}</span>
                  <div className="composer__pasted-actions">
                    <Tooltip label={t(open ? "composer.pastedHidePreview" : "composer.pastedShowPreview")}>
                      <button
                        type="button"
                        className="composer__pasted-btn composer__pasted-btn--icon"
                        aria-pressed={open}
                        onClick={() => togglePastedPreview(block.label)}
                      >
                        <Eye size={14} aria-hidden="true" />
                      </button>
                    </Tooltip>
                    <Tooltip label={t("composer.pastedExpand")}>
                      <button
                        type="button"
                        className="composer__pasted-btn"
                        onClick={() => expandPastedBlock(block)}
                      >
                        {t("composer.pastedExpand")}
                      </button>
                    </Tooltip>
                    <Tooltip label={t("composer.pastedRemove")}>
                      <button
                        type="button"
                        className="composer__pasted-btn composer__pasted-btn--icon composer__pasted-btn--danger"
                        onClick={() => removePastedBlock(block)}
                      >
                        <Trash2 size={14} aria-hidden="true" />
                      </button>
                    </Tooltip>
                  </div>
                </div>
                {open && <pre className="composer__pasted-preview">{block.text}</pre>}
              </div>
            );
          })}
        </div>
      )}
      <div
        className={[
          "composer-card",
          composerHeight !== null ? "composer-card--resized" : "",
          composerResizing ? "composer-card--resizing" : "",
          hasContextTags ? "composer-card--has-context" : "",
        ]
          .filter(Boolean)
          .join(" ")}
        ref={composerCardRef}
        style={composerCardStyle}
      >
        <div
          className="composer-resize-handle"
          onPointerDown={onComposerResizeStart}
          onDoubleClick={resetComposerHeight}
        />
        {hasContextTags ? (
          <div className="composer-context" aria-label={t("composer.contextItems")}>
            <div className="composer-context__tags">
            <ComposerSessionGroup
              kind="terminal"
              icon={<SquareTerminal size={14} />}
              sessions={showTerminalTags ? terminalSessions : []}
              groupLabel={(n) => t("composer.terminalsGroup", { n: String(n) })}
              activeFallback={t("composer.terminalActive")}
              closeTabLabel={(title) => t("terminal.closeTab", { title })}
              onOpen={onTerminalSessionOpen}
              onClose={onTerminalSessionClose}
            />
            <ComposerSessionGroup
              kind="browser"
              icon={<Monitor size={14} />}
              sessions={showBrowserTags ? browserSessions : []}
              groupLabel={(n) => t("composer.browsersGroup", { n: String(n) })}
              activeFallback={t("composer.browserActive")}
              closeTabLabel={(title) => t("browser.closeTab", { title })}
              onOpen={onBrowserSessionOpen}
              onClose={onBrowserSessionClose}
            />
            {showPageTag ? (
              <ComposerSessionChip
                kind="page"
                icon={<Globe size={14} />}
                label={pagePreviewLabel?.trim() || t("composer.pageActive")}
                onOpen={onPageSessionOpen}
                onClose={onPageSessionClose}
                closeLabel={t("browser.closeTab", { title: pagePreviewLabel?.trim() || t("composer.pageActive") })}
              />
            ) : null}
            {activeWriteContext ? (
              <div className="composer-context__item composer-context__item--write">
                {activeWriteContext.selection ? (
                  <Tooltip label={activeWriteContext.selection}>
                    <span className="composer-context__label">
                      <TextQuote size={14} />
                      <span className="composer-context__text">
                        <span className="composer-context__name">
                          {activeWriteContext.actionLabel} · {t("write.scopeSelection")}
                        </span>
                      </span>
                    </span>
                  </Tooltip>
                ) : (
                  <span className="composer-context__label">
                    <TextQuote size={14} />
                    <span className="composer-context__text">
                      <span className="composer-context__name">
                        {activeWriteContext.actionLabel} · {t("write.scopeDocument")}
                      </span>
                    </span>
                  </span>
                )}
                <Tooltip label={t("composer.removeReference")}>
                  <button type="button" onClick={() => setWriteContext(null)}>
                    <X size={13} />
                  </button>
                </Tooltip>
              </div>
            ) : null}
            {showCodeAttachments
              ? attachments.map((a) => (
              <div
                className={`composer-context__item${a.previewUrl ? " composer-context__item--image" : " composer-context__item--attachment"}`}
                key={a.path}
              >
                <Tooltip label={a.path}>
                  <span className="composer-context__label">
                    {a.previewUrl ? <img src={a.previewUrl} alt="" /> : <FileText size={14} />}
                    <span className="composer-context__text">
                      <span className="composer-context__name">{basename(a.path)}</span>
                    </span>
                  </span>
                </Tooltip>
                <Tooltip label={t("composer.removeImage")}>
                  <button
                    type="button"
                    onClick={() => setAttachments((prev) => prev.filter((x) => x.path !== a.path))}
                  >
                    <X size={13} />
                  </button>
                </Tooltip>
              </div>
            ))
              : null}
            {showCodeWorkspaceRefs
              ? workspaceRefs.map((ref) => {
              const refName = ref.isDir ? `${basename(ref.path)}/` : basename(ref.path);
              const refParent = parentPath(ref.path);
              return (
                <div
                  className={`composer-context__item composer-context__item--workspace${ref.isDir ? " composer-context__item--folder" : " composer-context__item--file"}`}
                  key={workspaceReferenceKey(ref)}
                >
                  <Tooltip label={formatWorkspaceReference(ref.path, ref.isDir)}>
                    <span className="composer-context__label">
                      {ref.isDir ? <Folder size={14} /> : <FileText size={14} />}
                      <span className="composer-context__text">
                        <span className="composer-context__at">@</span>
                        <span className="composer-context__name">{refName}</span>
                        {refParent ? <span className="composer-context__path">{refParent}</span> : null}
                      </span>
                    </span>
                  </Tooltip>
                  <Tooltip label={t("composer.removeReference")}>
                    <button
                      type="button"
                      onClick={() => removeWorkspaceReference(ref)}
                    >
                      <X size={13} />
                    </button>
                  </Tooltip>
                </div>
              );
            })
              : null}
            </div>
          </div>
        ) : null}
        <div
          className={[
            "composer",
            disabled ? " composer--disabled" : "",
            text.trimStart().startsWith("!") ? " composer--shell" : "",
          ]
            .filter(Boolean)
            .join(" ")}
        >
          <textarea
            ref={taRef}
            className="composer__input"
            value={text}
            onChange={(e) => setText(e.target.value)}
            onSelect={rememberCaret}
            onClick={rememberCaret}
            onKeyUp={rememberCaret}
            onFocus={rememberCaret}
            onPaste={onPaste}
            onKeyDown={onKeyDown}
            onCompositionStart={() => {
              composingRef.current = true;
            }}
            onCompositionEnd={() => {
              composingRef.current = false;
              lastCompositionEndAt.current = Date.now();
            }}
            placeholder={disabled ? t("common.loading") : t("composer.placeholder")}
            rows={1}
            disabled={disabled}
          />
        </div>
        {!sendExternally ? (
          <div className="composer-send-wrap">
            {running ? (
              <Tooltip label={t("composer.stop")}>
                <button
                  className="composer__btn composer__btn--stop"
                  type="button"
                  onClick={handleCancel}
                  disabled={decisionPending}
                  aria-label={t("composer.stopShort")}
                >
                  <Square size={14} fill="currentColor" />
                </button>
              </Tooltip>
            ) : (
              <Tooltip label={t("composer.send")}>
                <button className="composer__btn composer__btn--send" type="button" onClick={submit} disabled={sendDisabled}>
                  <ArrowUp size={16} />
                </button>
              </Tooltip>
            )}
          </div>
        ) : null}
        {retry ? (
          <p className="composer-retry-status" role="status">
            {t("status.retrying", { attempt: retry.attempt, max: retry.max })}
          </p>
        ) : null}
        <div className={composerMetaClass}>
          {hideModeBar && (
            <div className="composer-meta__mode">
              <ComposerModeBar
                mode={mode}
                onSetMode={onSetMode}
                inline
              />
            </div>
          )}
          {showWorkspaceSwitcher ? (
            <div className="composer-meta__control composer-meta__control--workspace composer-workspace-wrap" ref={workspaceAnchorRef}>
              <button
                className={`composer__workspace${workspaceMenuOpen ? " composer__workspace--open" : ""}${workspaceNone ? " composer__workspace--none" : ""}`}
                onClick={() => {
                  if (!running) {
                    setParamMenu(null);
                    setWorkspaceMenuOpen((open) => !open);
                  }
                }}
                disabled={running}
              >
                {workspaceNone ? <FolderX size={13} /> : <FolderGit2 size={13} />}
                <span>{workspaceName}</span>
                <ChevronDown size={12} />
              </button>
            </div>
          ) : null}
          <div className="composer-meta__model-effort">
            <div className="composer-meta__control composer-meta__control--model" ref={modelAnchorRef}>
              <ModelSwitcherTrigger
                label={modelLabelFromRef(modelLabel)}
                title={modelLabel}
                open={paramMenu === "model"}
                disabled={running}
                onClick={() => {
                  if (running) return;
                  setWorkspaceMenuOpen(false);
                  setParamMenu((current) => (current === "model" ? null : "model"));
                }}
              />
            </div>
            {effort?.supported && (
              <div className="composer-meta__control composer-meta__control--effort" ref={effortAnchorRef}>
                <div className="modelsw effortsw">
                  <EffortSwitcherTrigger
                    effort={effort}
                    open={paramMenu === "effort"}
                    disabled={running}
                    onClick={() => {
                      if (running) return;
                      setWorkspaceMenuOpen(false);
                      setParamMenu((current) => (current === "effort" ? null : "effort"));
                    }}
                  />
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
