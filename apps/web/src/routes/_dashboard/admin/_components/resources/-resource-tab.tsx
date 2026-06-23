import { useCallback, useState } from "react";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  useDeleteResource,
  useGenerateSSHKey,
  useGitHubStatus,
  useResources,
  useTestResource,
} from "@/features/resources";
import type { SharedResource } from "@/features/resources/types";
import { DnsManagerSheet } from "./-dns-manager";
import { ResourceCard } from "./-resource-card";
import { ResourceSheet } from "./-resource-sheet";
import type { ResourceType } from "./-resources.config";

export function ResourceTab({
  type,
  icon: Icon,
}: {
  type: ResourceType;
  icon: React.ComponentType<{ className?: string }>;
}) {
  const { data, isLoading } = useResources(type);
  const { data: ghStatus } = useGitHubStatus();
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editing, setEditing] = useState<SharedResource | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SharedResource | null>(null);
  const [dnsTarget, setDnsTarget] = useState<SharedResource | null>(null);
  const deleteMutation = useDeleteResource();
  const testMutation = useTestResource();
  const generateSSH = useGenerateSSHKey();
  const [sshAlgorithm, setSSHAlgorithm] = useState("ed25519");
  const [showGenerate, setShowGenerate] = useState(false);

  const openCreate = useCallback(() => {
    setEditing(null);
    setSheetOpen(true);
  }, []);

  const openEdit = useCallback((r: SharedResource) => {
    setEditing(r);
    setSheetOpen(true);
  }, []);

  if (isLoading) return <LoadingScreen variant="detail" />;

  const resources = data ?? [];

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-end gap-2">
        {type === "ssh_key" && (
          <Button size="sm" variant="outline" onClick={() => setShowGenerate(true)}>
            Generate Key
          </Button>
        )}
        <Button size="sm" onClick={openCreate}>
          Add
        </Button>
      </div>

      {/* SSH Key Generate Sheet */}
      {type === "ssh_key" && (
        <Sheet open={showGenerate} onOpenChange={setShowGenerate}>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>Generate SSH Key</SheetTitle>
              <SheetDescription>
                Create a new SSH key pair. The private key will be stored securely.
              </SheetDescription>
            </SheetHeader>
            <div className="mt-6 space-y-4">
              <div className="space-y-1.5">
                <Label className="text-sm font-medium">Algorithm</Label>
                <Select value={sshAlgorithm} onValueChange={setSSHAlgorithm}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ed25519">Ed25519 (recommended)</SelectItem>
                    <SelectItem value="rsa-4096">RSA 4096-bit</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  Ed25519 is faster and more secure. RSA 4096 for legacy compatibility.
                </p>
              </div>
              <Button
                className="w-full"
                onClick={() => {
                  generateSSH.mutate(
                    { algorithm: sshAlgorithm },
                    { onSuccess: () => setShowGenerate(false) },
                  );
                }}
                disabled={generateSSH.isPending}
              >
                {generateSSH.isPending ? "Generating..." : "Generate Key Pair"}
              </Button>
            </div>
          </SheetContent>
        </Sheet>
      )}

      {resources.length === 0 ? (
        <EmptyState
          icon={Icon as any}
          message={`No ${type.replace("_", " ")} resources yet`}
          actionLabel="Add"
          onAction={openCreate}
        />
      ) : (
        resources.map((r) => (
          <ResourceCard
            key={r.id}
            resource={r}
            type={type}
            icon={Icon}
            onTest={() => testMutation.mutate(r.id)}
            onEdit={() => openEdit(r)}
            onDelete={() => setDeleteTarget(r)}
            onManageDNS={type === "cloud_account" ? () => setDnsTarget(r) : undefined}
            testing={testMutation.isPending && testMutation.variables === r.id}
            installUrl={ghStatus?.install_url}
          />
        ))
      )}

      <ResourceSheet open={sheetOpen} onOpenChange={setSheetOpen} type={type} resource={editing} />

      <DnsManagerSheet
        open={!!dnsTarget}
        onOpenChange={(open) => {
          if (!open) setDnsTarget(null);
        }}
        resource={dnsTarget}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete resource"
        description={`Are you sure you want to delete "${deleteTarget?.name}"? This cannot be undone.`}
        confirmLabel="Delete"
        loading={deleteMutation.isPending}
        onConfirm={() => {
          if (deleteTarget) {
            deleteMutation.mutate(deleteTarget.id, {
              onSuccess: () => setDeleteTarget(null),
            });
          }
        }}
      />
    </div>
  );
}
