import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { installGlobalCrashHandlers } from "./lib/crash";
import { LocaleProvider } from "./lib/i18n";
import { initTextSize } from "./lib/textSize";
import { initTheme } from "./lib/theme";
import "./styles.css";

// Apply the saved appearance before the first paint so the webview does not
// flash the wrong theme while React boots.
initTheme();
initTextSize();

function prewarmFontFallbacks() {
  const span = document.createElement("span");
  span.style.cssText = "position:absolute;visibility:hidden;font-size:1px;pointer-events:none";
  span.textContent = "中文 日本語 русский язык العربية 0123456789 ✓ ✦ ∑ ∞";
  document.body.appendChild(span);
  void span.offsetHeight;
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      span.remove();
    });
  });
}

prewarmFontFallbacks();
installGlobalCrashHandlers();

// In the Wails shell, suppress the default webview context menu so accidental
// back/reload actions do not interrupt the session.
if (typeof window !== "undefined" && window.runtime) {
  window.addEventListener("contextmenu", (e) => {
    const target = e.target as HTMLElement | null;
    if (!target?.closest("input, textarea")) e.preventDefault();
  });
}

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");

createRoot(root).render(
  <StrictMode>
    <ErrorBoundary>
      <LocaleProvider>
        <App />
      </LocaleProvider>
    </ErrorBoundary>
  </StrictMode>,
);
