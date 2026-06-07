import { useCallback, useEffect, useMemo, useState } from "react";
import { MessageCircle, Plus, Trash2 } from "lucide-react";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { ClawChannel, ClawMessage } from "../lib/types";

const CHANNEL_TYPES: ClawChannel["type"][] = ["feishu", "lark", "wechat", "webhook"];

function emptyChannel(): ClawChannel {
  return {
    id: "",
    name: "New channel",
    type: "feishu",
    enabled: true,
    persona: "Be concise and practical.",
    model: "deepseek-chat",
    workspaceRoot: "",
    webhookURL: "",
  };
}

function formatRelativeTime(ts?: number): string {
  if (!ts) return "just now";
  const delta = Math.max(0, Date.now() - ts);
  if (delta < 60_000) return `${Math.max(1, Math.round(delta / 1000))}s ago`;
  if (delta < 3_600_000) return `${Math.round(delta / 60_000)}m ago`;
  return `${Math.round(delta / 3_600_000)}h ago`;
}

export interface ConnectPhoneViewProps {
  workspaceRoot: string;
}

export function ConnectPhoneView({ workspaceRoot }: ConnectPhoneViewProps) {
  const t = useT();
  const [channels, setChannels] = useState<ClawChannel[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [draft, setDraft] = useState<ClawChannel>(emptyChannel());
  const [messages, setMessages] = useState<ClawMessage[]>([]);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const items = await app.GetClawChannels().catch(() => [] as ClawChannel[]);
    setChannels(items);
    return items;
  }, []);

  const reloadMessages = useCallback(async (channelId: string | null) => {
    if (!channelId) {
      setMessages([]);
      return;
    }
    const items = await app.GetClawMessages(channelId).catch(() => [] as ClawMessage[]);
    setMessages(items);
  }, []);

  useEffect(() => {
    void reload().then((items) => {
      if (items[0] && !selectedId) {
        setSelectedId(items[0].id);
      }
    });
  }, [reload, selectedId]);

  useEffect(() => {
    const selected = channels.find((channel) => channel.id === selectedId);
    if (selected) setDraft(selected);
    void reloadMessages(selectedId);
  }, [channels, selectedId, reloadMessages]);

  const selected = useMemo(() => channels.find((channel) => channel.id === selectedId), [channels, selectedId]);

  const lastActivity = useMemo(() => {
    const last = messages[messages.length - 1];
    return last?.createdAt;
  }, [messages]);

  const save = async () => {
    setBusy(true);
    setErr(null);
    try {
      const next: ClawChannel = {
        ...draft,
        id: draft.id || `ch-${Date.now()}`,
        workspaceRoot: draft.workspaceRoot || workspaceRoot,
      };
      await app.SaveClawChannel(next);
      setSelectedId(next.id);
      await reload();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const remove = async (id: string) => {
    setBusy(true);
    setErr(null);
    try {
      await app.DeleteClawChannel(id);
      if (selectedId === id) setSelectedId(null);
      await reload();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const sendMessage = async () => {
    const trimmed = message.trim();
    if (!trimmed || !selectedId) return;
    setBusy(true);
    setErr(null);
    try {
      await app.SendClawMessage(selectedId, trimmed);
      setMessage("");
      await reloadMessages(selectedId);
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const createChannel = () => {
    const next = { ...emptyChannel(), id: "", workspaceRoot };
    setDraft(next);
    setSelectedId(null);
  };

  return (
    <div className="connect-phone">
      <section className="connect-phone__column">
        <header className="connect-phone__section-head">
          <h3>{t("phone.channels")}</h3>
          <button type="button" onClick={createChannel}>
            <Plus size={14} />
          </button>
        </header>
        <div className="connect-phone__list">
          {channels.map((channel) => (
            <button
              key={channel.id}
              type="button"
              className={`connect-phone__channel${selectedId === channel.id ? " connect-phone__channel--active" : ""}`}
              onClick={() => setSelectedId(channel.id)}
            >
              <div>
                <strong>{channel.name}</strong>
                <p>{selectedId === channel.id && lastActivity ? formatRelativeTime(lastActivity) : t("phone.noActivity")}</p>
              </div>
              <span className={`connect-phone__dot connect-phone__dot--${channel.enabled ? "ok" : "muted"}`} />
            </button>
          ))}
        </div>
      </section>

      <section className="connect-phone__column connect-phone__column--center">
        <header className="connect-phone__section-head">
          <h3>
            <MessageCircle size={15} /> {t("phone.chat")}
          </h3>
          <span>{selected?.name ?? t("phone.noChannel")}</span>
        </header>
        <div className="connect-phone__chatlog">
          {!selectedId ? (
            <div className="connect-phone__empty">{t("phone.selectChannel")}</div>
          ) : messages.length ? (
            messages.map((entry) => (
              <article
                key={entry.id}
                className={`connect-phone__bubble connect-phone__bubble--${entry.outgoing ? "outgoing" : "incoming"}`}
              >
                {entry.text}
              </article>
            ))
          ) : (
            <div className="connect-phone__empty">{t("phone.noMessages")}</div>
          )}
        </div>
        <div className="connect-phone__composer">
          <textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder={t("phone.replyPlaceholder")}
            rows={2}
            disabled={!selectedId || busy}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                void sendMessage();
              }
            }}
          />
          <button type="button" disabled={!message.trim() || !selectedId || busy} onClick={() => void sendMessage()}>
            {t("phone.send")}
          </button>
        </div>
      </section>

      <section className="connect-phone__column">
        <header className="connect-phone__section-head">
          <h3>{t("phone.settings")}</h3>
        </header>
        {err ? <div className="connect-phone__banner connect-phone__banner--error">{err}</div> : null}
        <div className="connect-phone__form">
          <label>
            {t("phone.name")}
            <input value={draft.name} onChange={(e) => setDraft((v) => ({ ...v, name: e.target.value }))} />
          </label>
          <label>
            {t("phone.type")}
            <select value={draft.type} onChange={(e) => setDraft((v) => ({ ...v, type: e.target.value as ClawChannel["type"] }))}>
              {CHANNEL_TYPES.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </select>
          </label>
          <label>
            {t("phone.persona")}
            <textarea value={draft.persona} onChange={(e) => setDraft((v) => ({ ...v, persona: e.target.value }))} />
          </label>
          <label>
            {t("phone.model")}
            <input value={draft.model} onChange={(e) => setDraft((v) => ({ ...v, model: e.target.value }))} />
          </label>
          <label>
            {t("phone.workspaceRoot")}
            <input value={draft.workspaceRoot || workspaceRoot} onChange={(e) => setDraft((v) => ({ ...v, workspaceRoot: e.target.value }))} />
          </label>
          <label>
            {t("phone.webhook")}
            <input value={draft.webhookURL} onChange={(e) => setDraft((v) => ({ ...v, webhookURL: e.target.value }))} />
          </label>
          <label className="connect-phone__checkbox">
            <input
              type="checkbox"
              checked={draft.enabled}
              onChange={(e) => setDraft((v) => ({ ...v, enabled: e.target.checked }))}
            />
            {t("phone.enabled")}
          </label>
        </div>
        <div className="connect-phone__actions">
          <button type="button" disabled={busy} onClick={() => void save()}>
            {t("phone.save")}
          </button>
          {selected ? (
            <button type="button" className="connect-phone__danger" disabled={busy} onClick={() => void remove(selected.id)}>
              <Trash2 size={14} />
              {t("phone.delete")}
            </button>
          ) : null}
        </div>
      </section>
    </div>
  );
}
