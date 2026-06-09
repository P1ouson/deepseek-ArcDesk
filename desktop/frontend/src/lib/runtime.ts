/** True when the UI is hosted inside a Wails desktop shell (not browser dev mock). */
export function isWailsRuntime(): boolean {
  return typeof window !== "undefined" && Boolean(window.go?.main?.App);
}
