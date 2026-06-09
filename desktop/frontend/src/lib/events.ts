/** CustomEvent names — keep dispatch/listener strings identical. */
export const DESKTOP_GIT_SETTINGS_EVENT = "ARCDESK:desktop-git-settings";
export const CODE_REVIEW_SETTINGS_EVENT = "ARCDESK:code-review-settings";
export const TAB_METAS_CHANGED_EVENT = "ARCDESK:tab-metas-changed";

export function notifyTabMetasChanged(): void {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent(TAB_METAS_CHANGED_EVENT));
}
