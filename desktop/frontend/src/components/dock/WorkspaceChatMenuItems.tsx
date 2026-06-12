import type { ReactNode } from "react";
import { MessageSquarePlus } from "lucide-react";
import { useT } from "../../lib/i18n";
import { FloatingMenuItems } from "../FloatingMenu";

export function WorkspaceChatMenuItems({
  path,
  onAddReference,
  onAddContent,
  extra,
}: {
  path: string;
  onAddReference: (path: string) => void;
  onAddContent: (path: string) => void;
  extra?: ReactNode;
}) {
  const t = useT();
  return (
    <>
      {extra}
      <FloatingMenuItems
        items={[
          {
            label: t("workspace.addFileReferenceToChat"),
            icon: <MessageSquarePlus size={14} />,
            onSelect: () => onAddReference(path),
          },
          {
            label: t("workspace.addFileContentToChat"),
            icon: <MessageSquarePlus size={14} />,
            onSelect: () => void onAddContent(path),
          },
        ]}
      />
    </>
  );
}
