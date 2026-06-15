import { Plus, SquareTerminal, X } from "lucide-react";
import { useT } from "../lib/i18n";
import { TerminalView } from "./TerminalView";
import type { TerminalTab } from "./TerminalPanel";

export interface PreviewTerminalPaneProps {
  tabs: TerminalTab[];
  activeId: string;
  onActiveChange: (id: string) => void;
  onNewTerminal: () => void;
  onCloseTab: (id: string, index: number) => void;
  /** When false, pane stays mounted but xterm pauses layout/focus until shown again. */
  visible?: boolean;
  hideTabBar?: boolean;
}

export function PreviewTerminalPane({
  tabs,
  activeId,
  onActiveChange,
  onNewTerminal,
  onCloseTab,
  visible = true,
  hideTabBar = false,
}: PreviewTerminalPaneProps) {
  const t = useT();

  if (tabs.length === 0) {
    return (
      <div className="unified-preview__empty unified-preview__empty--terminal">
        <SquareTerminal size={20} strokeWidth={1.5} />
        <p>{t("previewHub.terminalEmpty")}</p>
        <button type="button" className="unified-preview__cta" onClick={onNewTerminal}>
          {t("terminal.new")}
        </button>
      </div>
    );
  }

  return (
    <div className="preview-terminal-pane">
      {!hideTabBar ? (
      <div className="preview-terminal-pane__tabs" role="tablist" aria-label={t("terminal.tabs")}>
        {tabs.map((tab, index) => (
          <div
            key={tab.clientKey}
            className={`preview-terminal-pane__tab${tab.id === activeId ? " preview-terminal-pane__tab--active" : ""}`}
            role="presentation"
          >
            <button
              type="button"
              role="tab"
              aria-selected={tab.id === activeId}
              className="preview-terminal-pane__tab-main"
              onClick={() => onActiveChange(tab.id)}
            >
              <SquareTerminal size={12} />
              <span>{tab.title}</span>
            </button>
            <button
              type="button"
              className="preview-terminal-pane__tab-close"
              aria-label={t("terminal.closeTab", { title: tab.title })}
              onClick={() => onCloseTab(tab.id, index)}
            >
              <X size={12} />
            </button>
          </div>
        ))}
        <button type="button" className="preview-terminal-pane__new" onClick={onNewTerminal} aria-label={t("terminal.new")}>
          <Plus size={14} />
        </button>
      </div>
      ) : null}
      <div className="preview-terminal-pane__viewport">
        {tabs.map((tab) => (
          <div key={tab.clientKey} className="preview-terminal-pane__session" hidden={tab.id !== activeId}>
            <TerminalView
              sessionId={tab.id}
              active={visible && tab.id === activeId}
              shell={tab.shell}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
