import { Link } from "@tanstack/react-router";
import { Database, Loader2, Save } from "lucide-react";
import { useEffect, useState } from "react";
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
import { useUpdateBackupConfig } from "@/features/databases";
import type { ManagedDB } from "@/features/databases/types";
import { useResources } from "@/features/resources";
import type { SharedResource } from "@/features/resources/types";
import { BACKUP_SCHEDULE_PRESETS } from "./-db-helpers";

export function DbBackupConfigSection({ db }: { db: ManagedDB }) {
  const updateBackup = useUpdateBackupConfig(db.id);
  const { data: resources } = useResources("object_storage");
  const s3Resources = (resources ?? []).filter((r: SharedResource) => r.type === "object_storage");

  const [enabled, setEnabled] = useState(db.backup_enabled);
  const [s3Id, setS3Id] = useState(db.backup_s3_id || "");
  const [schedulePreset, setSchedulePreset] = useState(() => {
    const match = BACKUP_SCHEDULE_PRESETS.find((p) => p.value === db.backup_schedule);
    return match ? match.value : db.backup_schedule ? "custom" : "0 2 * * *";
  });
  const [customCron, setCustomCron] = useState(
    BACKUP_SCHEDULE_PRESETS.some((p) => p.value === db.backup_schedule)
      ? ""
      : db.backup_schedule || "",
  );

  useEffect(() => {
    setEnabled(db.backup_enabled);
    setS3Id(db.backup_s3_id || "");
    const match = BACKUP_SCHEDULE_PRESETS.find((p) => p.value === db.backup_schedule);
    setSchedulePreset(match ? match.value : db.backup_schedule ? "custom" : "0 2 * * *");
    if (!match && db.backup_schedule) {
      setCustomCron(db.backup_schedule);
    }
  }, [db.backup_enabled, db.backup_s3_id, db.backup_schedule]);

  const resolvedSchedule = schedulePreset === "custom" ? customCron : schedulePreset;

  const isDirty =
    enabled !== db.backup_enabled ||
    resolvedSchedule !== (db.backup_schedule || "0 2 * * *") ||
    (s3Id || "") !== (db.backup_s3_id || "");

  function handleSave() {
    updateBackup.mutate({
      enabled,
      schedule: resolvedSchedule,
      s3_id: s3Id || undefined,
    });
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Database className="h-4 w-4" /> Backup Configuration
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center gap-3">
          <ToggleSwitch checked={enabled} onChange={setEnabled} />
          <span className="text-sm font-medium">Automatic Backups</span>
        </div>

        {enabled && (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label className="text-sm">Destination</Label>
              {s3Resources.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No S3 storage configured.{" "}
                  <Link
                    to="/admin/resources"
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
          </div>
        )}

        {isDirty && (
          <Button
            size="sm"
            onClick={handleSave}
            disabled={updateBackup.isPending || (!resolvedSchedule && enabled)}
          >
            {updateBackup.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Save className="h-3.5 w-3.5" />
            )}{" "}
            Save
          </Button>
        )}
      </CardContent>
    </Card>
  );
}
