import { lazy, Suspense } from "react";

export interface DiffProps {
  original: string;
  modified: string;
  language?: string;
  maxHeight?: number;
}

const LcsImpl = lazy(() => import("./editors/HljsDiff"));
const UnifiedImpl = lazy(() => import("./editors/UnifiedDiff"));

export function DiffView(props: DiffProps) {
  return (
    <Suspense fallback={<pre className="code code--loading">{props.modified}</pre>}>
      <LcsImpl {...props} />
    </Suspense>
  );
}

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
