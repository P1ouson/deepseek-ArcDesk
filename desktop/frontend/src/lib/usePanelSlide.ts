import { useCallback, useEffect, useState } from "react";
import { afterLayoutFrame } from "./motion/afterLayoutFrame";
import { MOTION_DURATION_NORMAL_MS } from "./motion/constants";
import { motionDurationMs } from "./motion/prefersReducedMotion";

/** Panel slide open/close with deferred unmount (dock, terminal, popovers). */
export function usePanelSlide(open: boolean, durationMs = MOTION_DURATION_NORMAL_MS) {
  const [shown, setShown] = useState(open);
  const [animating, setAnimating] = useState(false);
  const duration = motionDurationMs(durationMs);

  useEffect(() => {
    if (open) {
      setShown(true);
      setAnimating(true);
      return afterLayoutFrame(() => setAnimating(false));
    }
    if (!shown) return;
    setAnimating(true);
    if (duration <= 0) {
      setShown(false);
      setAnimating(false);
      return;
    }
    const timer = window.setTimeout(() => {
      setShown(false);
      setAnimating(false);
    }, duration);
    return () => window.clearTimeout(timer);
  }, [open, shown, duration]);

  const visible = open || shown;

  return { visible, shown, animating, duration };
}

export function usePanelSlideMeasure(onMeasure: () => void) {
  return useCallback(() => afterLayoutFrame(onMeasure), [onMeasure]);
}
