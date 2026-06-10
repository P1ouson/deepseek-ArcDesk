import type { ReactNode } from "react";

/** Matches `--duration-normal` in design-system.css */
export const MOTION_UNFOLD_MS = 220;

type MotionUnfoldProps = {
  open: boolean;
  children: ReactNode;
  className?: string;
  innerClassName?: string;
};

/** Vertical unfold — same grid-row animation as extensions skill sources. */
export function MotionUnfold({ open, children, className, innerClassName }: MotionUnfoldProps) {
  const rootClass = ["motion-unfold", open && "motion-unfold--open", className].filter(Boolean).join(" ");
  const innerClass = ["motion-unfold__inner", innerClassName].filter(Boolean).join(" ");
  return (
    <div className={rootClass}>
      <div className={innerClass}>{children}</div>
    </div>
  );
}
