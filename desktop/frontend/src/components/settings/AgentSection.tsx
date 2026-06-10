import { useEffect, useState } from "react";
import { ChevronDown } from "lucide-react";
import { app } from "../../lib/bridge";
import { useT } from "../../lib/i18n";
import type { AgentSettingsInput, OutputStyleView, SettingsView } from "../../lib/types";
import { MotionUnfold } from "../MotionUnfold";
import { StudioSelect } from "../StudioSelect";
import { SettingsBlock, SettingsSaveChip, type SettingsSectionProps } from "../settingsPrimitives";
import { providerNames, toRef } from "./modelUtils";
import { SettingsShortcutRow } from "./SettingsShortcutRow";
import type { SettingsTab } from "./types";

const AUTO_PLAN_MODES = ["off", "on"] as const;

const ARCDESK_COMPACT_DEFAULTS = {
  softCompactRatio: 0.5,
  compactRatio: 0.8,
  compactForceRatio: 0.9,
} as const;

const TEMPERATURE_OPTIONS = [0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.2, 1.5, 2];
const MAX_STEPS_OPTIONS = [0, 5, 10, 15, 20, 30, 50, 100, 150, 200];
const SOFT_COMPACT_OPTIONS = [0.45, 0.5, 0.55, 0.6];
const COMPACT_OPTIONS = [0.75, 0.8, 0.85, 0.9];
const FORCE_COMPACT_OPTIONS = [0.85, 0.9, 0.95];

function ensureNumericOption(options: number[], value: number): number[] {
  if (options.includes(value)) return options;
  return [...options, value].sort((a, b) => a - b);
}

function SettingsNumericSelect({
  value,
  onChange,
  options,
  disabled,
  className,
  formatLabel,
}: {
  value: number;
  onChange: (value: number) => void;
  options: number[];
  disabled?: boolean;
  className?: string;
  formatLabel: (value: number) => string;
}) {
  const merged = ensureNumericOption(options, value);
  return (
    <StudioSelect
      className={className ?? "set-grow"}
      value={String(value)}
      disabled={disabled}
      onChange={(next) => onChange(Number(next))}
      options={merged.map((option) => ({ value: String(option), label: formatLabel(option) }))}
    />
  );
}

function agentDraftFromSettings(s: SettingsView): AgentSettingsInput {
  const a = s.agent;
  const prompt = a.usesDefaultPrompt || !a.systemPrompt.trim() ? "" : a.systemPrompt;
  return {
    temperature: a.temperature,
    maxSteps: a.maxSteps,
    systemPrompt: prompt,
    systemPromptFile: a.systemPromptFile,
    outputStyle: a.outputStyle,
    autoPlan: a.autoPlan,
    autoPlanClassifier: a.autoPlanClassifier,
    softCompactRatio: a.softCompactRatio,
    compactRatio: a.compactRatio,
    compactForceRatio: a.compactForceRatio,
    subagentModel: a.subagentModel,
    subagentModels: { ...a.subagentModels },
  };
}

function agentDraftEquals(a: AgentSettingsInput, b: AgentSettingsInput): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

function outputStyleLabel(name: string, t: ReturnType<typeof useT>): string {
  const key = `settings.agent.outputStyle.${name.replace(/-/g, "_")}` as const;
  const translated = t(key as "settings.agent.outputStyle.explanatory");
  return translated !== key ? translated : name;
}

function outputStyleDescription(name: string, styles: OutputStyleView[], t: ReturnType<typeof useT>): string {
  const key = `settings.agent.outputStyleDesc.${name.replace(/-/g, "_")}` as const;
  const translated = t(key as "settings.agent.outputStyleDesc.explanatory");
  if (translated !== key) return translated;
  return styles.find((st) => st.name === name)?.description ?? "";
}

function subagentSkillLabel(name: string, t: ReturnType<typeof useT>): string {
  const key = `settings.agent.skill.${name.replace(/-/g, "_")}` as const;
  const translated = t(key as "settings.agent.skill.explore");
  return translated !== key ? translated : name;
}

function normalizeAgentPromptForSave(draft: AgentSettingsInput, defaultPrompt: string): string {
  const trimmed = draft.systemPrompt.trim();
  const defaultTrimmed = defaultPrompt.trim();
  if (!trimmed || trimmed === defaultTrimmed) return "";
  return draft.systemPrompt;
}

