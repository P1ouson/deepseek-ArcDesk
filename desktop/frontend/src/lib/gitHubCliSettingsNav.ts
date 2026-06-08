export type GitHubCliSettingsNavDetail = {
  runCheck?: boolean;
  enableCheck?: boolean;
};

export const GITHUB_CLI_SETTINGS_EVENT = "ARCDESK:open-github-cli-settings";

export function openGitHubCliSettings(
  detail: GitHubCliSettingsNavDetail = { runCheck: true, enableCheck: true },
): void {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent(GITHUB_CLI_SETTINGS_EVENT, { detail }));
}
