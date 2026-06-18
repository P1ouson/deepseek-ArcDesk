import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { useT } from "../lib/i18n";
import { Tooltip } from "./Tooltip";

// CopyButton copies `text` to the clipboard on click and briefly flips to a check.
// navigator.clipboard works in the webview under the click's user gesture; a
// failure is swallowed (nothing to copy to).
export function CopyButton({
  text,
  className,
  label,
  variant = "default",
}: {
  text: string;
  className?: string;
  label?: string;
  /** `tool` matches message action chips (copy / rewind). */
  variant?: "default" | "tool";
}) {
  const t = useT();
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    const write = async () => {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
        return;
      }
      throw new Error("clipboard unavailable");
    };
    try {
      await write();
    } catch {
      try {
        const ta = document.createElement("textarea");
        ta.value = text;
        ta.setAttribute("readonly", "");
        ta.style.position = "fixed";
        ta.style.left = "-9999px";
        document.body.appendChild(ta);
        ta.select();
        const ok = document.execCommand("copy");
        document.body.removeChild(ta);
        if (!ok) return;
      } catch {
        return;
      }
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 1200);
  };
  const iconSize = variant === "tool" ? 15 : 13;
  const btnClass = variant === "tool" ? "msg-tool-btn motion-surface" : "copybtn";

  return (
    <Tooltip label={copied ? t("msg.copied") : t("msg.copy")}>
      <button
        className={`${btnClass} ${className ?? ""}`.trim()}
        onClick={copy}
        aria-label={t("msg.copy")}
        type="button"
      >
        {copied ? <Check size={iconSize} /> : <Copy size={iconSize} />}
        {label && variant !== "tool" ? (
          <span className="copybtn__label">{copied ? t("msg.copied") : label}</span>
        ) : null}
      </button>
    </Tooltip>
  );
}
