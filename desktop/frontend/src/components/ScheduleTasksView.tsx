import { useCallback, useEffect, useMemo, useState } from "react";
import { CalendarClock, FolderOpen, Pause, Play, Trash2 } from "lucide-react";
import { StudioSelect } from "./StudioSelect";
import { app, onScheduleTask } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { ScheduledTask, SettingsView } from "../lib/types";

type WorkspaceScope = "current" | "global" | "custom";

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
    nextRun: 0,
  };
}

function scheduleTypeLabel(type: ScheduledTask["scheduleType"], t: ReturnType<typeof useT>): string {
  switch (type) {
    case "daily":
      return t("schedule.type.daily");
    case "interval":
      return t("schedule.type.interval");
    case "one-time":
      return t("schedule.type.oneTime");
    case "manual":
      return t("schedule.type.manual");
    default:
      return type;
  }
}

function defaultScheduleValue(type: ScheduledTask["scheduleType"]): string {
  switch (type) {
    case "interval":
      return "1h";
    case "one-time": {
      const next = new Date();
      next.setDate(next.getDate() + 1);
      next.setHours(9, 0, 0, 0);
      return formatOneTimeValue(next);
    }
    case "manual":
      return "";
    default:
      return "09:00";
  }
}

function formatOneTimeValue(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function resolveWorkspaceScope(task: ScheduledTask, workspaceRoot: string): WorkspaceScope {
  const root = (task.workspaceRoot || "").trim();
  if (!root || root === ".") return "global";
  if (root === workspaceRoot) return "current";
  return "custom";
}

function resolveWorkspaceRoot(scope: WorkspaceScope, customRoot: string, workspaceRoot: string): string {
  if (scope === "global") return ".";
  if (scope === "current") return workspaceRoot;
  return customRoot.trim() || workspaceRoot;
}

function formatNextRun(ts: number, t: ReturnType<typeof useT>): string {
  if (!ts) return "—";
  const delta = ts - Date.now();
  if (delta <= 0) return t("schedule.dueNow");
  if (delta < 3_600_000) return t("schedule.inMinutes", { n: Math.max(1, Math.round(delta / 60_000)) });
  if (delta < 86_400_000) return t("schedule.inHours", { n: Math.max(1, Math.round(delta / 3_600_000)) });
  return new Date(ts).toLocaleString();
}

function collectModels(settings: SettingsView | null): string[] {
  if (!settings) return [];
  const seen = new Set<string>();
  const out: string[] = [];
  for (const provider of settings.providers) {
    for (const model of provider.models) {
      if (!seen.has(model)) {
        seen.add(model);
        out.push(model);
      }
    }
  }
  if (settings.defaultModel && !seen.has(settings.defaultModel)) {
    out.unshift(settings.defaultModel);
  }
  return out;
}

export interface ScheduleTasksViewProps {
  workspaceRoot: string;
}

export function ScheduleTasksView({ workspaceRoot }: ScheduleTasksViewProps) {
  const t = useT();
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);
  const [draft, setDraft] = useState<ScheduledTask>(() => emptyTask(workspaceRoot));
  const [workspaceScope, setWorkspaceScope] = useState<WorkspaceScope>("current");
  const [customWorkspaceRoot, setCustomWorkspaceRoot] = useState(workspaceRoot);
  const [models, setModels] = useState<string[]>([]);
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
    void app.Settings().then((settings) => {
      const list = collectModels(settings);
      if (list.length) setModels(list);
    });
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
    setDraft((prev) => {
      if (prev.id) return prev;
      return { ...prev, workspaceRoot: prev.workspaceRoot || workspaceRoot };
    });
    setCustomWorkspaceRoot((prev) => prev || workspaceRoot);
  }, [workspaceRoot]);

  const selected = useMemo(() => tasks.find((task) => task.id === selectedId), [tasks, selectedId]);

  const stats = useMemo(() => {
    const active = tasks.filter((task) => task.enabled).length;
    return { active, paused: tasks.length - active };
  }, [tasks]);

  const beginCreate = () => {
    setSelectedId(null);
    setWorkspaceScope("current");
    setCustomWorkspaceRoot(workspaceRoot);
    setDraft(emptyTask(workspaceRoot));
    setErr(null);
    setNotice(null);
  };

  const beginEdit = (task: ScheduledTask) => {
    setSelectedId(task.id);
    setWorkspaceScope(resolveWorkspaceScope(task, workspaceRoot));
    setCustomWorkspaceRoot(task.workspaceRoot && task.workspaceRoot !== "." ? task.workspaceRoot : workspaceRoot);
    setDraft(task);
    setErr(null);
    setNotice(null);
  };

  const saveTask = async (task: ScheduledTask, opts?: { requirePrompt?: boolean; syncDraft?: boolean }) => {
    const next: ScheduledTask = {
      ...task,
      id: task.id || `task-${Date.now()}`,
      name: task.name.trim() || t("schedule.untitledTask"),
      prompt: task.prompt.trim(),
      model: task.model.trim() || models[0] || "deepseek-chat",
    };
    if (opts?.requirePrompt !== false && !next.prompt) {
      throw new Error(t("schedule.promptRequired"));
    }
    await app.SaveScheduledTask(next);
    if (opts?.syncDraft !== false) {
      setSelectedId(next.id);
      setDraft(next);
    }
    await reload();
    return next;
  };

  const save = async () => {
    setBusy(true);
    setErr(null);
    setNotice(null);
    try {
      await saveTask({
        ...draft,
        workspaceRoot: resolveWorkspaceRoot(workspaceScope, customWorkspaceRoot, workspaceRoot),
      });
      setNotice(t("schedule.saved"));
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const toggleEnabled = async (task: ScheduledTask) => {
    setBusy(true);
    setErr(null);
    try {
      await saveTask({ ...task, enabled: !task.enabled }, { requirePrompt: false, syncDraft: selectedId === task.id });
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
      if (selectedId === id) beginCreate();
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
      await reload();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const pickWorkspace = async () => {
    const path = await app.PickWorkspace().catch(() => "");
    if (!path) return;
    setWorkspaceScope("custom");
    setCustomWorkspaceRoot(path);
    setDraft((prev) => ({ ...prev, workspaceRoot: path }));
  };

  const scheduleValueHint =
    draft.scheduleType === "daily"
      ? t("schedule.valueHint.daily")
      : draft.scheduleType === "interval"
        ? t("schedule.valueHint.interval")
        : draft.scheduleType === "one-time"
          ? t("schedule.valueHint.oneTime")
          : t("schedule.valueHint.manual");

  return (
    <div className="schedule-tasks">
      <section className="schedule-tasks__column">
        <header className="schedule-tasks__head">
          <div className="schedule-tasks__head-copy">
            <h3>
              <CalendarClock size={15} /> {t("schedule.title")}
            </h3>
            <p className="schedule-tasks__sub">{t("schedule.panelHint")}</p>
          </div>
        </header>

        <div className="schedule-tasks__stats">
          <span className="schedule-tasks__chip schedule-tasks__chip--ok">{t("schedule.statsActive", { n: stats.active })}</span>
          <span className="schedule-tasks__chip">{t("schedule.statsPaused", { n: stats.paused })}</span>
        </div>

        <div className="schedule-tasks__table-head" aria-hidden="true">
          <span>{t("schedule.col.name")}</span>
          <span>{t("schedule.col.type")}</span>
          <span>{t("schedule.col.next")}</span>
          <span>{t("schedule.col.status")}</span>
        </div>

        <div className="schedule-tasks__table">
          {tasks.map((task) => (
            <div
              key={task.id}
              className={`schedule-tasks__row-wrap${selectedId === task.id ? " schedule-tasks__row-wrap--active" : ""}`}
            >
              <button type="button" className="schedule-tasks__row" onClick={() => beginEdit(task)}>
                <strong>{task.name}</strong>
                <span>{scheduleTypeLabel(task.scheduleType, t)}</span>
                <span>{formatNextRun(task.nextRun, t)}</span>
                <span className={`schedule-tasks__status${task.enabled ? " schedule-tasks__status--on" : ""}`}>
                  {task.enabled ? t("schedule.status.active") : t("schedule.status.paused")}
                </span>
              </button>
              <button
                type="button"
                className="schedule-tasks__toggle"
                disabled={busy}
                aria-label={task.enabled ? t("schedule.pause") : t("schedule.resume")}
                onClick={() => void toggleEnabled(task)}
              >
                {task.enabled ? <Pause size={13} /> : <Play size={13} />}
              </button>
            </div>
          ))}
          {!tasks.length ? <div className="schedule-tasks__empty">{t("schedule.empty")}</div> : null}
        </div>
      </section>

      <section className="schedule-tasks__column schedule-tasks__column--wide">
        <header className="schedule-tasks__head">
          <div className="schedule-tasks__head-copy">
            <h3>{selected ? t("schedule.edit") : t("schedule.create")}</h3>
            <p className="schedule-tasks__sub">{t("schedule.formHint")}</p>
          </div>
        </header>

        {err ? <div className="schedule-tasks__banner schedule-tasks__banner--error">{err}</div> : null}
        {notice ? <div className="schedule-tasks__banner schedule-tasks__banner--ok">{notice}</div> : null}

        <div className="schedule-tasks__form-scroll">
          <div className="schedule-tasks__form">
            <label>
              {t("schedule.taskName")}
              <input value={draft.name} onChange={(e) => setDraft((v) => ({ ...v, name: e.target.value }))} placeholder={t("schedule.untitledTask")} />
            </label>

            <label>
              {t("schedule.prompt")}
              <textarea
                value={draft.prompt}
                onChange={(e) => setDraft((v) => ({ ...v, prompt: e.target.value }))}
                placeholder={t("schedule.promptPlaceholder")}
                rows={4}
              />
            </label>

            <div className="schedule-tasks__form-row">
              <label>
                {t("schedule.type")}
                <StudioSelect
                  value={draft.scheduleType}
                  onChange={(scheduleType) =>
                    setDraft((v) => ({
                      ...v,
                      scheduleType: scheduleType as ScheduledTask["scheduleType"],
                      scheduleValue: defaultScheduleValue(scheduleType as ScheduledTask["scheduleType"]),
                    }))
                  }
                  options={[
                    { value: "daily", label: t("schedule.type.daily") },
                    { value: "interval", label: t("schedule.type.interval") },
                    { value: "one-time", label: t("schedule.type.oneTime") },
                    { value: "manual", label: t("schedule.type.manual") },
                  ]}
                />
              </label>

              {draft.scheduleType !== "manual" ? (
                <label>
                  {t("schedule.value")}
                  <input
                    value={draft.scheduleValue}
                    onChange={(e) => setDraft((v) => ({ ...v, scheduleValue: e.target.value }))}
                    placeholder={
                      draft.scheduleType === "daily"
                        ? "09:00"
                        : draft.scheduleType === "one-time"
                          ? "2026-06-08 09:00"
                          : t("schedule.valuePlaceholder")
                    }
                  />
                  <span className="schedule-tasks__field-hint">{scheduleValueHint}</span>
                </label>
              ) : (
                <p className="schedule-tasks__field-note">{scheduleValueHint}</p>
              )}
            </div>

            <label>
              {t("schedule.workspaceRoot")}
              <StudioSelect
                value={workspaceScope}
                onChange={(scope) => {
                  const nextScope = scope as WorkspaceScope;
                  setWorkspaceScope(nextScope);
                  setDraft((v) => ({ ...v, workspaceRoot: resolveWorkspaceRoot(nextScope, customWorkspaceRoot, workspaceRoot) }));
                }}
                options={[
                  { value: "current", label: t("schedule.workspaceCurrent", { path: workspaceRoot || "." }) },
                  { value: "global", label: t("schedule.workspaceGlobal") },
                  { value: "custom", label: t("schedule.workspaceCustom") },
                ]}
              />
            </label>

            {workspaceScope === "custom" ? (
              <div className="schedule-tasks__path-row">
                <input value={customWorkspaceRoot} onChange={(e) => setCustomWorkspaceRoot(e.target.value)} />
                <button type="button" className="schedule-tasks__ghost-btn" disabled={busy} onClick={() => void pickWorkspace()}>
                  <FolderOpen size={14} />
                  {t("schedule.workspacePick")}
                </button>
              </div>
            ) : null}

            <label>
              {t("schedule.model")}
              {models.length ? (
                <StudioSelect
                  value={draft.model}
                  onChange={(model) => setDraft((v) => ({ ...v, model }))}
                  options={models.map((model) => ({ value: model, label: model }))}
                />
              ) : (
                <input value={draft.model} onChange={(e) => setDraft((v) => ({ ...v, model: e.target.value }))} />
              )}
            </label>

            {selected && draft.nextRun ? (
              <div className="schedule-tasks__next-run">
                <span>{t("schedule.nextRun")}</span>
                <strong>{formatNextRun(draft.nextRun, t)}</strong>
                <code>{new Date(draft.nextRun).toLocaleString()}</code>
              </div>
            ) : null}
          </div>
        </div>

        <footer className="schedule-tasks__footer">
          <label className="schedule-tasks__enable">
            <input
              type="checkbox"
              checked={draft.enabled}
              onChange={(e) => setDraft((v) => ({ ...v, enabled: e.target.checked }))}
            />
            <span>{t("schedule.enabled")}</span>
          </label>
          <div className="schedule-tasks__actions">
            <button type="button" className="schedule-tasks__primary" disabled={busy} onClick={() => void save()}>
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
        </footer>
      </section>
    </div>
  );
}
