import type { DiffProps } from "../DiffView";
import { diffLines } from "../../lib/diff";
import { DiffRows } from "./DiffRows";

export default function HljsDiff({ original, modified, language, maxHeight }: DiffProps) {
  const rows = diffLines(original, modified);
  return <DiffRows rows={rows} language={language} maxHeight={maxHeight} />;
}
