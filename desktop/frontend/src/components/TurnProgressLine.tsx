import { useEffect, useMemo, useState } from "react";
import { deriveTurnProgress } from "../lib/turnProgress";
import { useT } from "../lib/i18n";
import type { Item } from "../lib/useController";

export function TurnProgressLine({
  running,
  turnStartAt,
  items,
}: {
  running: boolean;
  turnStartAt: number;
  items: Item[];
}) {
  const t = useT();
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!running) return;
    const id = window.setInterval(() => setNow(Date.now()), 500);
    return () => window.clearInterval(id);
  }, [running]);

  const progress = useMemo(
    () => deriveTurnProgress({ running, turnStartAt, items, now, t }),
    [running, turnStartAt, items, now, t],
  );

  if (!progress) return null;

  return (
    <div className="turn-progress" role="status" aria-live="polite">
      <span className="turn-progress__label">{progress.label}</span>
      {progress.detail ? <span className="turn-progress__detail">{progress.detail}</span> : null}
      {progress.showSlowHint ? (
        <span className="turn-progress__slow">{t("turnProgress.slowHint")}</span>
      ) : null}
    </div>
  );
}
