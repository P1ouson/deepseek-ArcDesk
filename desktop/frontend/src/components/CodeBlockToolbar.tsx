import { useState } from "react";
import { Check, Copy, Download, Play } from "lucide-react";
import { useT } from "../lib/i18n";
import { Tooltip } from "./Tooltip";
import {
  canRunCodeBlock,
  codeBlockLanguageLabel,
  downloadCodeBlock,
  runCodeBlock,
} from "../lib/codeBlockActions";

export function CodeBlockToolbar({
  value,
  language,
}: {
  value: string;
  language?: string;
}) {
  const t = useT();
  const [copied, setCopied] = useState(false);
  const label = codeBlockLanguageLabel(language);
  const showRun = canRunCodeBlock(language);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1200);
    } catch {
      /* clipboard unavailable */
    }
  };

  return (
    <div className="code-block__header">
      <span className="code-block__lang">{label}</span>
      <div className="code-block__actions">
        <Tooltip label={copied ? t("msg.copied") : t("msg.copy")}>
          <button className="code-block__action" type="button" onClick={copy} aria-label={t("msg.copy")}>
            {copied ? <Check size={14} /> : <Copy size={14} />}
            <span>{copied ? t("msg.copied") : t("msg.copy")}</span>
          </button>
        </Tooltip>
        <Tooltip label={t("codeBlock.download")}>
          <button
            className="code-block__action"
            type="button"
            onClick={() => downloadCodeBlock(value, language)}
            aria-label={t("codeBlock.download")}
          >
            <Download size={14} />
            <span>{t("codeBlock.download")}</span>
          </button>
        </Tooltip>
        {showRun ? (
          <>
            <span className="code-block__sep" aria-hidden="true" />
            <Tooltip label={t("codeBlock.run")}>
              <button
                className="code-block__action code-block__action--run"
                type="button"
                onClick={() => runCodeBlock(value, language)}
                aria-label={t("codeBlock.run")}
              >
                <Play size={14} />
                <span>{t("codeBlock.run")}</span>
              </button>
            </Tooltip>
          </>
        ) : null}
      </div>
    </div>
  );
}
