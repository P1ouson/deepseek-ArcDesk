import { useEffect, useRef, type RefObject } from "react";

type DismissOverlayOptions = {
  mode?: "pointerdown" | "click";
  excludeRefs?: ReadonlyArray<RefObject<Element | null>>;
  excludeSelector?: string;
  closeOnEscape?: boolean;
  closeOnResize?: boolean;
};

/** Unified dismiss for floating UI — pointer or click, optional Escape/resize. */
export function useDismissOverlay(
  active: boolean,
  onDismiss: () => void,
  options: DismissOverlayOptions = {},
): void {
  const {
    mode = "pointerdown",
    excludeRefs = [],
    excludeSelector,
    closeOnEscape = mode === "click",
    closeOnResize = mode === "click",
  } = options;

  const onDismissRef = useRef(onDismiss);
  onDismissRef.current = onDismiss;
  const excludeRefsRef = useRef(excludeRefs);
  excludeRefsRef.current = excludeRefs;
  const excludeSelectorRef = useRef(excludeSelector);
  excludeSelectorRef.current = excludeSelector;

  useEffect(() => {
    if (!active) return;

    const shouldIgnoreTarget = (target: EventTarget | null) => {
      if (!(target instanceof Node)) return true;
      for (const ref of excludeRefsRef.current) {
        if (ref.current?.contains(target)) return true;
      }
      const selector = excludeSelectorRef.current;
      if (selector && target instanceof Element && target.closest(selector)) return true;
      return false;
    };

    const dismiss = () => onDismissRef.current();

    if (mode === "pointerdown") {
      const onPointerDown = (event: PointerEvent) => {
        if (shouldIgnoreTarget(event.target)) return;
        dismiss();
      };
      window.addEventListener("pointerdown", onPointerDown, true);
      return () => window.removeEventListener("pointerdown", onPointerDown, true);
    }

    const onClick = (event: MouseEvent) => {
      if (shouldIgnoreTarget(event.target)) return;
      dismiss();
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") dismiss();
    };

    window.addEventListener("click", onClick);
    if (closeOnResize) window.addEventListener("resize", dismiss);
    if (closeOnEscape) window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("click", onClick);
      if (closeOnResize) window.removeEventListener("resize", dismiss);
      if (closeOnEscape) window.removeEventListener("keydown", onKey);
    };
  }, [active, mode, closeOnEscape, closeOnResize]);
}
