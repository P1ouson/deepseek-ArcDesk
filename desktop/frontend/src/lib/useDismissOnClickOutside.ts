import { useEffect } from "react";

export function useDismissOnClickOutside(active: boolean, onDismiss: () => void) {
  useEffect(() => {
    if (!active) return;
    const close = () => onDismiss();
    const onKey = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    window.addEventListener("click", close);
    window.addEventListener("resize", close);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("resize", close);
      window.removeEventListener("keydown", onKey);
    };
  }, [active, onDismiss]);
}
