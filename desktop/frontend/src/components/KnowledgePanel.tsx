import {
  AlertTriangle,
  BookMarked,
  Check,
  FileCode2,
  Layers,
  Search,
  ShieldAlert,
  Sparkles,
  Trash2,
  X,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import {
  cardPreview,
  cardTitle,
  dedupeByFingerprint,
  injectableCount,
  searchAndSort,
  sectionCounts,
  type KnowledgeSection,
} from "../lib/knowledgeStudio";
import { useT } from "../lib/i18n";
import type { KnowledgeEntry, KnowledgeView } from "../lib/types";
import { ResizableDrawer } from "./ResizableDrawer";
import { StudioCenterModal } from "./StudioCenterModal";

type KnowledgeLabelKey =
  | "knowledge.section.all"
  | "knowledge.section.fix"
  | "knowledge.section.playbook"
  | "knowledge.section.convention"
  | "knowledge.section.negative"
  | "knowledge.section.stale";

type KnowledgeHintKey =
  | "knowledge.section.allHint"
  | "knowledge.section.fixHint"
  | "knowledge.section.playbookHint"
  | "knowledge.section.conventionHint"
  | "knowledge.section.negativeHint"
  | "knowledge.section.staleHint";

type NavItem = {
  id: KnowledgeSection;
  labelKey: KnowledgeLabelKey;
  hintKey: KnowledgeHintKey;
  icon: typeof Sparkles;
};

const NAV: NavItem[] = [
  { id: "all", labelKey: "knowledge.section.all", hintKey: "knowledge.section.allHint", icon: Layers },
  { id: "fix", labelKey: "knowledge.section.fix", hintKey: "knowledge.section.fixHint", icon: Sparkles },
  { id: "playbook", labelKey: "knowledge.section.playbook", hintKey: "knowledge.section.playbookHint", icon: BookMarked },
  { id: "convention", labelKey: "knowledge.section.convention", hintKey: "knowledge.section.conventionHint", icon: FileCode2 },
  { id: "negative", labelKey: "knowledge.section.negative", hintKey: "knowledge.section.negativeHint", icon: ShieldAlert },
  { id: "stale", labelKey: "knowledge.section.stale", hintKey: "knowledge.section.staleHint", icon: AlertTriangle },
];

function confidenceClass(confidence: string): string {
  switch (confidence.trim().toLowerCase()) {
    case "user_confirmed":
      return "knowledge-studio__badge--confirmed";
    case "verified":
      return "knowledge-studio__badge--verified";
    case "draft":
      return "knowledge-studio__badge--draft";
    case "stale":
      return "knowledge-studio__badge--stale";
    default:
      return "knowledge-studio__badge--draft";
  }
}

function confidenceLabel(confidence: string, t: ReturnType<typeof useT>): string {
  switch (confidence.trim().toLowerCase()) {
    case "user_confirmed":
      return t("knowledge.confidence.userConfirmed");
    case "verified":
      return t("knowledge.confidence.verified");
    case "draft":
      return t("knowledge.confidence.draft");
    case "stale":
      return t("knowledge.confidence.stale");
    default:
      return confidence;
  }
}

function kindLabel(kind: string | undefined, t: ReturnType<typeof useT>): string {
  switch ((kind || "fix").toLowerCase()) {
    case "playbook":
      return t("knowledge.kind.playbook");
    case "convention":
      return t("knowledge.kind.convention");
    default:
      return t("knowledge.kind.fix");
  }
}

function KnowledgeEntryCard({
  entry,
  active,
  onSelect,
}: {
  entry: KnowledgeEntry;
  active: boolean;
  onSelect: () => void;
}) {
  const t = useT();
  const title = cardTitle(entry);
  const preview = cardPreview(entry);

  return (
    <button
      type="button"
      className={`knowledge-studio__card${active ? " knowledge-studio__card--active" : ""}`}
      onClick={onSelect}
    >
      <div className="knowledge-studio__card-head">
        <code className="knowledge-studio__card-id">{title}</code>
        <span className={`knowledge-studio__badge ${confidenceClass(entry.confidence)}`}>
          {confidenceLabel(entry.confidence, t)}
        </span>
      </div>
      <p className="knowledge-studio__card-preview">{preview || "—"}</p>
      <div className="knowledge-studio__card-meta">
        <span className="knowledge-studio__chip">{kindLabel(entry.kind, t)}</span>
        {entry.hits > 0 ? <span className="knowledge-studio__chip">{t("knowledge.hits", { count: entry.hits })}</span> : null}
        {(entry.paths ?? []).slice(0, 2).map((path) => (
          <span key={path} className="knowledge-studio__chip knowledge-studio__chip--path">
            {path.split(/[/\\]/).pop() || path}
          </span>
        ))}
        {entry.provenanceStale && entry.provenanceStaleReason === "commit_mismatch" ? (
          <span className="knowledge-studio__chip knowledge-studio__chip--warn">
            {t("knowledge.commitMismatchShort")}
          </span>
        ) : null}
      </div>
    </button>
  );
}

function previewModalTitle(entry: KnowledgeEntry): string {
  const sig = entry.signature.trim();
  if (sig) return sig;
  const title = cardTitle(entry);
  if (title.length <= 80) return title;
  return `${title.slice(0, 77)}…`;
}

function sameLessonText(a: string, b: string): boolean {
  const norm = (s: string) => s.trim().replace(/\s+/g, " ").toLowerCase();
  const left = norm(a);
  const right = norm(b);
  return left !== "" && left === right;
}

function KnowledgeDetail({
  entry,
  busy,
  onClose,
  onConfirm,
  onStale,
  inModal = false,
}: {
  entry: KnowledgeEntry;
  busy: boolean;
  onClose: () => void;
  onConfirm: () => void;
  onStale: () => void;
  inModal?: boolean;
}) {
  const t = useT();
  const userStale = entry.confidence.trim().toLowerCase() === "stale";
  const provenanceStale = entry.provenanceStale === true;
  const commitMismatch = entry.provenanceStaleReason === "commit_mismatch";
  const summaryText = (entry.summary.trim() || cardPreview(entry, 240)).trim();
  const fixText = entry.fix.trim();
  const showSummary = summaryText !== "" && !sameLessonText(summaryText, fixText);

  const badges = (
    <div className="knowledge-studio__detail-badges">
      <span className={`knowledge-studio__badge ${confidenceClass(entry.confidence)}`}>
        {confidenceLabel(entry.confidence, t)}
      </span>
      <span className="knowledge-studio__chip">{kindLabel(entry.kind, t)}</span>
      {entry.hits > 0 ? <span className="knowledge-studio__chip">{t("knowledge.hits", { count: entry.hits })}</span> : null}
    </div>
  );

  const body = (
    <>
      {inModal ? <div className="knowledge-studio__detail-meta">{badges}</div> : null}

      {commitMismatch ? (
        <section className="knowledge-studio__detail-block">
          <p className="knowledge-studio__detail-note knowledge-studio__detail-note--warn">
            {t("knowledge.commitMismatchNote", {
              recorded: entry.repoHead?.slice(0, 7) ?? "—",
              current: entry.currentRepoHead?.slice(0, 7) ?? "—",
            })}
          </p>
        </section>
      ) : null}

      {showSummary ? (
        <section className="knowledge-studio__detail-block">
          <h3>{t("knowledge.detail.summary")}</h3>
          <p className="knowledge-studio__detail-copy">{summaryText}</p>
        </section>
      ) : null}

      {entry.error ? (
        <section className="knowledge-studio__detail-block">
          <h3>{t("knowledge.detail.problem")}</h3>
          <pre className="knowledge-studio__detail-pre">{entry.error}</pre>
        </section>
      ) : null}

      {fixText ? (
        <section className="knowledge-studio__detail-block">
          <h3>{t("knowledge.detail.fix")}</h3>
          <p className="knowledge-studio__detail-copy">{fixText}</p>
        </section>
      ) : null}

      <section className="knowledge-studio__detail-block">
        <h3>{t("knowledge.detail.evidence")}</h3>
        <dl className="knowledge-studio__evidence">
          <div>
            <dt>{t("knowledge.detail.id")}</dt>
            <dd><code>{entry.id}</code></dd>
          </div>
          {entry.signature ? (
            <div>
              <dt>{t("knowledge.detail.signature")}</dt>
              <dd><code>{entry.signature}</code></dd>
            </div>
          ) : null}
          {(entry.paths ?? []).length > 0 ? (
            <div>
              <dt>{t("knowledge.detail.paths")}</dt>
              <dd className="knowledge-studio__path-list">
                {(entry.paths ?? []).map((path) => (
                  <code key={path}>{path}</code>
                ))}
              </dd>
            </div>
          ) : null}
          {entry.repoHead ? (
            <div>
              <dt>{t("knowledge.detail.repoHead")}</dt>
              <dd><code>{entry.repoHead.slice(0, 7)}</code></dd>
            </div>
          ) : null}
        </dl>
      </section>
    </>
  );

  const actions = (
    <div className="knowledge-studio__detail-actions">
      {userStale ? (
        <p className="knowledge-studio__detail-note">{t("knowledge.staleNote")}</p>
      ) : (
        <>
          {provenanceStale && !commitMismatch ? (
            <p className="knowledge-studio__detail-note">{t("knowledge.provenanceStaleNote")}</p>
          ) : null}
          <button type="button" className="knowledge-studio__action knowledge-studio__action--primary" disabled={busy} onClick={onConfirm}>
            <Check size={14} aria-hidden="true" />
            {t("knowledge.confirm")}
          </button>
          <button type="button" className="knowledge-studio__action" disabled={busy} onClick={onStale}>
            <Trash2 size={14} aria-hidden="true" />
            {t("knowledge.stale")}
          </button>
        </>
      )}
    </div>
  );

  if (inModal) {
    return (
      <div className="knowledge-studio__detail knowledge-studio__detail--modal">
        <div className="knowledge-studio__detail-scroll">{body}</div>
        {actions}
      </div>
    );
  }

  return (
    <div className="knowledge-studio__detail">
      <div className="knowledge-studio__detail-head">
        <div>
          <code className="knowledge-studio__detail-id">{cardTitle(entry)}</code>
          {badges}
        </div>
        <button type="button" className="knowledge-studio__detail-close" onClick={onClose} aria-label={t("common.close")}>
          <X size={16} aria-hidden="true" />
        </button>
      </div>
      {body}
      {actions}
    </div>
  );
}

function KnowledgeStudio({
  view,
  onConfirm,
  onStale,
}: {
  view: KnowledgeView | null;
  onConfirm: (id: string) => Promise<void> | void;
  onStale: (id: string) => Promise<void> | void;
}) {
  const t = useT();
  const [section, setSection] = useState<KnowledgeSection>("all");
  const [query, setQuery] = useState("");
  const [previewEntry, setPreviewEntry] = useState<KnowledgeEntry | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const entries = useMemo(() => dedupeByFingerprint(view?.entries ?? []), [view?.entries]);
  const counts = useMemo(() => sectionCounts(entries), [entries]);
  const filtered = useMemo(() => searchAndSort(entries, query, section), [entries, query, section]);

  useEffect(() => {
    setQuery("");
    setPreviewEntry(null);
  }, [section]);

  useEffect(() => {
    if (!previewEntry) return;
    if (!entries.some((entry) => entry.id === previewEntry.id)) {
      setPreviewEntry(null);
    }
  }, [entries, previewEntry]);

  if (view === null) {
    return <div className="knowledge-studio__loading">{t("common.loading")}</div>;
  }

  if (!view.available) {
    return (
      <div className="knowledge-studio__empty-page">
        <AlertTriangle size={18} aria-hidden="true" />
        <p>{t("knowledge.unavailable")}</p>
      </div>
    );
  }

  const activeNav = NAV.find((item) => item.id === section) ?? NAV[0];

  return (
    <div className="knowledge-studio-shell">
      <aside className="knowledge-studio__sidebar">
        <div className="knowledge-studio__sidebar-head">
          <div className="knowledge-studio__sidebar-title">
            <BookMarked size={16} aria-hidden="true" />
            {t("knowledge.title")}
          </div>
          <p className="knowledge-studio__sidebar-sub">{t("knowledge.subtitle")}</p>
        </div>

        <nav className="knowledge-studio__nav" aria-label={t("knowledge.title")}>
          <span className="knowledge-studio__nav-label">{t("knowledge.nav.experience")}</span>
          {NAV.map((item) => {
            const Icon = item.icon;
            return (
              <button
                key={item.id}
                type="button"
                className={`knowledge-studio__nav-item${section === item.id ? " knowledge-studio__nav-item--active" : ""}`}
                onClick={() => setSection(item.id)}
              >
                <Icon size={14} aria-hidden="true" />
                <span className="knowledge-studio__nav-copy">
                  <strong>{t(item.labelKey)}</strong>
                  <small>{t(item.hintKey)}</small>
                </span>
                <span className="knowledge-studio__nav-count">{counts[item.id]}</span>
              </button>
            );
          })}
        </nav>

        <div className="knowledge-studio__sidebar-foot">
          <p>{t("knowledge.rulesHint")}</p>
        </div>
      </aside>

      <main className="knowledge-studio__main">
        <header className="knowledge-studio__toolbar">
          <div className="knowledge-studio__toolbar-copy">
            <h2 className="knowledge-studio__panel-title">{t(activeNav.labelKey)}</h2>
            <p className="knowledge-studio__panel-sub">
              {query.trim()
                ? t("knowledge.statsSearch", {
                    total: entries.length,
                    shown: filtered.length,
                  })
                : t("knowledge.stats", {
                    total: entries.length,
                    injectable: injectableCount(entries),
                    shown: filtered.length,
                  })}
            </p>
          </div>
        </header>

        <label className="knowledge-studio__search">
          <Search size={15} aria-hidden="true" />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t("knowledge.searchPlaceholder")}
          />
        </label>

        <div className="knowledge-studio__body">
          <div className="knowledge-studio__scroll">
            {filtered.length === 0 ? (
              <div className="knowledge-studio__empty">
                <p>{query.trim() ? t("knowledge.searchEmpty") : t("knowledge.empty")}</p>
              </div>
            ) : (
              <div className="knowledge-studio__grid">
                {filtered.map((entry) => (
                  <KnowledgeEntryCard
                    key={entry.id}
                    entry={entry}
                    active={previewEntry?.id === entry.id}
                    onSelect={() => setPreviewEntry(entry)}
                  />
                ))}
              </div>
            )}
          </div>
        </div>

        {previewEntry ? (
          <StudioCenterModal
            title={previewModalTitle(previewEntry)}
            titleId="knowledge-preview-title"
            onClose={() => setPreviewEntry(null)}
            wide
            className="knowledge-studio__preview-modal"
          >
            <KnowledgeDetail
              entry={previewEntry}
              busy={busyId === previewEntry.id}
              inModal
              onClose={() => setPreviewEntry(null)}
              onConfirm={async () => {
                setBusyId(previewEntry.id);
                try {
                  await onConfirm(previewEntry.id);
                  setPreviewEntry(null);
                } finally {
                  setBusyId(null);
                }
              }}
              onStale={async () => {
                setBusyId(previewEntry.id);
                try {
                  await onStale(previewEntry.id);
                  setPreviewEntry(null);
                } finally {
                  setBusyId(null);
                }
              }}
            />
          </StudioCenterModal>
        ) : null}
      </main>
    </div>
  );
}

export function KnowledgePanel({
  view,
  onClose,
  onConfirm,
  onStale,
  presentation = "page",
}: {
  view: KnowledgeView | null;
  onClose?: () => void;
  onConfirm: (id: string) => Promise<void> | void;
  onStale: (id: string) => Promise<void> | void;
  presentation?: "page" | "drawer";
}) {
  const t = useT();

  const studio = (
    <div className={`mode-center mode-center--knowledge${presentation === "page" ? " knowledge-studio-page" : ""}`}>
      {presentation === "drawer" && onClose ? (
        <div className="knowledge-studio__drawer-head">
          <h2>{t("knowledge.title")}</h2>
          <button type="button" className="knowledge-studio__drawer-close" onClick={onClose} aria-label={t("common.close")}>
            <X size={16} />
          </button>
        </div>
      ) : null}
      <KnowledgeStudio view={view} onConfirm={onConfirm} onStale={onStale} />
    </div>
  );

  if (presentation === "drawer" && onClose) {
    return <ResizableDrawer onClose={onClose}>{studio}</ResizableDrawer>;
  }

  return studio;
}
