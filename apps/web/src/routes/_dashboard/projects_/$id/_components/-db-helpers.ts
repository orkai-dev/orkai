import { toast } from "sonner";

export function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text);
  toast.success("Copied");
}

export function formatCPU(raw: string): string {
  if (!raw) return "0m";
  if (raw.endsWith("n")) {
    const n = Number.parseInt(raw.slice(0, -1), 10);
    return `${Math.round(n / 1_000_000)}m`;
  }
  if (raw.endsWith("m")) return raw;
  return `${Math.round(Number.parseFloat(raw) * 1000)}m`;
}

export function formatMem(raw: string): string {
  if (!raw) return "0Mi";
  if (raw.endsWith("Ki")) {
    const ki = Number.parseInt(raw.slice(0, -2), 10);
    return ki >= 1024 ? `${(ki / 1024).toFixed(0)}Mi` : `${ki}Ki`;
  }
  if (raw.endsWith("Mi") || raw.endsWith("Gi")) return raw;
  const b = Number.parseInt(raw, 10);
  if (b >= 1024 * 1024 * 1024) return `${(b / (1024 * 1024 * 1024)).toFixed(1)}Gi`;
  if (b >= 1024 * 1024) return `${Math.round(b / (1024 * 1024))}Mi`;
  return `${Math.round(b / 1024)}Ki`;
}

export function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / 1024 ** i).toFixed(1)} ${units[i]}`;
}

export function backupStatusVariant(
  status: string,
): "warning" | "success" | "destructive" | "secondary" {
  switch (status) {
    case "pending":
    case "running":
      return "warning";
    case "completed":
      return "success";
    case "failed":
      return "destructive";
    default:
      return "secondary";
  }
}

export const BACKUP_SCHEDULE_PRESETS = [
  { value: "0 */6 * * *", label: "Every 6 hours" },
  { value: "0 2 * * *", label: "Daily at 2:00 AM (recommended)" },
  { value: "0 4 * * *", label: "Daily at 4:00 AM" },
  { value: "0 2 * * 0", label: "Weekly (Sunday 2 AM)" },
  { value: "custom", label: "Custom" },
] as const;

export const ENGINE_PROTOCOL: Record<string, string> = {
  postgres: "postgresql",
  mysql: "mysql",
  mariadb: "mysql",
  redis: "redis",
  valkey: "redis",
  mongo: "mongodb",
};

export const ENGINE_DEFAULT_PORT: Record<string, number> = {
  postgres: 30432,
  mysql: 30306,
  mariadb: 30306,
  redis: 30379,
  valkey: 30380,
  mongo: 30017,
};
