export type LocalStorageStore<T> = {
  get: () => T;
  set: (value: T) => void;
  remove: () => void;
};

export function createLocalStorageStore<T>(
  key: string,
  parse: (raw: string) => T | undefined,
  serialize: (value: T) => string,
  defaultValue: T,
): LocalStorageStore<T> {
  const get = (): T => {
    if (typeof window === "undefined") return defaultValue;
    try {
      const raw = window.localStorage.getItem(key);
      if (raw == null) return defaultValue;
      return parse(raw) ?? defaultValue;
    } catch {
      return defaultValue;
    }
  };

  const set = (value: T) => {
    if (typeof window === "undefined") return;
    try {
      window.localStorage.setItem(key, serialize(value));
    } catch {
      /* quota / private mode */
    }
  };

  const remove = () => {
    if (typeof window === "undefined") return;
    try {
      window.localStorage.removeItem(key);
    } catch {
      /* ignore */
    }
  };

  return { get, set, remove };
}

export function stringStore(key: string, defaultValue = ""): LocalStorageStore<string> {
  return createLocalStorageStore(
    key,
    (raw) => raw.trim(),
    (value) => value.trim(),
    defaultValue,
  );
}
