import { useEffect, useState } from "react";
import { afterLayoutFrame } from "./motion/afterLayoutFrame";
import { MOTION_DURATION_NORMAL_MS } from "./motion/constants";
import { motionDurationMs } from "./motion/prefersReducedMotion";

export type MountTransitionStatus = "closed" | "opening" | "open" | "closing";

/** Mount/unmount overlay UI with enter/exit timing and reduced-motion support. */
export function useMountTransition(open: boolean, durationMs = MOTION_DURATION_NORMAL_MS) {
  const [mounted, setMounted] = useState(open);
  const [status, setStatus] = useState<MountTransitionStatus>(open ? "open" : "closed");
  const duration = motionDurationMs(durationMs);

  useEffect(() => {
    if (open) {
      setMounted(true);
      setStatus("opening");
      return afterLayoutFrame(() => setStatus("open"));
    }
    if (!mounted) return;
    setStatus("closing");
    if (duration <= 0) {
      setMounted(false);
      setStatus("closed");
      return;
    }
    const timer = window.setTimeout(() => {
      setMounted(false);
      setStatus("closed");
    }, duration);
    return () => window.clearTimeout(timer);
  }, [open, mounted, duration]);

  return { mounted, status, duration };
}
