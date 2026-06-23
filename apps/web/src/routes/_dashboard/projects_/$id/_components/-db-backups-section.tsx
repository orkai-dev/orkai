import { Database, Loader2 } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { DatabaseBackup, ManagedDB } from "@/features/databases/types";
import { DbBackupConfigSection } from "./-db-backup-config-section";
import { DbBackupRow } from "./-db-backup-row";

export function DbBackupsSection({
  db,
  backups,
  isReady,
  onTriggerBackup,
  isTriggering,
  onRestore,
  isRestoring,
}: {
  db: ManagedDB;
  backups: DatabaseBackup[];
  isReady: boolean;
  onTriggerBackup: () => void;
  isTriggering: boolean;
  onRestore: (backupId: string) => void;
  isRestoring: boolean;
}) {
  const [showAllBackups, setShowAllBackups] = useState(false);

  return (
    <div className="space-y-6">
      <DbBackupConfigSection db={db} />

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Database className="h-4 w-4" /> Backup History
          </CardTitle>
          <Button
            size="sm"
            onClick={onTriggerBackup}
            disabled={isTriggering || !db.backup_s3_id || !isReady}
            title={
              !isReady
                ? "Database must be running"
                : !db.backup_s3_id
                  ? "Configure S3 storage first"
                  : "Run backup now"
            }
          >
            {isTriggering ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Database className="h-3.5 w-3.5" />
            )}{" "}
            Run Now
          </Button>
        </CardHeader>
        <CardContent>
          {backups.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No backups yet. Configure automatic backups above or click "Run Now" to create one.
            </p>
          ) : (
            <div className="space-y-2">
              {(showAllBackups ? backups : backups.slice(0, 10)).map((backup) => (
                <DbBackupRow
                  key={backup.id}
                  backup={backup}
                  onRestore={onRestore}
                  isRestoring={isRestoring}
                  dbReady={isReady}
                />
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
