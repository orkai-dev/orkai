import { useState } from "react";
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

export function DeleteProjectDialog({
  open,
  onOpenChange,
  project,
  apps,
  databases,
  loading,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  project: { name: string; namespace: string };
  apps: { name: string }[];
  databases: { name: string }[];
  loading: boolean;
  onConfirm: () => void;
}) {
  const [confirmName, setConfirmName] = useState("");
  const totalResources = apps.length + databases.length;

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) setConfirmName("");
        onOpenChange(v);
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Delete Project</DialogTitle>
          <DialogDescription>
            This will permanently destroy the namespace and all resources inside it.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {totalResources > 0 && (
            <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3">
              <p className="mb-2 text-xs font-medium text-destructive">
                The following will be permanently deleted:
              </p>
              <div className="space-y-1.5">
                {apps.length > 0 && (
                  <div className="flex items-start gap-2 text-sm">
                    <span className="mt-1.5 inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-destructive" />
                    <span>
                      <strong>{apps.length}</strong> app{apps.length > 1 ? "s" : ""}:{" "}
                      <span className="text-muted-foreground">
                        {apps.map((a) => a.name).join(", ")}
                      </span>
                    </span>
                  </div>
                )}
                {databases.length > 0 && (
                  <div className="flex items-start gap-2 text-sm">
                    <span className="mt-1.5 inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-destructive" />
                    <span>
                      <strong>{databases.length}</strong> database{databases.length > 1 ? "s" : ""}:{" "}
                      <span className="text-muted-foreground">
                        {databases.map((d) => d.name).join(", ")}
                      </span>
                    </span>
                  </div>
                )}
                <div className="flex items-start gap-2 text-sm">
                  <span className="mt-1.5 inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-destructive" />
                  <span>All volumes, secrets, domains, and environment variables</span>
                </div>
              </div>
            </div>
          )}

          {project.namespace && (
            <p className="text-xs text-muted-foreground">
              Namespace <code className="rounded bg-muted px-1 py-0.5">{project.namespace}</code>{" "}
              will be deleted from the cluster.
            </p>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="confirm-project-name" className="text-sm">
              Type <strong className="font-mono">{project.name}</strong> to confirm
            </Label>
            <Input
              id="confirm-project-name"
              className="border-border bg-background"
              placeholder={project.name}
              value={confirmName}
              onChange={(e) => setConfirmName(e.target.value)}
              autoComplete="off"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={confirmName !== project.name || loading}
            onClick={onConfirm}
          >
            {loading ? "Deleting..." : "Delete Project"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
