import { useCallback, useEffect, useRef, useState } from "react";
import type { KeyboardEvent, PointerEvent as ReactPointerEvent } from "react";
import { Plus, SquareTerminal, X } from "lucide-react";
import { useT } from "../lib/i18n";
import { shortCwd } from "../lib/workspaceFilePreview";
import { TerminalView } from "./TerminalView";

const TERMINAL_PANEL_DEFAULT_HEIGHT = 260;
const TERMINAL_PANEL_MIN_HEIGHT = 160;
const TERMINAL_PANEL_MAX_HEIGHT = 560;

export function clampTerminalPanelHeight(height: number): number {
  return Math.min(TERMINAL_PANEL_MAX_HEIGHT, Math.max(TERMINAL_PANEL_MIN_HEIGHT, Math.round(height)));
}

export { TERMINAL_PANEL_DEFAULT_HEIGHT };

export interface TerminalTab {
  id: string;
  /** Stable React key — independent of backend session id and tab order */
  clientKey: string;
  title: string;
  shell?: string;
}

export interface BottomTerminalPanelProps {
  height: number;
  cwd?: string;
  tabs: TerminalTab[];
  activeId: string;
  onActiveChange: (id: string) => void;
  onNewTerminal: () => void;
  onCloseTab: (id: string, index: number) => void;
  onClosePanel: () => void;
  onResizeHeight: (height: number) => void;
}

export function BottomTerminalPanel({
  height,
  cwd,
  tabs,
  activeId,
  onActiveChange,
  onNewTerminal,
  onCloseTab,
  onClosePanel,
  onResizeHeight,
}: BottomTerminalPanelProps) {
  const t = useT();
  const [resizing, setResizing] = useState(false);
  const tabsRef = useRef<HTMLDivElement>(null);

  const startResize = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      event.preventDefault();
      setResizing(true);
      const startY = event.clientY;
      const startHeight = height;
      let nextHeight = startHeight;
      const onMove = (moveEvent: PointerEvent) => {
        nextHeight = clampTerminalPanelHeight(startHeight - (moveEvent.clientY - startY));
        onResizeHeight(nextHeight);
      };
      const onDone = () => {
        onResizeHeight(nextHeight);
        setResizing(false);
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onDone);
        window.removeEventListener("pointercancel", onDone);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
      };
      document.body.style.cursor = "row-resize";
      document.body.style.userSelect = "none";
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onDone);
      window.addEventListener("pointercancel", onDone);
    },
    [height, onResizeHeight],
  );

  const onPanelKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      onClosePanel();
    }
  };

  useEffect(() => {
    const el = tabsRef.current;
    if (!el) return;
    const activeBtn = el.querySelector<HTMLButtonElement>(`[data-terminal-id="${activeId}"]`);
    activeBtn?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [activeId, tabs.length]);

  return (
    <section
      className={`terminal-panel${resizing ? " terminal-panel--resizing" : ""}`}
      style={{ height: clampTerminalPanelHeight(height) }}
      aria-label={t("terminal.title")}
      onKeyDown={onPanelKeyDown}
    >
      <button
        type="button"
        className="terminal-panel__resizer wails-no-drag"
        aria-label={t("terminal.resize")}
        onPointerDown={startResize}
      />
      <header className="terminal-panel__head terminal-panel__head--tabs">
        <div className="terminal-panel__tabs" ref={tabsRef} role="tablist" aria-label={t("terminal.tabs")}>
          {tabs.map((tab, index) => (
            <div
              key={tab.clientKey}
              className={`terminal-panel__tab${tab.id === activeId ? " terminal-panel__tab--active" : ""}`}
              role="presentation"
            >
              <button
                type="button"
                role="tab"
                data-terminal-id={tab.id}
                aria-selected={tab.id === activeId}
                className="terminal-panel__tab-main"
                onClick={() => onActiveChange(tab.id)}
              >
                <SquareTerminal size={12} />
                <span>{tab.title}</span>
              </button>
              {tabs.length > 1 && (
              <button
                type="button"
                className="terminal-panel__tab-close"
                aria-label={t("terminal.closeTab", { title: tab.title })}
                onClick={(event) => {
                  event.stopPropagation();
                  onCloseTab(tab.id, index);
                }}
              >
                  <X size={12} />
                </button>
              )}
            </div>
          ))}
        </div>
        <div className="terminal-panel__actions">
          {cwd && <span className="terminal-panel__cwd">{shortCwd(cwd)}</span>}
          <button type="button" className="terminal-panel__action" onClick={onNewTerminal} aria-label={t("terminal.new")}>
            <Plus size={14} />
          </button>
          <button type="button" className="terminal-panel__close" onClick={onClosePanel} aria-label={t("terminal.close")}>
            <X size={14} />
          </button>
        </div>
      </header>
      <div className="terminal-panel__body terminal-panel__body--xterm">
        {tabs.map((tab) => (
          <TerminalView key={tab.clientKey} sessionId={tab.id} active={tab.id === activeId} shell={tab.shell} />
        ))}
      </div>
    </section>
  );
}
