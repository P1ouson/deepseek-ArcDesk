/** Last path segment of a model id, e.g. "amazon/nova-2-lite" → "nova-2-lite". */
export function modelShortLabel(modelId: string): string {
  const trimmed = modelId.trim();
  if (!trimmed) return "";
  const slash = trimmed.lastIndexOf("/");
  return slash >= 0 ? trimmed.slice(slash + 1) : trimmed;
}

/** Short display label for an internal provider/model ref. */
export function modelLabelFromRef(ref: string): string {
  if (!ref) return "";
  const slash = ref.indexOf("/");
  const modelId = slash >= 0 ? ref.slice(slash + 1) : ref;
  return modelShortLabel(modelId);
}
