import { useEffect, useRef, type RefObject } from "react";

type DismissOptions = {
  excludeRefs?: ReadonlyArray<RefObject<Element | null>>;
  excludeSelector?: string;
};

/** Closes floating UI when the user presses outside excluded anchors/menus. */
export function useDismissOnOutsidePointerDown(
  active: boolean,
  onDismiss: () => void,
  options: DismissOptions = {},
): void {
  const onDismissRef = useRef(onDismiss);
  onDismissRef.current = onDismiss;
  const excludeRefsRef = useRef(options.excludeRefs ?? []);
  excludeRefsRef.current = options.excludeRefs ?? [];
  const excludeSelectorRef = useRef(options.excludeSelector);
  excludeSelectorRef.current = options.excludeSelector;

  useEffect(() => {
    if (!active) return;
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      for (const ref of excludeRefsRef.current) {
        if (ref.current?.contains(target)) return;
      }
      const excludeSelector = excludeSelectorRef.current;
      if (excludeSelector && target instanceof Element && target.closest(excludeSelector)) return;
      onDismissRef.current();
    };
    window.addEventListener("pointerdown", onPointerDown, true);
    return () => window.removeEventListener("pointerdown", onPointerDown, true);
  }, [active]);
}
