/** Ensures only one studio-style dropdown is open at a time. */

let activeClose: (() => void) | null = null;

export function openStudioSelect(close: () => void): void {
  if (activeClose && activeClose !== close) {
    activeClose();
  }
  activeClose = close;
}

export function closeStudioSelect(close: () => void): void {
  if (activeClose === close) {
    activeClose = null;
  }
}
