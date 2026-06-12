/** Normalize unknown thrown/rejected values to a user-facing message string. */
export function toErrorMessage(err: unknown, fallback = ""): string {
  if (err instanceof Error) return err.message || fallback;
  if (typeof err === "string") return err || fallback;
  if (err == null) return fallback;
  return String(err);
}
