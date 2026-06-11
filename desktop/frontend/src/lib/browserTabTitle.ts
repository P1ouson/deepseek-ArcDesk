export function browserTabTitle(url: string, index = 0): string {
  try {
    const parsed = new URL(url);
    const host = parsed.hostname.toLowerCase();
    if (host === "localhost" || host === "127.0.0.1" || host === "::1" || host === "[::1]") {
      const port = parsed.port || (parsed.protocol === "https:" ? "443" : "80");
      return `${host}:${port}`;
    }
    return host || `Tab ${index + 1}`;
  } catch {
    return `Tab ${index + 1}`;
  }
}
