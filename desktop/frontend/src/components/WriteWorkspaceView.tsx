import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { KeyboardEvent as ReactKeyboardEvent, SyntheticEvent } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  Download,
  FileText,
  ListTree,
  Minimize2,
  PanelRightClose,
  PanelRightOpen,
  PenLine,
  Save,
  Sparkles,
  TextQuote,
  Wand2,
} from "lucide-react";
import { app } from "../lib/bridge";
import { toErrorMessage } from "../lib/errors";
import { useT } from "../lib/i18n";
import type { ComposerWriteContext } from "../lib/types";
import type { WriteTurn } from "../lib/writeConversation";
import { latestWriteAssistant } from "../lib/writeConversation";
import { exportDocxFile, exportHtmlFile, exportMarkdownFile, exportPdfFile } from "../lib/writeExport";
import { isWordDocumentPath } from "../lib/writeDocument";
import { writeActionPromptKey } from "../lib/writePrompts";
import { basename, parentPath } from "../lib/workspaceFilePreview";
import { closeStudioSelect, openStudioSelect } from "../lib/studioSelectRegistry";
import { AnchoredPopover } from "./AnchoredPopover";
import { Tooltip } from "./Tooltip";
import { WriteConversationThread } from "./WriteConversationThread";

export type WriteViewMode = "source" | "preview";
export type WriteApplyMode = "selection" | "append" | "replace";

export interface WriteWorkspaceViewProps {
  filePath?: string;
  onSaved?: () => void;
  onFilePathChange?: (path: string) => void;
  onDraftComposer?: (context: ComposerWriteContext) => void;
  onDirtyChange?: (dirty: boolean) => void;
  conversationTurns?: WriteTurn[];
  agentRunning?: boolean;
  rightPanelOpen?: boolean;
  onToggleRightPanel?: () => void;
}

const QUICK_ACTION_KEYS = [
  { key: "write.action.summarize", icon: TextQuote },
  { key: "write.action.outline", icon: ListTree },
  { key: "write.action.polish", icon: Wand2 },
  { key: "write.action.expand", icon: PenLine },
  { key: "write.action.shorten", icon: Minimize2 },
  { key: "write.action.proofread", icon: Sparkles },
] as const;

const VIEW_MODES: WriteViewMode[] = ["source", "preview"];
const APPLY_MODES: WriteApplyMode[] = ["selection", "append", "replace"];
const AUTO_SAVE_MS = 2500;

function countWritingStats(text: string): { chars: number; words: number; readingMin: number } {
  const trimmed = text.trim();
  if (!trimmed) return { chars: 0, words: 0, readingMin: 0 };
  const words = trimmed.split(/\s+/).filter(Boolean).length;
  return {
    chars: trimmed.length,
    words,
    readingMin: Math.max(1, Math.ceil(words / 200)),
  };
}

