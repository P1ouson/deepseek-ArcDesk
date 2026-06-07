import { useCallback, useEffect, useMemo, useState } from "react";
import { CalendarClock, Play, Plus, Trash2 } from "lucide-react";
import { app, onScheduleTask } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { ScheduledTask } from "../lib/types";

function emptyTask(workspaceRoot: string): ScheduledTask {
  return {
    id: "",
    name: "",
    prompt: "",
    scheduleType: "daily",
    scheduleValue: "09:00",
    workspaceRoot,
    model: "deepseek-chat",
    enabled: true,
    nextRun: Date.now() + 3_600_000,
  };
}

function formatNextRun(ts: number, t: ReturnType<typeof useT>): string {
  if (!ts) return "—";
  const delta = ts - Date.now();
  if (delta <= 0) return t("schedule.dueNow");
  if (delta < 3_600_000) return t("schedule.inMinutes", { n: Math.round(delta / 60_000) });
  if (delta < 86_400_000) return t("schedule.inHours", { n: Math.round(delta / 3_600_000) });
  return new Date(ts).toLocaleString();
}

export interface ScheduleTasksViewProps {
  workspaceRoot: string;
}

export function ScheduleTasksView({ workspaceRoot }: ScheduleTasksViewProps) {
  const t = useT();
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);
  const [draft, setDraft] = useState<ScheduledTask>(() => emptyTask(workspaceRoot));
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const items = await app.GetScheduledTasks().catch(() => [] as ScheduledTask[]);
    setTasks(items);
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  useEffect(() => {
    return onScheduleTask((event) => {
      void reload();
      if (event.error) {
        setNotice(t("schedule.failed", { name: event.name, error: event.error }));
        return;
      }
      if (event.source === "auto") {
        setNotice(t("schedule.autoRan", { name: event.name }));
      }
    });
  }, [reload, t]);

  useEffect(() => {
    setDraft((prev) => ({ ...prev, workspaceRoot: prev.workspaceRoot || workspaceRoot }));
  }, [workspaceRoot]);

  const selected = useMemo(() => tasks.find((task) => task.id === selectedId), [tasks, selectedId]);

  const save = async () => {
    setBusy(true);
    setErr(null);
    setNotice(null);
    try {
      const next: ScheduledTask = {
        ...draft,
        id: draft.id || `task-${Date.now()}`,
        name: draft.name.trim() || t("schedule.untitledTask"),
        workspaceRoot: draft.workspaceRoot || workspaceRoot,
      };
      await app.SaveScheduledTask(next);
      setSelectedId(next.id);
      setDraft(next);
      await reload();
      setNotice(t("schedule.saved"));
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
      await app.DeleteScheduledTask(id);
      if (selectedId === id) {
        setSelectedId(null);
        setDraft(emptyTask(workspaceRoot));
      }
      await reload();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const trigger = async (id: string) => {
    setBusy(true);
    setErr(null);
    setNotice(null);
    try {
      await app.TriggerScheduledTask(id);
      setNotice(t("schedule.triggered"));
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="schedule-tasks">
      <section className="schedule-tasks__column">
        <header className="schedule-tasks__head">
          <h3>
            <CalendarClock size={15} /> {t("schedule.title")}
          </h3>
          <button
            type="button"
            onClick={() => {
              setSelectedId(null);
              setDraft(emptyTask(workspaceRoot));
            }}
          >
            <Plus size={14} />
          </button>
        </header>
        <div className="schedule-tasks__table">
          {tasks.map((task) => (
            <button
              key={task.id}
              type="button"
              className={`schedule-tasks__row${selectedId === task.id ? " schedule-tasks__row--active" : ""}`}
              onClick={() => {
                setSelectedId(task.id);
                setDraft(task);
              }}
            >
              <strong>{task.name}</strong>
              <span>{task.scheduleType}</span>
              <span>{formatNextRun(task.nextRun, t)}</span>
              <span>{task.enabled ? t("schedule.status.active") : t("schedule.status.paused")}</span>
            </button>
          ))}
          {!tasks.length ? <div className="schedule-tasks__empty">{t("schedule.empty")}</div> : null}
        </div>
      </section>

      <section className="schedule-tasks__column schedule-tasks__column--wide">
        <header className="schedule-tasks__head">
          <h3>{selected ? t("schedule.edit") : t("schedule.create")}</h3>
        </header>
        {err ? <div className="schedule-tasks__banner schedule-tasks__banner--error">{err}</div> : null}
        {notice ? <div className="schedule-tasks__banner schedule-tasks__banner--ok">{notice}</div> : null}
        <div className="schedule-tasks__form">
          <label>
            {t("schedule.taskName")}
            <input value={draft.name} onChange={(e) => setDraft((v) => ({ ...v, name: e.target.value }))} placeholder={t("schedule.create")} />
          </label>
          <label>
            {t("schedule.prompt")}
            <textarea
              value={draft.prompt}
              onChange={(e) => setDraft((v) => ({ ...v, prompt: e.target.value }))}
              placeholder={t("schedule.promptPlaceholder")}
            />
          </label>
          <label>
            {t("schedule.type")}
            <select
              value={draft.scheduleType}
              onChange={(e) => setDraft((v) => ({ ...v, scheduleType: e.target.value as ScheduledTask["scheduleType"] }))}
            >
              <option value="daily">{t("schedule.type.daily")}</option>
              <option value="interval">{t("schedule.type.interval")}</option>
              <option value="one-time">{t("schedule.type.oneTime")}</option>
              <option value="manual">{t("schedule.type.manual")}</option>
            </select>
          </label>
          <label>
            {t("schedule.value")}
            <input
              value={draft.scheduleValue}
              onChange={(e) => setDraft((v) => ({ ...v, scheduleValue: e.target.value }))}
              placeholder={t("schedule.valuePlaceholder")}
            />
          </label>
          <label>
            {t("schedule.workspaceRoot")}
            <input value={draft.workspaceRoot || workspaceRoot} onChange={(e) => setDraft((v) => ({ ...v, workspaceRoot: e.target.value }))} />
          </label>
          <label>
            {t("schedule.model")}
            <input value={draft.model} onChange={(e) => setDraft((v) => ({ ...v, model: e.target.value }))} />
          </label>
          <label className="schedule-tasks__checkbox">
            <input type="checkbox" checked={draft.enabled} onChange={(e) => setDraft((v) => ({ ...v, enabled: e.target.checked }))} />
            {t("schedule.enabled")}
          </label>
        </div>
        <div className="schedule-tasks__actions">
          <button type="button" disabled={busy} onClick={() => void save()}>
            {t("schedule.save")}
          </button>
          {selected ? (
            <>
              <button type="button" disabled={busy} onClick={() => void trigger(selected.id)}>
                <Play size={14} />
                {t("schedule.runNow")}
              </button>
              <button type="button" className="schedule-tasks__danger" disabled={busy} onClick={() => void remove(selected.id)}>
                <Trash2 size={14} />
                {t("schedule.delete")}
              </button>
            </>
          ) : null}
        </div>
      </section>
    </div>
  );
}
