export function DockEmptyState({
  title,
  hint,
  searchMode = false,
}: {
  title: string;
  hint?: string;
  searchMode?: boolean;
}) {
  return (
    <li className={`dock-panel__empty${searchMode ? " dock-panel__search-empty" : ""}`}>
      <span>{title}</span>
      {hint ? <small>{hint}</small> : null}
    </li>
  );
}
