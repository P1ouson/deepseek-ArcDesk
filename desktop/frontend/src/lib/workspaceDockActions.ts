import type { DragEvent, MouseEvent } from "react";
import { formatWorkspaceReference, WORKSPACE_REF_DRAG_TYPE } from "./workspaceDrag";
import { addWorkspaceFileContentToChat } from "./workspaceAddToChat";

export function startWorkspaceDrag(event: DragEvent<HTMLElement>, path: string, isDir = false) {
  const ref = formatWorkspaceReference(path, isDir);
  event.dataTransfer.effectAllowed = "copy";
  event.dataTransfer.setData(WORKSPACE_REF_DRAG_TYPE, JSON.stringify({ path, isDir }));
  event.dataTransfer.setData("text/plain", ref);
}

export function openWorkspaceRowMenu<T extends { x: number; y: number }>(
  event: MouseEvent<HTMLElement>,
  payload: Omit<T, "x" | "y">,
  setMenu: (menu: T | null) => void,
) {
  event.preventDefault();
  event.stopPropagation();
  setMenu({ ...payload, x: event.clientX, y: event.clientY } as T);
}

export async function addWorkspaceReferenceToChat(
  path: string,
  onAddToChat: ((text: string) => void) | undefined,
  clearMenu: () => void,
  isDir = false,
) {
  onAddToChat?.(formatWorkspaceReference(path, isDir));
  clearMenu();
}

export async function addWorkspaceFileToChat(
  path: string,
  onAddToChat: ((text: string) => void) | undefined,
  clearMenu: () => void,
  truncatedLabel: string,
) {
  clearMenu();
  if (!onAddToChat) return;
  await addWorkspaceFileContentToChat(path, onAddToChat, truncatedLabel);
}
