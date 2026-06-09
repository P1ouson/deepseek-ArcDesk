import { useCallback, useEffect, useState } from "react";
import { Loader2, Plus, Trash2 } from "lucide-react";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { ClawCallbackInfo, ClawChannel } from "../lib/types";
import { SettingsActionButton, SettingsBlock, SettingsSaveChip } from "./settingsPrimitives";

function emptyChannel(): ClawChannel {
  return {
    id: `wecom-${Date.now()}`,
    name: "",
    type: "wechat",
    enabled: true,
    persona: "",
    model: "",
    workspaceRoot: "",
    webhookURL: "",
    wecomCorpId: "",
    wecomAgentId: "",
    wecomSecret: "",
    wecomToken: "",
    wecomEncodingAESKey: "",
  };
}

export function ClawChannelsSettings({ busy, workspaceRoot }: { busy: boolean; workspaceRoot?: string }) {
  const t = useT();
  const [channels, setChannels] = useState<ClawChannel[]>([]);
  const [callback, setCallback] = useState<ClawCallbackInfo | null>(null);
  const [draft, setDraft] = useState<ClawChannel | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [testing, setTesting] = useState(false);

  const reload = useCallback(async () => {
    const list = await app.GetClawChannels().catch(() => [] as ClawChannel[]);
    setChannels(list);
    const channelId = list[0]?.id ?? "";
    const info = channelId ? await app.GetClawCallbackInfo(channelId).catch(() => null) : null;
    setCallback(info);
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const saveDraft = async () => {
    if (!draft) return;
    setErr(null);
    try {
      await app.SaveClawChannel({
        ...draft,
        workspaceRoot: draft.workspaceRoot || workspaceRoot || "",
      });
      setDraft(null);
      setNotice(t("claw.saved"));
      await reload();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    }
  };

  const removeChannel = async (id: string) => {
    setErr(null);
    try {
      await app.DeleteClawChannel(id);
      if (draft?.id === id) setDraft(null);
      await reload();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    }
  };

  const testChannel = async () => {
    if (!draft) return;
    setTesting(true);
    setErr(null);
    try {
      const msg = await app.TestClawWeComChannel(draft);
      setNotice(msg || t("claw.testOk"));
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setTesting(false);
    }
  };

  return (
    <SettingsBlock title={t("claw.title")} hint={t("claw.hint")}>
      {err ? <p className="settings-block__note settings-block__note--warn">{err}</p> : null}
      {notice ? <p className="settings-block__note">{notice}</p> : null}
      {callback?.url ? (
        <p className="settings-block__note">
          {t("claw.callbackUrl")}: <code>{callback.url}</code>
        </p>
      ) : null}

      <div className="settings-block__stack">
        {channels.map((ch) => (
          <div key={ch.id} className="settings-claw-row">
            <button type="button" className="settings-claw-row__main" disabled={busy} onClick={() => setDraft(ch)}>
              <strong>{ch.name || ch.id}</strong>
              <span>{ch.enabled ? t("claw.enabled") : t("claw.disabled")}</span>
            </button>
            <button type="button" className="settings-claw-row__delete" disabled={busy} onClick={() => void removeChannel(ch.id)}>
              <Trash2 size={14} />
            </button>
          </div>
        ))}
        <SettingsActionButton
          primary={false}
          disabled={busy}
          onClick={() => setDraft(emptyChannel())}
        >
          <Plus size={14} /> {t("claw.add")}
        </SettingsActionButton>
      </div>

      {draft ? (
        <div className="settings-block__form settings-claw-form">
          <div className="set-row set-row--stack">
            <label className="set-label">{t("claw.name")}</label>
            <input className="mem-input" value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
          </div>
          <div className="set-row set-row--stack">
            <label className="set-label">{t("claw.corpId")}</label>
            <input className="mem-input" value={draft.wecomCorpId ?? ""} onChange={(e) => setDraft({ ...draft, wecomCorpId: e.target.value })} />
          </div>
          <div className="set-row set-row--stack">
            <label className="set-label">{t("claw.agentId")}</label>
            <input className="mem-input" value={draft.wecomAgentId ?? ""} onChange={(e) => setDraft({ ...draft, wecomAgentId: e.target.value })} />
          </div>
          <div className="set-row set-row--stack">
            <label className="set-label">{t("claw.secret")}</label>
            <input className="mem-input" type="password" value={draft.wecomSecret ?? ""} onChange={(e) => setDraft({ ...draft, wecomSecret: e.target.value })} />
          </div>
          <div className="set-row set-row--stack">
            <label className="set-label">{t("claw.token")}</label>
            <input className="mem-input" value={draft.wecomToken ?? ""} onChange={(e) => setDraft({ ...draft, wecomToken: e.target.value })} />
          </div>
          <div className="set-row set-row--stack">
            <label className="set-label">{t("claw.aesKey")}</label>
            <input className="mem-input" value={draft.wecomEncodingAESKey ?? ""} onChange={(e) => setDraft({ ...draft, wecomEncodingAESKey: e.target.value })} />
          </div>
          <label className="set-check">
            <input type="checkbox" checked={draft.enabled} onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })} />
            {t("claw.enabled")}
          </label>
          <div className="settings-claw-form__actions">
            <SettingsActionButton primary={false} disabled={busy || testing} onClick={() => void testChannel()}>
              {testing ? <Loader2 size={14} className="dock-panel__spin" /> : t("claw.test")}
            </SettingsActionButton>
            <SettingsSaveChip disabled={busy} ready onClick={() => void saveDraft()}>
              {t("common.save")}
            </SettingsSaveChip>
            <SettingsActionButton primary={false} disabled={busy} onClick={() => setDraft(null)}>
              {t("common.cancel")}
            </SettingsActionButton>
          </div>
        </div>
      ) : null}
    </SettingsBlock>
  );
}