export function AgentSection({
  s,
  busy,
  apply,
  onNavigateTab,
  onComposerPrompt,
}: SettingsSectionProps & { onNavigateTab: (tab: SettingsTab) => void; onComposerPrompt?: (text: string) => void }) {
  const t = useT();
  const saved = agentDraftFromSettings(s);
  const [draft, setDraft] = useState<AgentSettingsInput>(saved);
  const [outputStyles, setOutputStyles] = useState<OutputStyleView[]>([]);
  const [subagentSkills, setSubagentSkills] = useState<string[]>([]);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  useEffect(() => {
    setDraft(agentDraftFromSettings(s));
  }, [s]);

  useEffect(() => {
    void app.ListOutputStyles().then(setOutputStyles).catch(() => setOutputStyles([]));
    void app
      .Capabilities()
      .then((view) => {
        const names = view.skills.filter((sk) => sk.runAs === "subagent").map((sk) => sk.name);
        setSubagentSkills(Array.from(new Set(names)).sort());
      })
      .catch(() => setSubagentSkills([]));
  }, []);

  const defaultPrompt = s.agent.defaultSystemPrompt;
  const dirty = !agentDraftEquals(
    { ...draft, systemPrompt: normalizeAgentPromptForSave(draft, defaultPrompt) },
    { ...saved, systemPrompt: normalizeAgentPromptForSave(saved, defaultPrompt) },
  );
  const providerOpts = providerNames(s.providers);
  const plannerLabel = s.plannerModel ? toRef(s.plannerModel, s) : t("common.none");
  const promptDisplay = draft.systemPrompt.trim() ? draft.systemPrompt : defaultPrompt;
  const promptDirty =
    normalizeAgentPromptForSave(draft, defaultPrompt) !== normalizeAgentPromptForSave(saved, defaultPrompt);

  const patchDraft = (patch: Partial<AgentSettingsInput>) => {
    setDraft((current) => ({ ...current, ...patch }));
  };

  const patchSubagentModel = (skill: string, model: string) => {
    setDraft((current) => {
      const next = { ...current.subagentModels };
      if (!model) delete next[skill];
      else next[skill] = model;
      return { ...current, subagentModels: next };
    });
  };

  const save = () => {
    const payload: AgentSettingsInput = {
      ...draft,
      systemPrompt: normalizeAgentPromptForSave(draft, defaultPrompt),
    };
    void apply(() => app.SetAgentSettings(payload));
  };

  const resetPrompt = () => patchDraft({ systemPrompt: "" });

  const resetCompactDefaults = () => {
    patchDraft({ ...ARCDESK_COMPACT_DEFAULTS });
  };

  const formatRatioLabel = (ratio: number) => `${Math.round(ratio * 100)}%`;

  const formatMaxStepsLabel = (steps: number) =>
    steps === 0 ? t("settings.agent.maxStepsUnlimited") : t("settings.agent.maxStepsValue", { n: String(steps) });

  const formatTemperatureLabel = (temp: number) => String(temp);

  const compactDefaultsDirty =
    draft.softCompactRatio !== ARCDESK_COMPACT_DEFAULTS.softCompactRatio ||
    draft.compactRatio !== ARCDESK_COMPACT_DEFAULTS.compactRatio ||
    draft.compactForceRatio !== ARCDESK_COMPACT_DEFAULTS.compactForceRatio;

  return (
    <>
      <SettingsBlock title={t("settings.runtimeModes.title")} hint={t("settings.runtimeModes.hint")}>
        <div className="settings-runtime-modes">
          <article className="settings-runtime-modes__card">
            <strong>{t("settings.runtimeModes.normalTitle")}</strong>
            <p>{t("settings.runtimeModes.normalBody")}</p>
          </article>
          <article className="settings-runtime-modes__card">
            <strong>{t("settings.runtimeModes.planTitle")}</strong>
            <p>{t("settings.runtimeModes.planBody")}</p>
          </article>
          <article className="settings-runtime-modes__card">
            <strong>{t("settings.runtimeModes.yoloTitle")}</strong>
            <p>{t("settings.runtimeModes.yoloBody")}</p>
          </article>
        </div>
        <p className="settings-block__note">{t("settings.runtimeModes.shortcut")}</p>
      </SettingsBlock>

      <SettingsBlock title={t("settings.toolsExtras.title")} hint={t("settings.toolsExtras.hint")}>
        <div className="settings-block__stack settings-block__stack--shortcut-actions">
          <SettingsShortcutRow
            title={t("settings.toolsExtras.branchesTitle")}
            hint={t("settings.toolsExtras.branchesHint")}
            buttonLabel={t("settings.toolsExtras.branchesAction")}
            onClick={() => onComposerPrompt?.("/tree")}
            disabled={busy || !onComposerPrompt}
          />
          <SettingsShortcutRow
            title={t("settings.toolsExtras.hooksTitle")}
            hint={t("settings.toolsExtras.hooksHint")}
            buttonLabel={t("settings.toolsExtras.hooksAction")}
            onClick={() => onComposerPrompt?.("/hooks list")}
            disabled={busy || !onComposerPrompt}
          />
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.agent.inferenceTitle")} hint={t("settings.agent.inferenceHint")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.agent.temperature")}</label>
            <SettingsNumericSelect
              value={draft.temperature}
              disabled={busy}
              options={TEMPERATURE_OPTIONS}
              formatLabel={formatTemperatureLabel}
              onChange={(value) => patchDraft({ temperature: value })}
            />
          </div>
          <div className="set-row">
            <label className="set-label">{t("settings.agent.maxSteps")}</label>
            <SettingsNumericSelect
              value={draft.maxSteps}
              disabled={busy}
              options={MAX_STEPS_OPTIONS}
              formatLabel={formatMaxStepsLabel}
              onChange={(value) => patchDraft({ maxSteps: value })}
            />
          </div>
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.agent.promptTitle")} hint={t("settings.agent.systemPromptHint")}>
        <div className="settings-instructions-editor">
          <textarea
            id="agent-prompt"
            className="settings-block__textarea mem-input"
            rows={10}
            value={promptDisplay}
            disabled={busy}
            onChange={(e) => patchDraft({ systemPrompt: e.target.value })}
          />
          <div className="settings-instructions-editor__bar">
            <button type="button" className="settings-action-btn settings-action-btn--compact" disabled={busy || !promptDirty} onClick={resetPrompt}>
              {t("settings.agent.resetPrompt")}
            </button>
          </div>
        </div>
        <div className="set-row settings-agent-style-row">
          <label className="set-label" htmlFor="agent-style">
            {t("settings.agent.outputStyle")}
          </label>
          <StudioSelect
            className="set-grow"
            id="agent-style"
            value={draft.outputStyle}
            disabled={busy}
            onChange={(value) => patchDraft({ outputStyle: value })}
            options={[
              { value: "", label: t("settings.agent.outputStyleDefault") },
              ...outputStyles.map((style) => ({
                value: style.name,
                label: style.builtin
                  ? outputStyleLabel(style.name, t)
                  : `${style.name}（${t("settings.agent.outputStyleCustom")}）`,
              })),
            ]}
          />
        </div>
        {draft.outputStyle ? (
          <p className="settings-block__note settings-block__note--inline">
            {outputStyleDescription(draft.outputStyle, outputStyles, t)}
          </p>
        ) : null}
      </SettingsBlock>

      <SettingsBlock title={t("settings.agent.planningTitle")} hint={t("settings.agent.planningHint")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.autoPlan")}</label>
            <div className="set-seg set-seg--compact">
              {AUTO_PLAN_MODES.map((mode) => (
                <button
                  key={mode}
                  type="button"
                  className={`set-seg__btn${draft.autoPlan === mode ? " set-seg__btn--on" : ""}`}
                  disabled={busy}
                  onClick={() => patchDraft({ autoPlan: mode })}
                >
                  {mode === "on" ? t("settings.autoPlan.on") : t("settings.autoPlan.off")}
                </button>
              ))}
            </div>
          </div>
          <div className="set-row">
            <label className="set-label">{t("settings.plannerModel")}</label>
            <div className="settings-agent-linked-value">
              <span className="settings-agent-linked-value__text">{plannerLabel}</span>
              <button type="button" className="settings-action-btn settings-action-btn--compact" onClick={() => onNavigateTab("models")}>
                {t("settings.agent.openModels")}
              </button>
            </div>
          </div>
          <div className="set-row set-row--stack">
            <label className="set-label">{t("settings.agent.autoPlanClassifier")}</label>
            <StudioSelect
              className="set-grow"
              value={draft.autoPlanClassifier}
              disabled={busy || draft.autoPlan === "off"}
              onChange={(value) => patchDraft({ autoPlanClassifier: value })}
              options={providerOpts}
            />
            <p className="settings-block__note settings-block__note--inline">{t("settings.agent.autoPlanClassifierHint")}</p>
          </div>
          <p className="settings-block__note settings-block__note--inline">{t("settings.agent.permissionsNote")}</p>
          <button type="button" className="settings-action-btn settings-action-btn--compact" onClick={() => onNavigateTab("permissions")}>
            {t("settings.agent.openPermissions")}
          </button>
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.agent.subagentTitle")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.agent.subagentDefault")}</label>
            <StudioSelect
              className="set-grow"
              value={draft.subagentModel}
              disabled={busy || providerOpts.length <= 1}
              onChange={(value) => patchDraft({ subagentModel: value })}
              options={providerOpts}
            />
          </div>
          {subagentSkills.length > 0 ? (
            <div className="settings-agent-subagent-list">
              {subagentSkills.map((skill) => (
                <div key={skill} className="set-row settings-agent-subagent-list__row">
                  <label className="set-label">{subagentSkillLabel(skill, t)}</label>
                  <StudioSelect
                    className="set-grow"
                    value={draft.subagentModels[skill] ?? ""}
                    disabled={busy || providerOpts.length <= 1}
                    onChange={(value) => patchSubagentModel(skill, value)}
                    options={providerOpts}
                  />
                </div>
              ))}
            </div>
          ) : (
            <p className="settings-block__note">{t("settings.agent.subagentEmpty")}</p>
          )}
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.agent.advancedTitle")} hint={t("settings.agent.advancedHint")}>
        <button
          type="button"
          className={`settings-agent-advanced-toggle${advancedOpen ? " settings-agent-advanced-toggle--open" : ""}`}
          aria-expanded={advancedOpen}
          onClick={() => setAdvancedOpen((open) => !open)}
        >
          <ChevronDown size={13} aria-hidden="true" />
          <span>{advancedOpen ? t("settings.agent.advancedHide") : t("settings.agent.advancedShow")}</span>
        </button>
        <MotionUnfold open={advancedOpen}>
          <div className="settings-block__form settings-agent-advanced-body">
            <div className="set-row set-row--stack">
              <label className="set-label">{t("settings.agent.systemPromptFile")}</label>
              <input
                id="agent-prompt-file"
                className="mem-input settings-block__input"
                value={draft.systemPromptFile}
                disabled={busy}
                placeholder={t("settings.agent.systemPromptFilePlaceholder")}
                onChange={(e) => patchDraft({ systemPromptFile: e.target.value })}
              />
              <p className="settings-block__note settings-block__note--inline">{t("settings.agent.systemPromptFileHint")}</p>
            </div>
            <div className="set-row">
              <label className="set-label">{t("settings.agent.softCompactRatio")}</label>
              <SettingsNumericSelect
                value={draft.softCompactRatio}
                disabled={busy}
                options={SOFT_COMPACT_OPTIONS}
                formatLabel={formatRatioLabel}
                onChange={(value) => patchDraft({ softCompactRatio: value })}
              />
            </div>
            <div className="set-row">
              <label className="set-label">{t("settings.agent.compactRatio")}</label>
              <SettingsNumericSelect
                value={draft.compactRatio}
                disabled={busy}
                options={COMPACT_OPTIONS}
                formatLabel={formatRatioLabel}
                onChange={(value) => patchDraft({ compactRatio: value })}
              />
            </div>
            <div className="set-row">
              <label className="set-label">{t("settings.agent.compactForceRatio")}</label>
              <SettingsNumericSelect
                value={draft.compactForceRatio}
                disabled={busy}
                options={FORCE_COMPACT_OPTIONS}
                formatLabel={formatRatioLabel}
                onChange={(value) => patchDraft({ compactForceRatio: value })}
              />
            </div>
            <p className="settings-block__note settings-block__note--inline">{t("settings.agent.compactHint")}</p>
            <div className="settings-agent-prompt-actions">
              <button
                type="button"
                className="settings-action-btn settings-action-btn--compact"
                disabled={busy || !compactDefaultsDirty}
                onClick={resetCompactDefaults}
              >
                {t("settings.agent.resetCompactDefaults")}
              </button>
            </div>
          </div>
        </MotionUnfold>
      </SettingsBlock>

      <div className="settings-agent-save-row">
        <SettingsSaveChip disabled={busy || !dirty} ready={dirty} onClick={save}>
          {t("settings.agent.save")}
        </SettingsSaveChip>
      </div>
    </>
  );
}
