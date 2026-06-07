import { app } from "./bridge";
import { formatSelectionReference } from "./workspaceFilePreview";
import { formatWorkspaceReference } from "./workspaceDrag";

export async function addWorkspaceFileContentToChat(
  path: string,
  onAddToChat: (text: string) => void,
  truncatedLabel: string,
): Promise<void> {
  try {
    const file = await app.ReadFile(path);
    if (file.err || file.binary) {
      onAddToChat(formatWorkspaceReference(path, false));
      return;
    }
    const suffix = file.truncated ? `\n\n${truncatedLabel}` : "";
    onAddToChat(formatSelectionReference(path, file.body) + suffix);
  } catch {
    onAddToChat(formatWorkspaceReference(path, false));
  }
}
