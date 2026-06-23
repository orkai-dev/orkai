import type { UseMutationResult } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
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
import type { ManagedDB } from "@/features/databases/types";
import { ENGINE_LABELS } from "@/lib/constants";

export function DbDeleteDialog({
  db,
  projectId,
  open,
  onOpenChange,
  confirmName,
  onConfirmNameChange,
  deleteDb,
}: {
  db: ManagedDB;
  projectId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  confirmName: string;
  onConfirmNameChange: (name: string) => void;
  deleteDb: UseMutationResult<void, Error, undefined, unknown>;
}) {
  const navigate = useNavigate();

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onConfirmNameChange("");
        onOpenChange(next);
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Delete Database</DialogTitle>
          <DialogDescription>
            Permanently delete <strong>{db.name}</strong> ({ENGINE_LABELS[db.engine]})? All data
            will be lost.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-1.5 py-2">
          <Label htmlFor="confirm-db-name" className="text-sm">
            Type <strong className="font-mono">{db.name}</strong> to confirm
          </Label>
          <Input
            id="confirm-db-name"
            placeholder={db.name}
            value={confirmName}
            onChange={(e) => onConfirmNameChange(e.target.value)}
            autoComplete="off"
          />
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={confirmName !== db.name || deleteDb.isPending}
            onClick={() =>
              deleteDb.mutate(undefined, {
                onSuccess: () => navigate({ to: "/projects/$id", params: { id: projectId } }),
              })
            }
          >
            {deleteDb.isPending ? "Deleting..." : "Delete"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
