import { useCallback, useState, type ComponentProps } from "react";
import { Composer, type ComposerSendState } from "./Composer";
import { ComposerModeBar, ComposerModeToggle } from "./ComposerModeBar";
import { ComposerDockFooter, type ComposerDockFooterProps } from "./ComposerDockFooter";
import { ComposerSendButton } from "./ComposerSendButton";
export type FloatingComposerProps = ComponentProps<typeof Composer> & ComposerDockFooterProps;

export { ComposerModeBar, ComposerModeToggle };

export function FloatingComposer({
  context,
  usage,
  balance,
  sessionCost,
  sessionCurrency,
  terminalCount,
  ...composerProps
}: FloatingComposerProps) {
  const planActive = composerProps.mode === "plan";
  const [sendState, setSendState] = useState<ComposerSendState | null>(null);
  const handleSendState = useCallback((state: ComposerSendState | null) => {
    setSendState(state);
  }, []);

  return (
    <div className="composer-dock">
      <div className="composer-shell-row">
        <div className={`composer-shell${planActive ? " composer-shell--plan" : ""}`}>
          <Composer {...composerProps} hideModeBar sendExternally onSendState={handleSendState} />
        </div>
        {composerProps.running || sendState ? (
          <ComposerSendButton
            disabled={!composerProps.running && (sendState?.disabled ?? true)}
            running={composerProps.running}
            onClick={sendState?.onSend ?? (() => {})}
            onCancel={() => {
              composerProps.onCancel();
            }}
          />
        ) : null}
      </div>
      <ComposerDockFooter
        context={context}
        usage={usage}
        balance={balance}
        sessionCost={sessionCost}
        sessionCurrency={sessionCurrency}
        terminalCount={terminalCount}
      />
    </div>
  );
}

export function SlashSuggestionPills({
  visible,
  onPick,
}: {
  visible: boolean;
  onPick: (cmd: string) => void;
}) {
  if (!visible) return null;
  const cmds = ["/plan", "/review", "/btw", "/goal", "/sdd"];
  return (
    <div className="composer-shell__cmdrow">
      {cmds.map((cmd) => (
        <button key={cmd} type="button" className="composer-shell__slash-pill" onClick={() => onPick(`${cmd} `)}>
          {cmd}
        </button>
      ))}
    </div>
  );
}
