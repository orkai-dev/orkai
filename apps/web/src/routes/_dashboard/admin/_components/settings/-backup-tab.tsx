import { Link } from "@tanstack/react-router";
import { Archive, Database, Loader2, Play, Save } from "lucide-react";
import { useEffect, useState } from "react";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ToggleSwitch } from "@/components/ui/toggle-switch";
import { useResources } from "@/features/resources";
import type { SharedResource } from "@/features/resources/types";
import {
  useSaveSystemBackupConfig,
  useSystemBackupConfig,
  useSystemBackups,
  useTriggerSystemBackup,
} from "@/features/system-backup";
import type { SystemBackup } from "@/features/system-backup/types";
import { relativeTime } from "@/lib/format";

// ── Backup Tab ──────────────────────────────────────────────────────

const BACKUP_SCHEDULE_PRESETS = [
  { value: "0 */6 * * *", label: "Every 6 hours" },
  { value: "0 2 * * *", label: "Daily at 2:00 AM" },
  { value: "0 3 * * *", label: "Daily at 3:00 AM (recommended)" },
  { value: "0 2 * * 0", label: "Weekly (Sunday 2 AM)" },
  { value: "custom", label: "Custom" },
] as const;

function backupStatusVariant(status: string): "warning" | "success" | "destructive" | "secondary" {
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

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / 1024 ** i).toFixed(1)} ${units[i]}`;
}

export function BackupTab() {
  const { data: config, isLoading: configLoading } = useSystemBackupConfig();
  const saveConfig = useSaveSystemBackupConfig();
  const { data: rawBackups, isLoading: backupsLoading } = useSystemBackups();
  const backups = rawBackups ?? [];
  const triggerBackup = useTriggerSystemBackup();
  const { data: resources } = useResources("object_storage");
  const s3Resources = (resources ?? []).filter((r: SharedResource) => r.type === "object_storage");

  // ── Form state ──
  const [enabled, setEnabled] = useState(false);
  const [s3Id, setS3Id] = useState("");
  const [path, setPath] = useState("orkai-backups");
  const [retention, setRetention] = useState(30);
  const [schedulePreset, setSchedulePreset] = useState("0 3 * * *");
  const [customCron, setCustomCron] = useState("");
  const [showAllBackups, setShowAllBackups] = useState(false);

  // Sync from server
  useEffect(() => {
    if (config) {
      setEnabled(config.enabled);
      setS3Id(config.s3_id || "");
      setPath(config.path || "orkai-backups");
      setRetention(config.retention || 30);
      const match = BACKUP_SCHEDULE_PRESETS.find((p) => p.value === config.schedule);
      setSchedulePreset(match ? match.value : config.schedule ? "custom" : "0 3 * * *");
      if (!match && config.schedule) {
        setCustomCron(config.schedule);
      }
    }
  }, [config]);

  const resolvedSchedule = schedulePreset === "custom" ? customCron : schedulePreset;

  // Dirty detection
  const isDirty =
    !config ||
    enabled !== config.enabled ||
    (s3Id || "") !== (config.s3_id || "") ||
    (path || "orkai-backups") !== (config.path || "orkai-backups") ||
    retention !== (config.retention || 30) ||
    resolvedSchedule !== (config.schedule || "0 3 * * *");

  function handleSave() {
    saveConfig.mutate({
      enabled,
      s3_id: s3Id,
      schedule: resolvedSchedule,
      path: path || "orkai-backups",
      retention,
    });
  }

  if (configLoading) return <LoadingScreen />;

  return (
    <div className="space-y-6">
      {/* Backup Configuration */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Archive className="h-4 w-4" /> Backup Configuration
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-3">
            <ToggleSwitch
              checked={enabled}
              onChange={(v) => {
                setEnabled(v);
                if (!v) {
                  saveConfig.mutate({
                    enabled: false,
                    s3_id: s3Id,
                    schedule: resolvedSchedule,
                    path: path || "orkai-backups",
                    retention,
                  });
                }
              }}
            />
            <span className="text-sm font-medium">Automatic Backups</span>
          </div>

          {enabled && (
            <div className="space-y-4">
              {/* S3 Destination */}
              <div className="space-y-2">
                <Label className="text-sm">S3 Destination</Label>
                {s3Resources.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    No S3 storage configured.{" "}
                    <Link
                      to="/admin/resources"
                      search={{ tab: "object_storage" }}
                      className="text-primary underline underline-offset-4 hover:text-primary/80"
                    >
                      Add one in Resources.
                    </Link>
                  </p>
                ) : (
                  <Select value={s3Id} onValueChange={setS3Id}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select S3 resource" />
                    </SelectTrigger>
                    <SelectContent>
                      {s3Resources.map((r: SharedResource) => (
                        <SelectItem key={r.id} value={r.id}>
                          {r.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>

              {/* Directory */}
              <div className="space-y-2">
                <Label className="text-sm">Directory</Label>
                <Input
                  value={path}
                  onChange={(e) => setPath(e.target.value)}
                  placeholder="orkai-backups"
                  className="max-w-md font-mono"
                />
              </div>

              {/* Schedule */}
              <div className="space-y-2">
                <Label className="text-sm">Schedule</Label>
                <Select value={schedulePreset} onValueChange={setSchedulePreset}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {BACKUP_SCHEDULE_PRESETS.map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {schedulePreset === "custom" && (
                  <Input
                    value={customCron}
                    onChange={(e) => setCustomCron(e.target.value)}
                    placeholder="0 */6 * * *"
                    className="font-mono"
                  />
                )}
              </div>

              {/* Retention */}
              <div className="space-y-2">
                <Label className="text-sm">Retention</Label>
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    min={1}
                    value={retention}
                    onChange={(e) => setRetention(Number(e.target.value) || 1)}
                    className="w-24"
                  />
                  <span className="text-sm text-muted-foreground">backups</span>
                </div>
              </div>

              {/* Save */}
              <Button
                size="sm"
                onClick={handleSave}
                disabled={saveConfig.isPending || (!resolvedSchedule && enabled) || !isDirty}
              >
                {saveConfig.isPending ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Save className="h-3.5 w-3.5" />
                )}{" "}
                Save
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Backup History */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Database className="h-4 w-4" /> Backup History
          </CardTitle>
          <Button
            size="sm"
            onClick={() => triggerBackup.mutate()}
            disabled={triggerBackup.isPending || !config?.s3_id}
            title={!config?.s3_id ? "Configure S3 storage first" : "Run backup now"}
          >
            {triggerBackup.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Play className="h-3.5 w-3.5" />
            )}{" "}
            Backup Now
          </Button>
        </CardHeader>
        <CardContent>
          {backupsLoading ? (
            <LoadingScreen />
          ) : backups.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-8 text-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                <Archive className="h-5 w-5 text-muted-foreground" />
              </div>
              <p className="text-sm text-muted-foreground">No backups yet</p>
            </div>
          ) : (
            <div className="space-y-2">
              {(showAllBackups ? backups : backups.slice(0, 10)).map((backup: SystemBackup) => (
                <div
                  key={backup.id}
                  className="flex items-center justify-between rounded-md border px-3 py-2 text-sm"
                >
                  <div className="flex items-center gap-3">
                    <Badge variant={backupStatusVariant(backup.status)} className="text-xs">
                      {backup.status}
                    </Badge>
                    <span className="font-mono text-xs">{backup.file_name}</span>
                  </div>
                  <div className="flex items-center gap-4 text-xs text-muted-foreground">
                    <span>{formatBytes(backup.size_bytes)}</span>
                    <span>{relativeTime(backup.created_at)}</span>
                  </div>
                  {backup.status === "failed" && backup.error && (
                    <p className="mt-1 text-xs text-destructive">{backup.error}</p>
                  )}
                </div>
              ))}
              {backups.length > 10 && (
                <button
                  type="button"
                  onClick={() => setShowAllBackups(!showAllBackups)}
                  className="w-full py-2 text-center text-xs text-primary hover:underline"
                >
                  {showAllBackups ? "Show less" : `Show all ${backups.length} backups`}
                </button>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
