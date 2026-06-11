import type { PointerEvent as ReactPointerEvent } from "react";

export type PointerResizeOptions = {
  event: ReactPointerEvent<HTMLElement>;
  cursor: string;
  onStart?: () => void;
  onMove: (event: PointerEvent) => void;
  onCommit: () => void;
};

/**
 * Column/row resize via window-level pointer listeners.
 * WebView2 often drops pointermove on narrow separator buttons; window listeners are reliable.
 */
export function attachPointerResize({ event, cursor, onStart, onMove, onCommit }: PointerResizeOptions): void {
  if (event.button !== 0) return;
  event.preventDefault();
  event.stopPropagation();

  onStart?.();

  const pointerId = event.pointerId;
  document.body.style.cursor = cursor;
  document.body.style.userSelect = "none";

  const handleMove = (moveEvent: PointerEvent) => {
    if (moveEvent.pointerId !== pointerId) return;
    onMove(moveEvent);
  };

  const handleDone = (doneEvent: PointerEvent) => {
    if (doneEvent.pointerId !== pointerId) return;
    document.body.style.cursor = "";
    document.body.style.userSelect = "";
    window.removeEventListener("pointermove", handleMove);
    window.removeEventListener("pointerup", handleDone);
    window.removeEventListener("pointercancel", handleDone);
    onCommit();
  };

  window.addEventListener("pointermove", handleMove);
  window.addEventListener("pointerup", handleDone);
  window.addEventListener("pointercancel", handleDone);
}
