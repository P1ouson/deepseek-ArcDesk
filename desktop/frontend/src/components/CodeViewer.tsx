import { lazy, Suspense, useCallback, useEffect, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";
import { CopyButton } from "./CopyButton";
import { CodeBlockToolbar } from "./CodeBlockToolbar";
import { useT } from "../lib/i18n";
import { Tooltip } from "./Tooltip";

export interface EditorProps {
  value: string;
  language?: string;
  readOnly?: boolean;
  maxHeight?: number;
  /** Single inner frame — no outer .code-block chrome (file preview, etc.). */
  flat?: boolean;
  /** Chat-style header: language label + copy / download / run. */
  toolbar?: boolean;
  lineNumbers?: boolean;
}

// ── EDITOR SEAM (code) ───────────────────────────────────────────────────────
// Every code view in the app renders through this component, so upgrading the
// editor is a one-line change here — swap the lazily-imported module:
//
//   ./editors/HljsCode         current — highlight.js read-only view
//   ./editors/MonacoCode       pnpm add @monaco-editor/react monaco-editor
//   ./editors/CodeMirrorCode   pnpm add @uiw/react-codemirror @codemirror/lang-*
//
// The replacement only has to honor EditorProps. It's lazy-loaded so a heavy
// editor (~MBs) never lands in the initial bundle — it streams in the first time
// a code block or tool result is shown. See desktop/README.md ("Editor seam").
const Impl = lazy(() => import("./editors/HljsCode"));

export function CodeViewer(props: EditorProps) {
  const { flat, toolbar, lineNumbers, maxHeight } = props;
  const t = useT();
  const bodyRef = useRef<HTMLDivElement>(null);
  const [overflowing, setOverflowing] = useState(false);

  const syncOverflow = useCallback(() => {
    const el = bodyRef.current;
    if (!el || !maxHeight) {
      setOverflowing(false);
      return;
    }
    setOverflowing(el.scrollHeight > el.clientHeight + 2);
  }, [maxHeight]);

  useEffect(() => {
    syncOverflow();
    const el = bodyRef.current;
    if (!el) return;
    const observer = new ResizeObserver(() => syncOverflow());
    observer.observe(el);
    return () => observer.disconnect();
  }, [props.value, syncOverflow]);

  const scrollToBottom = () => {
    const el = bodyRef.current;
    if (!el) return;
    el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
  };

  return (
    <div
      className={[
        "code-block",
        flat ? "code-block--flat" : "",
        toolbar ? "code-block--toolbar" : "",
        lineNumbers ? "code-block--lined" : "",
        overflowing ? "code-block--overflow" : "",
      ]
        .filter(Boolean)
        .join(" ")}
      style={toolbar && maxHeight ? { maxHeight } : undefined}
    >
      {toolbar ? <CodeBlockToolbar value={props.value} language={props.language} /> : null}
      {!toolbar ? <CopyButton text={props.value} className="code-block__copy" /> : null}
      <div
        ref={bodyRef}
        className="code-block__body"
        style={!toolbar && maxHeight ? { maxHeight } : undefined}
      >
        <Suspense
          fallback={
            <pre className="code code--loading">
              <code>{props.value}</code>
            </pre>
          }
        >
          <Impl {...props} maxHeight={toolbar ? undefined : maxHeight} />
        </Suspense>
      </div>
      {toolbar && overflowing ? (
        <Tooltip label={t("codeBlock.scrollDown")}>
          <button
            className="code-block__scroll"
            type="button"
            onClick={scrollToBottom}
            aria-label={t("codeBlock.scrollDown")}
          >
            <ChevronDown size={14} />
          </button>
        </Tooltip>
      ) : null}
    </div>
  );
}