export function WriteWorkspaceView({
  filePath,
  onSaved,
  onFilePathChange,
  onDraftComposer,
  onDirtyChange,
  conversationTurns = [],
  agentRunning = false,
  rightPanelOpen = false,
  onToggleRightPanel,
}: WriteWorkspaceViewProps) {
  const t = useT();
  const editorRef = useRef<HTMLTextAreaElement>(null);
  const exportAnchorRef = useRef<HTMLButtonElement>(null);
  const [viewMode, setViewMode] = useState<WriteViewMode>("source");
  const [content, setContent] = useState("");
  const [previewHtml, setPreviewHtml] = useState("");
  const [dirty, setDirty] = useState(false);
  const [autoSaved, setAutoSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [selectionText, setSelectionText] = useState("");
  const [stagedAction, setStagedAction] = useState<(typeof QUICK_ACTION_KEYS)[number]["key"] | null>(null);
  const [applyMode, setApplyMode] = useState<WriteApplyMode>("selection");
  const [exportMenuOpen, setExportMenuOpen] = useState(false);
  const closeExportMenu = useCallback(() => setExportMenuOpen(false), []);
  const [fimSuggestion, setFimSuggestion] = useState("");
  const [fimBusy, setFimBusy] = useState(false);

  const latestAssistant = useMemo(() => latestWriteAssistant(conversationTurns), [conversationTurns]);
  const agentReply = latestAssistant?.text ?? "";
  const agentReplyStreaming = latestAssistant?.streaming ?? false;
  const wordDocument = isWordDocumentPath(filePath);

  const setDirtyState = useCallback(
    (next: boolean) => {
      setDirty(next);
      if (next) setAutoSaved(false);
      onDirtyChange?.(next);
    },
    [onDirtyChange],
  );

  const load = useCallback(async () => {
    if (!filePath) {
      setContent("");
      setPreviewHtml("");
      setDirtyState(false);
      setSelectionText("");
      setStagedAction(null);
      setFimSuggestion("");
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const body = await app.ReadWriteFile(filePath);
      setContent(body);
      setDirtyState(false);
      setSelectionText("");
      setStagedAction(null);
      setFimSuggestion("");
      if (wordDocument) {
        try {
          const preview = await app.ReadWriteFilePreview(filePath);
          setPreviewHtml(preview);
        } catch {
          setPreviewHtml("");
        }
      } else {
        setPreviewHtml("");
      }
    } catch (e) {
      setErr(toErrorMessage(e));
    } finally {
      setBusy(false);
    }
  }, [filePath, setDirtyState, wordDocument]);

  useEffect(() => {
    if (wordDocument) setViewMode("preview");
  }, [filePath, wordDocument]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!exportMenuOpen) return;
    openStudioSelect(closeExportMenu);
    return () => closeStudioSelect(closeExportMenu);
  }, [exportMenuOpen, closeExportMenu]);

  const save = useCallback(async () => {
    let targetPath = filePath?.trim() ?? "";
    if (!targetPath) {
      const stamp = new Date().toISOString().slice(0, 10);
      const picked = await app.PickSaveFilePath(`untitled-${stamp}.md`);
      if (!picked?.trim()) return;
      targetPath = picked.trim();
      onFilePathChange?.(targetPath);
    }
    setBusy(true);
    setErr(null);
    try {
      await app.WriteWriteFile(targetPath, content);
      setDirtyState(false);
      setAutoSaved(true);
      onSaved?.();
    } catch (e) {
      setErr(toErrorMessage(e));
    } finally {
      setBusy(false);
    }
  }, [content, filePath, onFilePathChange, onSaved, setDirtyState]);

  useEffect(() => {
    if (!dirty || busy) return;
    const id = window.setTimeout(() => {
      void save();
    }, AUTO_SAVE_MS);
    return () => window.clearTimeout(id);
  }, [busy, content, dirty, save]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
        event.preventDefault();
        if (dirty && !busy) void save();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [busy, dirty, save]);

  const stats = useMemo(() => countWritingStats(content), [content]);
  const fileName = filePath ? basename(filePath) : "";
  const fileDir = filePath ? parentPath(filePath) : "";
  const hasSelection = selectionText.trim().length > 0;

  const requestFim = useCallback(async () => {
    const el = editorRef.current;
    if (!el || !filePath || fimBusy) return;
    const start = el.selectionStart;
    const end = el.selectionEnd;
    const before = el.value.slice(0, start);
    const after = el.value.slice(end);
    if (!before.trim() && !after.trim()) return;
    setFimBusy(true);
    try {
      const suggestion = await app.CompleteWriteInline(before, after);
      setFimSuggestion(suggestion.trim());
    } catch {
      setFimSuggestion("");
    } finally {
      setFimBusy(false);
    }
  }, [filePath, fimBusy]);

  const acceptFim = () => {
    const el = editorRef.current;
    if (!el || !fimSuggestion) return;
    const start = el.selectionStart;
    const end = el.selectionEnd;
    const next = `${el.value.slice(0, start)}${fimSuggestion}${el.value.slice(end)}`;
    setContent(next);
    setDirtyState(true);
    setFimSuggestion("");
    window.requestAnimationFrame(() => {
      const pos = start + fimSuggestion.length;
      el.focus();
      el.setSelectionRange(pos, pos);
    });
  };

  const syncSelection = (event: SyntheticEvent<HTMLTextAreaElement>) => {
    const el = event.currentTarget;
    const text = el.value.slice(el.selectionStart, el.selectionEnd).trim();
    setSelectionText(text);
    setFimSuggestion("");
  };

  const stageQuickAction = (actionKey: (typeof QUICK_ACTION_KEYS)[number]["key"]) => {
    if (!onDraftComposer || !filePath) return;
    const excerpt = selectionText.trim() || content.trim();
    if (!excerpt) return;
    const scope = hasSelection ? "selection" : "document";
    onDraftComposer({
      actionKey,
      actionLabel: t(actionKey),
      instruction: t(writeActionPromptKey(actionKey, scope) as "write.action.summarizePromptSelection"),
      scope,
      filePath: hasSelection ? undefined : filePath,
      fileName,
      selection: hasSelection ? selectionText.trim() : undefined,
    });
    setStagedAction(actionKey);
  };

  const applyAgentReply = () => {
    const text = agentReply?.trim();
    if (!text || !filePath) return;
    const el = editorRef.current;
    if (applyMode === "replace") {
      setContent(text);
    } else if (applyMode === "append") {
      setContent((prev) => (prev.trim() ? `${prev.trimEnd()}\n\n${text}` : text));
    } else if (el && hasSelection) {
      const start = el.selectionStart;
      const end = el.selectionEnd;
      setContent(el.value.slice(0, start) + text + el.value.slice(end));
    } else {
      setContent((prev) => (prev.trim() ? `${prev.trimEnd()}\n\n${text}` : text));
    }
    setDirtyState(true);
    setSelectionText("");
  };

  const onEditorKeyDown = (event: ReactKeyboardEvent<HTMLTextAreaElement>) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
      event.preventDefault();
      if (dirty && !busy) void save();
      return;
    }
    if ((event.ctrlKey || event.metaKey) && event.code === "Space") {
      event.preventDefault();
      void requestFim();
      return;
    }
    if (event.key === "Tab" && fimSuggestion && !event.shiftKey) {
      event.preventDefault();
      acceptFim();
      return;
    }
    if (event.key === "Escape") {
      setFimSuggestion("");
    }
  };

  const showEditor = viewMode === "source";
  const showPreview = viewMode === "preview";

  return (
    <div className="write-workspace write-studio write-studio--split">
      {err ? <div className="write-workspace__banner write-workspace__banner--error">{err}</div> : null}

      {!filePath ? (
        <div className="write-studio__empty">
          <FileText size={28} strokeWidth={1.5} />
          <strong>{t("write.emptyTitle")}</strong>
          <p>{t("write.emptySelect")}</p>
        </div>
      ) : (
        <div className="write-studio__split">
          <div className="write-studio__editor-column">
            <header className="write-studio__editor-toolbar wails-no-drag">
              <div className="write-studio__doc">
                <span className="write-studio__doc-icon" aria-hidden="true">
                  <FileText size={16} />
                </span>
                <div className="write-studio__doc-copy">
                  <strong>{fileName}</strong>
                  {fileDir ? <span>{fileDir}</span> : null}
                </div>
              </div>

              <div className="write-studio__segments" role="tablist" aria-label={t("write.viewMode")}>
                {VIEW_MODES.map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    role="tab"
                    aria-selected={viewMode === mode}
                    className={`write-studio__segment${viewMode === mode ? " write-studio__segment--active" : ""}`}
                    onClick={() => setViewMode(mode)}
                  >
                    {t(`write.tab.${mode}` as "write.tab.source")}
                  </button>
                ))}
              </div>

              <div className="write-studio__toolbar-actions">
                {onToggleRightPanel ? (
                  <Tooltip label={rightPanelOpen ? t("sidebar.collapsePanel") : t("sidebar.openPanel")}>
                    <button
                      type="button"
                      className={`write-studio__icon-btn${rightPanelOpen ? " write-studio__icon-btn--active" : ""}`}
                      onClick={() => onToggleRightPanel()}
                      aria-label={rightPanelOpen ? t("sidebar.collapsePanel") : t("sidebar.openPanel")}
                      aria-pressed={rightPanelOpen}
                    >
                      {rightPanelOpen ? <PanelRightClose size={15} /> : <PanelRightOpen size={15} />}
                    </button>
                  </Tooltip>
                ) : null}
                <button
                  ref={exportAnchorRef}
                  type="button"
                  className="write-studio__icon-btn"
                  disabled={!content.trim()}
                  onClick={() => setExportMenuOpen((open) => !open)}
                  aria-label={t("write.export.menu")}
                >
                  <Download size={15} />
                </button>
                <AnchoredPopover
                  open={exportMenuOpen}
                  anchorRef={exportAnchorRef}
                  onClose={closeExportMenu}
                  className="write-studio__export-popover"
                  align="end"
                  placement="bottom"
                >
                  <div className="write-studio__export-menu">
                    {(
                      [
                        ["markdown", () => exportMarkdownFile(content, fileName)],
                        ["html", () => exportHtmlFile(content, fileName)],
                        ["pdf", () => exportPdfFile(content, fileName)],
                        ["docx", () => exportDocxFile(content, fileName)],
                      ] as const
                    ).map(([kind, run]) => (
                      <button
                        key={kind}
                        type="button"
                        className="write-studio__export-item"
                        onClick={() => {
                          run();
                          setExportMenuOpen(false);
                        }}
                      >
                        {t(`write.export.${kind}` as "write.export.markdown")}
                      </button>
                    ))}
                  </div>
                </AnchoredPopover>
                <button
                  type="button"
                  className="write-studio__save"
                  disabled={busy || (!dirty && Boolean(filePath))}
                  onClick={() => void save()}
                >
                  <Save size={14} />
                  {t("write.save")}
                </button>
              </div>
            </header>

            <div className={`write-workspace__pane write-workspace__pane--${viewMode} write-studio__pane`}>
              {showEditor ? (
                <div className="write-studio__editor-wrap">
                  <textarea
                    ref={editorRef}
                    className="write-workspace__editor write-studio__editor"
                    value={content}
                    disabled={busy}
                    onChange={(e) => {
                      setContent(e.target.value);
                      setDirtyState(true);
                      setFimSuggestion("");
                    }}
                    onSelect={syncSelection}
                    onKeyUp={syncSelection}
                    onMouseUp={syncSelection}
                    onKeyDown={onEditorKeyDown}
                    spellCheck
                  />
                  {fimSuggestion ? <div className="write-studio__fim-ghost">{fimSuggestion}</div> : null}
                </div>
              ) : null}
              {showPreview ? (
                <div className={`write-workspace__preview write-studio__preview${viewMode === "preview" ? " write-studio__preview--live" : ""}${wordDocument && previewHtml ? " write-studio__preview--word" : ""}`}>
                  {wordDocument && previewHtml ? (
                    <div className="write-studio__word-preview" dangerouslySetInnerHTML={{ __html: previewHtml }} />
                  ) : (
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{content || t("write.previewEmpty")}</ReactMarkdown>
                  )}
                </div>
              ) : null}
            </div>

            <footer className="write-workspace__footer write-studio__footer">
              <span>{t("write.chars", { n: stats.chars })}</span>
              <span>{t("write.words", { n: stats.words })}</span>
              {dirty ? <span className="write-workspace__dirty">{t("write.unsaved")}</span> : null}
              {!dirty && autoSaved ? <span className="write-studio__autosaved">{t("write.autoSaved")}</span> : null}
              <span className="write-workspace__fim">
                {fimBusy ? t("write.fimLoading") : fimSuggestion ? t("write.fimHint") : t("write.fimEmpty")}
              </span>
              <span className="write-studio__hint">{t("write.saveShortcut")}</span>
            </footer>
          </div>

          <aside className="write-studio__assistant-panel" aria-label={t("write.assistant")}>
            <header className="write-studio__panel-head">
              <div className="write-studio__panel-title">
                <Sparkles size={15} />
                <h3>{t("write.assistant")}</h3>
              </div>
            </header>

            <div className="write-studio__action-tabs" role="tablist" aria-label={t("write.actionsTitle")}>
              {QUICK_ACTION_KEYS.map(({ key, icon: Icon }) => (
                <Tooltip key={key} label={t(`${key}Hint`)}>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={stagedAction === key}
                    className={`write-studio__action-tab${stagedAction === key ? " write-studio__action-tab--active" : ""}`}
                    disabled={!onDraftComposer || agentRunning || (!selectionText && !content.trim())}
                    onClick={() => stageQuickAction(key)}
                  >
                    <Icon size={13} />
                    {t(key)}
                  </button>
                </Tooltip>
              ))}
            </div>

            <div className="write-studio__panel-reply">
              <div className="write-studio__reply-head">
                <span>{t("write.conversationTitle")}</span>
              </div>
              <WriteConversationThread turns={conversationTurns} running={agentRunning} variant="panel" />
            </div>

            <footer className="write-studio__panel-foot">
              <div className="write-studio__apply-modes" role="group" aria-label={t("write.applyModeLabel")}>
                {APPLY_MODES.map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    className={`write-studio__apply-mode${applyMode === mode ? " write-studio__apply-mode--active" : ""}`}
                    disabled={mode === "selection" && !hasSelection}
                    onClick={() => setApplyMode(mode)}
                  >
                    {t(`write.applyMode.${mode}` as "write.applyMode.selection")}
                  </button>
                ))}
              </div>
              <button
                type="button"
                className="write-studio__apply-primary"
                disabled={!agentReply?.trim() || agentReplyStreaming || (applyMode === "selection" && !hasSelection)}
                onClick={applyAgentReply}
              >
                {t("write.applyReply")}
              </button>
            </footer>
          </aside>
        </div>
      )}
    </div>
  );
}
