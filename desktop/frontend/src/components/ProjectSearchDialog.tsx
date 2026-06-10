import { useEffect, useMemo, useRef } from "react";
import { createPortal } from "react-dom";
import { Folder, MessageSquare, Search, X } from "lucide-react";
import { asArray } from "../lib/array";
import { getLocale, useT } from "../lib/i18n";
import type { ProjectNode } from "../lib/types";

function filterProjectTree(tree: ProjectNode[], query: string): ProjectNode[] {
  const q = query.trim().toLowerCase();
  if (!q) return tree;
  const matches = (node: ProjectNode) =>
    [node.label, node.root, node.topicId].some((value) => (value ?? "").toLowerCase().includes(q));
  const filterNode = (node: ProjectNode): ProjectNode | null => {
    const children = asArray(node.children)
      .map(filterNode)
      .filter((child): child is ProjectNode => child !== null);
    if (matches(node) || children.length > 0) return { ...node, children };
    return null;
  };
  return tree.map(filterNode).filter((node): node is ProjectNode => node !== null);
}

function topicActivityLabel(ms: number): string {
  if (ms <= 0) return "";
  const delta = Date.now() - ms;
  const locale = getLocale();
  const rtf = new Intl.RelativeTimeFormat(locale === "zh" ? "zh-CN" : "en", { numeric: "auto" });
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (delta < minute) return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  if (delta < hour) return rtf.format(-Math.max(1, Math.round(delta / minute)), "minute");
  if (delta < day) return rtf.format(-Math.round(delta / hour), "hour");
  if (delta < 7 * day) return rtf.format(-Math.round(delta / day), "day");
  return new Date(ms).toLocaleDateString();
}

function topicRows(node: ProjectNode): ProjectNode[] {
  return asArray(node.children).filter((child) => child.kind === "topic" || child.kind === "global_topic");
}

function folderScope(node: ProjectNode): "global" | "project" {
  return node.kind === "global_folder" ? "global" : "project";
}

export function ProjectSearchDialog({
  open,
  query,
  tree,
  activeScope,
  activeWorkspaceRoot,
  activeTopicId,
  onQueryChange,
  onClose,
  onOpenTopic,
}: {
  open: boolean;
  query: string;
  tree: ProjectNode[];
  activeScope?: string;
  activeWorkspaceRoot?: string;
  activeTopicId?: string;
  onQueryChange: (query: string) => void;
  onClose: () => void;
  onOpenTopic: (scope: string, workspaceRoot: string, topicId: string) => Promise<void> | void;
}) {
  const t = useT();
  const inputRef = useRef<HTMLInputElement | null>(null);
  const filteredTree = useMemo(() => filterProjectTree(tree, query), [query, tree]);

  useEffect(() => {
    if (!open) return;
    inputRef.current?.focus();
  }, [open]);

  if (!open) return null;

  const selectTopic = (node: ProjectNode) => {
    const scope = node.kind === "global_topic" ? "global" : "project";
    void onOpenTopic(scope, node.root ?? "", node.topicId ?? "");
    onClose();
  };

  const isTopicActive = (node: ProjectNode) => {
    const scope = node.kind === "global_topic" ? "global" : "project";
    return (
      activeTopicId === node.topicId &&
      activeScope === scope &&
      (scope === "global" || activeWorkspaceRoot === node.root)
    );
  };

  return createPortal(
    <div className="modal-backdrop modal-backdrop--static project-search-dialog-backdrop" role="presentation">
      <div
        className="modal project-search-dialog wails-no-drag"
        role="dialog"
        aria-modal="true"
        aria-labelledby="project-search-dialog-title"
      >
        <button
          type="button"
          className="project-search-dialog__close"
          onClick={onClose}
          aria-label={t("common.close")}
        >
          <X size={16} strokeWidth={2} aria-hidden="true" />
        </button>
        <div className="project-search-dialog__head">
          <h3 className="project-search-dialog__title" id="project-search-dialog-title">
            {t("projectTree.searchTitle")}
          </h3>
          <label className="project-search-dialog__search">
            <Search size={15} aria-hidden="true" />
            <input
              ref={inputRef}
              value={query}
              placeholder={t("projectTree.searchPlaceholder")}
              onChange={(event) => onQueryChange(event.target.value)}
            />
          </label>
          <p className="project-search-dialog__hint">{t("projectTree.searchHint")}</p>
        </div>
        <div className="project-search-dialog__body">
          {filteredTree.length === 0 ? (
            <div className="project-search-dialog__empty">{t("projectTree.emptyNoMatch")}</div>
          ) : (
            filteredTree.map((group) => {
              if (group.kind === "topic" || group.kind === "global_topic") {
                const label = (group.label || group.topicId || "Untitled").replace(/^●\s*/, "");
                return (
                  <button
                    key={group.key}
                    type="button"
                    className={`project-search-dialog__topic${isTopicActive(group) ? " project-search-dialog__topic--active" : ""}`}
                    onClick={() => selectTopic(group)}
                  >
                    <MessageSquare size={13} aria-hidden="true" />
                    <span className="project-search-dialog__topic-copy">
                      <span className="project-search-dialog__topic-label">{label}</span>
                      {group.lastActivityAt ? (
                        <span className="project-search-dialog__topic-meta">{topicActivityLabel(group.lastActivityAt)}</span>
                      ) : null}
                    </span>
                  </button>
                );
              }

              const topics = topicRows(group);
              const scope = folderScope(group);
              const projectLabel = group.label || (scope === "global" ? "Global" : "Untitled");

              return (
                <section key={group.key} className="project-search-dialog__group">
                  <div className="project-search-dialog__project">
                    <Folder size={13} aria-hidden="true" />
                    <span className="project-search-dialog__project-label">{projectLabel}</span>
                    {group.root ? <span className="project-search-dialog__project-path">{group.root}</span> : null}
                  </div>
                  {topics.length === 0 ? (
                    <div className="project-search-dialog__group-empty">{t("projectTree.searchNoSessions")}</div>
                  ) : (
                    topics.map((topic) => {
                      const label = (topic.label || topic.topicId || "Untitled").replace(/^●\s*/, "");
                      return (
                        <button
                          key={topic.key}
                          type="button"
                          className={`project-search-dialog__topic${isTopicActive(topic) ? " project-search-dialog__topic--active" : ""}`}
                          onClick={() => selectTopic(topic)}
                        >
                          <MessageSquare size={13} aria-hidden="true" />
                          <span className="project-search-dialog__topic-copy">
                            <span className="project-search-dialog__topic-label">{label}</span>
                            {topic.lastActivityAt ? (
                              <span className="project-search-dialog__topic-meta">{topicActivityLabel(topic.lastActivityAt)}</span>
                            ) : null}
                          </span>
                        </button>
                      );
                    })
                  )}
                </section>
              );
            })
          )}
        </div>
      </div>
    </div>,
    document.body,
  );
}
