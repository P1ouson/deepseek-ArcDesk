import { HistoryPanel } from "./HistoryPanel";
import { MemoryPanel } from "./MemoryPanel";
import type { HistoryMessage, MemoryView, SessionMeta } from "../lib/types";

export type SettingsDataModalState =
  | { kind: "history"; sessions: SessionMeta[] }
  | { kind: "trash"; sessions: SessionMeta[] }
  | { kind: "memory"; view: MemoryView };

export function SettingsWorkspaceDataModal({
  state,
  running,
  onClose,
  onResume,
  onPreview,
  onDelete,
  onRename,
  onRestore,
  onPurge,
  onPurgeAll,
  onRemember,
  onForget,
  onSaveDoc,
}: {
  state: SettingsDataModalState;
  running: boolean;
  onClose: () => void;
  onResume: (session: SessionMeta) => void;
  onPreview: (path: string) => Promise<HistoryMessage[]>;
  onDelete: (path: string) => void;
  onRename: (path: string, title: string) => void;
  onRestore: (path: string) => void;
  onPurge: (path: string) => void;
  onPurgeAll: (paths: string[]) => void;
  onRemember: (scope: string, note: string) => Promise<void> | void;
  onForget: (name: string) => Promise<void> | void;
  onSaveDoc: (path: string, body: string) => Promise<void> | void;
}) {
  if (state.kind === "memory") {
    return (
      <MemoryPanel
        view={state.view}
        presentation="modal"
        onClose={onClose}
        onRemember={onRemember}
        onForget={onForget}
        onSaveDoc={onSaveDoc}
      />
    );
  }

  return (
    <HistoryPanel
      kind={state.kind}
      sessions={state.sessions}
      running={running}
      presentation="modal"
      onResume={onResume}
      onPreview={onPreview}
      onDelete={onDelete}
      onRename={onRename}
      onRestore={onRestore}
      onPurge={onPurge}
      onPurgeAll={onPurgeAll}
      onClose={onClose}
    />
  );
}
