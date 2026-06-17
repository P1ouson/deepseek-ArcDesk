import { app } from "./bridge";
import type { BalanceInfo } from "./types";

/** Prefer live session balance; fall back across open tabs for the settings overview. */
export async function fetchSettingsBalance(
  activeTabId?: string,
  seeded?: BalanceInfo,
): Promise<BalanceInfo | null> {
  if (seeded?.available && seeded.display) return seeded;

  const tryTab = async (tabId: string): Promise<BalanceInfo | null> => {
    try {
      const balance = await app.BalanceForTab(tabId);
      return balance?.available && balance.display ? balance : null;
    } catch {
      return null;
    }
  };

  if (activeTabId) {
    const fromActive = await tryTab(activeTabId);
    if (fromActive) return fromActive;
  }

  const tabs = await app.ListTabs().catch(() => []);
  for (const tab of tabs) {
    if (tab.id === activeTabId) continue;
    const balance = await tryTab(tab.id);
    if (balance) return balance;
  }

  try {
    const balance = await app.Balance();
    if (balance?.available && balance.display) return balance;
    return balance ?? seeded ?? null;
  } catch {
    return seeded ?? null;
  }
}
