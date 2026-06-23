/** Parse Kubernetes CPU quantity to millicores. */
export function parseToMillicores(raw: string): number {
  if (!raw) return 0;
  const s = raw.trim();
  if (s.endsWith("n")) return Number.parseFloat(s) / 1_000_000;
  if (s.endsWith("m")) return Number.parseFloat(s);
  return Number.parseFloat(s) * 1000 || 0;
}

/** Parse Kubernetes memory quantity to MiB. */
export function parseToMiB(raw: string): number {
  if (!raw) return 0;
  const s = raw.trim();
  if (s.endsWith("Ki")) return Number.parseFloat(s) / 1024;
  if (s.endsWith("Mi")) return Number.parseFloat(s);
  if (s.endsWith("Gi")) return Number.parseFloat(s) * 1024;
  return Number.parseFloat(s) / (1024 * 1024) || 0;
}

/** Format millicores for display. */
export function fmtCPU(millis: number): string {
  if (millis >= 1000) return `${(millis / 1000).toFixed(1)} cores`;
  return `${Math.round(millis)}m`;
}

/** Format MiB for display. */
export function fmtMem(mib: number): string {
  if (mib >= 1024) return `${(mib / 1024).toFixed(1)} Gi`;
  if (mib >= 1) return `${Math.round(mib)} Mi`;
  return `${Math.round(mib * 1024)} Ki`;
}

/** Parse CPU or memory resource string to a normalized numeric value (millicores or MiB). */
export function parseResourceValue(val: string): number {
  if (!val) return 0;
  if (val.endsWith("m")) return Number.parseFloat(val);
  if (val.endsWith("n")) return Number.parseFloat(val) / 1_000_000;
  if (/^\d+(\.\d+)?$/.test(val)) return Number.parseFloat(val) * 1000;
  if (val.endsWith("Ki")) return Number.parseFloat(val) / 1024;
  if (val.endsWith("Mi")) return Number.parseFloat(val);
  if (val.endsWith("Gi")) return Number.parseFloat(val) * 1024;
  return Number.parseFloat(val) || 0;
}

/** Percentage string from used/total resource quantities. */
export function pctString(used: string, total: string): string {
  const u = parseResourceValue(used);
  const t = parseResourceValue(total);
  if (t === 0) return "0%";
  return `${Math.round((u / t) * 100)}%`;
}

/** Percentage number (0–100) from used/total resource quantities. */
export function pctNumber(used: string, total: string): number {
  const u = parseResourceValue(used);
  const t = parseResourceValue(total);
  if (t === 0) return 0;
  return Math.min(100, Math.round((u / t) * 100));
}
