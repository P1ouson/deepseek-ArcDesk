import { lazy, Suspense } from "react";

const UnifiedImpl = lazy(() => import("./editors/UnifiedDiff"));

export function UnifiedDiffView({
  unified,
  language,
  maxHeight,
}: {
  unified: string;
  language?: string;
  maxHeight?: number;
}) {
  return (
    <Suspense fallback={<pre className="code code--loading">{unified.slice(0, 400)}</pre>}>
      <UnifiedImpl unified={unified} language={language} maxHeight={maxHeight} />
    </Suspense>
  );
}
