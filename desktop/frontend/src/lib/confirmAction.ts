export interface NativeConfirmRequest {
  title?: string;
  message?: string;
  detail?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
}

export async function confirmAction(req: NativeConfirmRequest): Promise<boolean> {
  const { app } = await import("./bridge");
  try {
    return await app.ConfirmAction({
      title: req.title ?? "",
      message: req.message ?? "",
      detail: req.detail ?? "",
      confirmLabel: req.confirmLabel ?? "",
      cancelLabel: req.cancelLabel ?? "",
      destructive: req.destructive ?? false,
    });
  } catch {
    const message = [req.title, req.message, req.detail].filter(Boolean).join("\n\n");
    return window.confirm(message || req.message || "");
  }
}
