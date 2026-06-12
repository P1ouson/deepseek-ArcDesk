import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, ReactNode, RefObject } from "react";
import { createPortal } from "react-dom";
import { clamp } from "../lib/clamp";
import { useDismissOverlay } from "../lib/useDismissOverlay";
import { useMountTransition } from "../lib/useMountTransition";

type PopoverPosition = {
  left: number;
  top: number;
};

const EDGE_GAP = 8;
const DEFAULT_OFFSET = 8;

function samePosition(a: PopoverPosition | null, b: PopoverPosition): boolean {
  return !!a && Math.abs(a.left - b.left) < 0.5 && Math.abs(a.top - b.top) < 0.5;
}

function calculatePosition(
  anchor: DOMRect,
  menu: DOMRect,
  align: "start" | "end",
  offset: number,
  placement: "auto" | "bottom" | "top",
): PopoverPosition {
  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;
  const aboveTop = anchor.top - menu.height - offset;
  const belowTop = anchor.bottom + offset;
  const fitsAbove = aboveTop >= EDGE_GAP;
  const fitsBelow = belowTop + menu.height <= viewportHeight - EDGE_GAP;
  const anchorNearBottom = anchor.bottom > viewportHeight * 0.55;

  let top = belowTop;
  if (placement === "top") {
    top = fitsAbove ? aboveTop : fitsBelow ? belowTop : aboveTop;
  } else if (placement === "bottom") {
    top = fitsBelow ? belowTop : fitsAbove ? aboveTop : belowTop;
  } else if (anchorNearBottom && fitsAbove) {
    top = aboveTop;
  } else if (!anchorNearBottom && fitsBelow) {
    top = belowTop;
  } else if (fitsAbove) {
    top = aboveTop;
  } else if (fitsBelow) {
    top = belowTop;
  } else {
    top = aboveTop >= belowTop ? aboveTop : belowTop;
  }

  let topClamped = clamp(top, EDGE_GAP, Math.max(EDGE_GAP, viewportHeight - menu.height - EDGE_GAP));
  const coversAnchor =
    topClamped + menu.height + offset > anchor.top && topClamped < anchor.bottom + offset;
  if (coversAnchor) {
    const above = anchor.top - menu.height - offset;
    const below = anchor.bottom + offset;
    if (above >= EDGE_GAP) topClamped = above;
    else if (below + menu.height <= viewportHeight - EDGE_GAP) topClamped = below;
    else topClamped = Math.max(EDGE_GAP, above);
  }

  const rawLeft = align === "end" ? anchor.right - menu.width : anchor.left;
  const left = clamp(rawLeft, EDGE_GAP, Math.max(EDGE_GAP, viewportWidth - menu.width - EDGE_GAP));
  return { left, top: topClamped };
}

export function AnchoredPopover({
  open,
  anchorRef,
  onClose,
  className,
  children,
  align = "start",
  offset = DEFAULT_OFFSET,
  placement = "auto",
  style,
}: {
  open: boolean;
  anchorRef: RefObject<HTMLElement>;
  onClose: () => void;
  className: string;
  children: ReactNode;
  align?: "start" | "end";
  offset?: number;
  placement?: "auto" | "bottom" | "top";
  style?: CSSProperties;
}) {
  const menuRef = useRef<HTMLDivElement>(null);
  const { mounted } = useMountTransition(open);
  const [unfolded, setUnfolded] = useState(false);
  const [position, setPosition] = useState<PopoverPosition | null>(null);

  useEffect(() => {
    if (!open) setUnfolded(false);
  }, [open]);

  useLayoutEffect(() => {
    if (!mounted || !open) return;
    setUnfolded(false);
    let frame2 = 0;
    const frame1 = requestAnimationFrame(() => {
      frame2 = requestAnimationFrame(() => setUnfolded(true));
    });
    return () => {
      cancelAnimationFrame(frame1);
      if (frame2) cancelAnimationFrame(frame2);
    };
  }, [mounted, open]);

  useDismissOverlay(mounted && open, onClose, {
    excludeRefs: [anchorRef, menuRef],
  });

  const updatePosition = useCallback(() => {
    const anchor = anchorRef.current?.getBoundingClientRect();
    const menu = menuRef.current?.getBoundingClientRect();
    if (!anchor || !menu || menu.height <= 0) return;
    const next = calculatePosition(anchor, menu, align, offset, placement);
    setPosition((current) => (samePosition(current, next) ? current : next));
  }, [align, anchorRef, offset, placement]);

  useLayoutEffect(() => {
    if (!mounted) {
      setPosition(null);
      return;
    }
    updatePosition();
  }, [mounted, updatePosition, unfolded]);

  useEffect(() => {
    if (!mounted) return;
    const menu = menuRef.current;
    if (!menu || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => updatePosition());
    observer.observe(menu);
    return () => observer.disconnect();
  }, [mounted, updatePosition]);

  useEffect(() => {
    if (!mounted) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    const closeOnResize = () => onClose();
    const closeOnScroll = (event: Event) => {
      const target = event.target;
      const menu = menuRef.current;
      if (menu && target instanceof Node && menu.contains(target)) {
        return;
      }
      onClose();
    };
    window.addEventListener("keydown", closeOnEscape);
    window.addEventListener("resize", closeOnResize);
    window.addEventListener("scroll", closeOnScroll, true);
    return () => {
      window.removeEventListener("keydown", closeOnEscape);
      window.removeEventListener("resize", closeOnResize);
      window.removeEventListener("scroll", closeOnScroll, true);
    };
  }, [mounted, onClose]);

  if (!mounted) return null;

  return createPortal(
    <>
      <div className="anchored-popover__backdrop" onMouseDown={onClose} />
      <div
        ref={menuRef}
        className={`anchored-popover motion-unfold${unfolded ? " motion-unfold--open" : ""}${className ? ` ${className}` : ""}`}
        style={{
          ...style,
          left: position?.left ?? -9999,
          top: position?.top ?? -9999,
          visibility: position ? "visible" : "hidden",
        }}
        onMouseDown={(event) => {
          event.stopPropagation();
        }}
        onClick={(event) => {
          event.stopPropagation();
        }}
        onWheel={(event) => {
          event.stopPropagation();
        }}
      >
        <div className="motion-unfold__inner">{children}</div>
      </div>
    </>,
    document.body,
  );
}
