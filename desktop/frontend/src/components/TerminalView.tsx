import { useCallback, useEffect, useRef } from "react";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import { useT } from "../lib/i18n";
import {
  closeTerminal,
  onTerminalExit,
  onTerminalOutput,
  resizeTerminal,
  writeTerminal,
} from "../lib/terminalBridge";
import "@xterm/xterm/css/xterm.css";

export interface TerminalViewProps {
  sessionId: string;
  active: boolean;
  shell?: string;
}

export function TerminalView({ sessionId, active, shell }: TerminalViewProps) {
  const t = useT();
  const hostRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);

  const fitTerminal = useCallback(() => {
    if (!active) return;
    const term = termRef.current;
    const fit = fitRef.current;
    if (!term || !fit) return;
    fit.fit();
    resizeTerminal(sessionId, term.cols, term.rows);
  }, [active, sessionId]);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: "Consolas, 'Cascadia Mono', 'Courier New', monospace",
      theme: {
        background: "#1a1a2e",
        foreground: "#e8e4df",
        cursor: "#c45c26",
      },
      scrollback: 5000,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    termRef.current = term;
    fitRef.current = fit;

    if (shell) {
      term.writeln(`\x1b[90m${shell}\x1b[0m`);
    }

    const onData = term.onData((data) => writeTerminal(sessionId, data));
    const offOutput = onTerminalOutput(sessionId, (data) => term.write(data));
    const offExit = onTerminalExit(sessionId, (code) => {
      term.writeln(`\r\n\x1b[90m[${t("terminal.exited", { code: String(code) })}]\x1b[0m`);
    });

    requestAnimationFrame(() => fitTerminal());

    const ro = new ResizeObserver(() => fitTerminal());
    ro.observe(host);

    return () => {
      ro.disconnect();
      onData.dispose();
      offOutput();
      offExit();
      closeTerminal(sessionId);
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
    };
  }, [fitTerminal, sessionId, shell, t]);

  useEffect(() => {
    if (!active) return;
    requestAnimationFrame(() => fitTerminal());
  }, [active, fitTerminal]);

  return (
    <div className={`terminal-view${active ? " terminal-view--active" : ""}`} aria-hidden={!active}>
      <div className="terminal-panel__xterm-host" ref={hostRef} />
    </div>
  );
}
