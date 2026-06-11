/** Fail-fast wrapper for Wails IPC calls that must not hang the UI forever. */
export async function withIPCTimeout<T>(
  promise: Promise<T>,
  ms: number,
  label: string,
): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      promise,
      new Promise<T>((_, reject) => {
        timer = window.setTimeout(
          () => reject(new Error(`${label} timed out after ${ms}ms`)),
          ms,
        );
      }),
    ]);
  } finally {
    if (timer !== undefined) window.clearTimeout(timer);
  }
}

export const IPC_META_TIMEOUT_MS = 15_000;
export const IPC_HYDRATE_TIMEOUT_MS = 20_000;
export const IPC_BALANCE_TIMEOUT_MS = 8_000;
export const IPC_ONBOARDING_TIMEOUT_MS = 8_000;
export const IPC_LIST_DIR_TIMEOUT_MS = 12_000;
export const BOOT_READY_POLL_MS = 100;
export const BOOT_READY_MAX_POLLS = 180; // 90s
