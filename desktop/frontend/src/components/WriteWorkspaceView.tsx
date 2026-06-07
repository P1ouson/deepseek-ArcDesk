import { useCallback, useEffect, useMemo, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";

export type WriteViewMode = "source" | "split" | "preview";

export interface WriteWorkspaceViewProps {
  filePath?: string;
  onSaved?: () => void;
  onPrompt?: (text: string) => void;
}

const QUICK_ACTION_KEYS = [
  { key: "write.action.summarize", prompt: "write.action.summarizePrompt" },
  { key: "write.action.outline", prompt: "write.action.outlinePrompt" },
  { key: "write.action.polish", prompt: "write.action.polishPrompt" },
  { key: "write.action.expand", prompt: "write.action.expandPrompt" },
] as const;

export function WriteWorkspaceView({ filePath, onSaved, onPrompt }: WriteWorkspaceViewProps) {
  const t = useT();
  const [viewMode, setViewMode] = useState<WriteViewMode>("split");
  const [content, setContent] = useState("");
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!filePath) {
      setContent("");
      setDirty(false);
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const body = await app.ReadWriteFile(filePath);
      setContent(body);
      setDirty(false);
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  }, [filePath]);

  useEffect(() => {
    void load();
  }, [load]);

  const wordCount = useMemo(() => content.trim().split(/\s+/).filter(Boolean).length, [content]);

  const save = async () => {
    if (!filePath) return;
    setBusy(true);
    setErr(null);
    try {
      await app.WriteWriteFile(filePath, content);
      setDirty(false);
      onSaved?.();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const exportMarkdown = () => {
    if (!filePath || !content) return;
    const blob = new Blob([content], { type: "text/markdown;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filePath.split(/[/\\]/).pop() || "draft.md";
    anchor.click();
    URL.revokeObjectURL(url);
  };

  const showEditor = viewMode === "source" || viewMode === "split";
  const showPreview = viewMode === "preview" || viewMode === "split";

  return (
    <div className="write-workspace">
      <div className="write-workspace__primary">
        <div className="write-workspace__toolbar">
          <div className="write-workspace__exports">
            <button type="button" disabled={!filePath || !content.trim()} onClick={exportMarkdown}>
              {t("write.export.markdown")}
            </button>
            <button type="button" disabled title={t("write.export.soon")}>{t("write.export.html")}</button>
            <button type="button" disabled title={t("write.export.soon")}>{t("write.export.pdf")}</button>
            <button type="button" disabled title={t("write.export.soon")}>{t("write.export.docx")}</button>
          </div>
          <div className="write-workspace__tabs">
            <button type="button" className="write-workspace__tab write-workspace__tab--disabled" disabled title="Live editor coming soon">
              {t("write.tab.live")}
            </button>
            {(["source", "split", "preview"] as const).map((mode) => (
              <button
                key={mode}
                type="button"
                className={`write-workspace__tab${viewMode === mode ? " write-workspace__tab--active" : ""}`}
                onClick={() => setViewMode(mode)}
              >
                {t(`write.tab.${mode}` as "write.tab.source")}
              </button>
            ))}
          </div>
          <button type="button" className="write-workspace__save" disabled={!filePath || busy || !dirty} onClick={() => void save()}>
            {t("write.save")}
          </button>
        </div>

        {err ? <div className="write-workspace__banner write-workspace__banner--error">{err}</div> : null}

        {!filePath ? (
          <div className="write-workspace__empty">{t("write.emptySelect")}</div>
        ) : (
          <div className={`write-workspace__pane write-workspace__pane--${viewMode}`}>
            {showEditor ? (
              <textarea
                className="write-workspace__editor"
                value={content}
                disabled={busy}
                onChange={(e) => {
                  setContent(e.target.value);
                  setDirty(true);
                }}
                spellCheck
              />
            ) : null}
            {showPreview ? (
              <div className="write-workspace__preview">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{content || t("write.previewEmpty")}</ReactMarkdown>
              </div>
            ) : null}
          </div>
        )}

        <footer className="write-workspace__footer">
          <span>{t("write.words", { n: wordCount })}</span>
          {dirty ? <span className="write-workspace__dirty">{t("write.unsaved")}</span> : null}
          <span className="write-workspace__fim">{t("write.fimTodo")}</span>
        </footer>
      </div>

      <aside className="write-workspace__aside">
        <section className="write-workspace__card">
          <h3>{t("write.assistant")}</h3>
          <div className="write-workspace__quickgrid">
            {QUICK_ACTION_KEYS.map(({ key, prompt }) => (
              <button
                key={key}
                type="button"
                disabled={!filePath || !onPrompt}
                onClick={() => onPrompt?.(t(prompt))}
              >
                {t(key)}
              </button>
            ))}
          </div>
        </section>
        <section className="write-workspace__card">
          <h3>{t("write.inlineAgent")}</h3>
          <p className="write-workspace__hint">{t("write.inlineHint")}</p>
        </section>
      </aside>
    </div>
  );
}
