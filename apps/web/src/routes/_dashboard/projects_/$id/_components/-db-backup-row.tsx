import { Loader2, RotateCcw } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { DatabaseBackup } from "@/features/databases/types";
import { formatDurationBetween } from "@/lib/format";
import { backupStatusVariant, formatBytes } from "./-db-helpers";

export function DbRestoreBackupDialog({
  backup,
  open,
  onOpenChange,
  onRestore,
  isRestoring,
}: {
  backup: DatabaseBackup;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onRestore: (backupId: string) => void;
  isRestoring: boolean;
}) {
  const [confirmText, setConfirmText] = useState("");

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setConfirmText("");
        onOpenChange(next);
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Restore Database</DialogTitle>
          <DialogDescription>
            This will overwrite the current database with the backup from{" "}
            <strong>{new Date(backup.created_at).toLocaleString()}</strong>. This action cannot be
            undone.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-2 py-2">
          <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3">
            <p className="text-xs text-destructive">
              All current data will be replaced with the backup contents.
            </p>
          </div>
          <Label>
            Type <strong>RESTORE</strong> to confirm
          </Label>
          <Input
            value={confirmText}
            onChange={(e) => setConfirmText(e.target.value)}
            placeholder="RESTORE"
            className="font-mono"
          />
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={confirmText !== "RESTORE" || isRestoring}
            onClick={() => {
              onRestore(backup.id);
              onOpenChange(false);
              setConfirmText("");
            }}
          >
            <RotateCcw className="h-3.5 w-3.5" /> Restore
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function DbBackupRow({
  backup,
  onRestore,
  isRestoring,
  dbReady,
}: {
  backup: DatabaseBackup;
  onRestore: (backupId: string) => void;
  isRestoring: boolean;
  dbReady: boolean;
}) {
  const [showConfirm, setShowConfirm] = useState(false);
  const restoreRunning = backup.restore_status === "running";

  return (
    <>
      <div className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
        <div className="flex items-center gap-3">
          <Badge variant={backupStatusVariant(backup.status)} className="text-xs">
            {backup.status}
          </Badge>
          {backup.restore_status && (
            <Badge
              variant={
                backup.restore_status === "completed"
                  ? "outline"
                  : backup.restore_status === "failed"
                    ? "destructive"
                    : "secondary"
              }
              className="text-xs"
            >
              {restoreRunning ? (
                <span className="flex items-center gap-1">
                  <Loader2 className="h-3 w-3 animate-spin" /> restoring
                </span>
              ) : (
                `restore ${backup.restore_status}`
              )}
            </Badge>
          )}
          <span className="text-xs text-muted-foreground">
            {new Date(backup.created_at).toLocaleString()}
          </span>
        </div>
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span>{formatBytes(backup.size_bytes)}</span>
          <span>{formatDurationBetween(backup.started_at, backup.finished_at)}</span>
          {backup.status === "completed" && !restoreRunning && (
            <Button
              variant="ghost"
              size="sm"
              className="h-6 gap-1 px-2 text-xs"
              onClick={() => setShowConfirm(true)}
              disabled={isRestoring || !dbReady}
              title={!dbReady ? "Database must be running to restore" : "Restore from this backup"}
            >
              <RotateCcw className="h-3 w-3" /> Restore
            </Button>
          )}
        </div>
      </div>

      <DbRestoreBackupDialog
        backup={backup}
        open={showConfirm}
        onOpenChange={setShowConfirm}
        onRestore={onRestore}
        isRestoring={isRestoring}
      />
    </>
  );
}
