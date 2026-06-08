import { unifiedDiffToRows } from "../../lib/unifiedDiff";
import { DiffRows } from "./DiffRows";

export default function UnifiedDiff({
  unified,
  language,
  maxHeight,
}: {
  unified: string;
  language?: string;
  maxHeight?: number;
}) {
  const rows = unifiedDiffToRows(unified);
  return <DiffRows rows={rows} language={language} maxHeight={maxHeight} />;
}
