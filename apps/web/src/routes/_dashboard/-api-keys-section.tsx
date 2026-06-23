import { Check, Copy, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { type APIKeyRole, useApiKeys, useCreateApiKey, useRevokeApiKey } from "@/features/api-keys";
import { dateInputToEndOfDayISO, localDateInputValue, timeAgo } from "@/lib/format";
import { cn } from "@/lib/utils";

function PanelHeader({ title, description }: { title: string; description: string }) {
  return (
    <div className="border-b px-5 py-4">
      <h2 className="text-sm font-semibold">{title}</h2>
      <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
    </div>
  );
}

function roleLabel(role: APIKeyRole) {
  return role === "admin" ? "Administrative" : "Common / deployments";
}

function formatOptionalDate(value?: string) {
  if (!value) return "Never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleDateString();
}

export function APIKeysSection({ isAdmin }: { isAdmin: boolean }) {
  const { data: keys, isLoading } = useApiKeys();
  const createKey = useCreateApiKey();
  const revokeKey = useRevokeApiKey();

  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [role, setRole] = useState<APIKeyRole>("member");
  const [expiresAt, setExpiresAt] = useState("");

  const [revealedKey, setRevealedKey] = useState<string | null>(null);
  const [keyCopied, setKeyCopied] = useState(false);

  const [revokeTarget, setRevokeTarget] = useState<{ id: string; name: string } | null>(null);

  const resetCreateForm = () => {
    setName("");
    setRole("member");
    setExpiresAt("");
  };

  const handleCreate = () => {
    const trimmed = name.trim();
    if (!trimmed) {
      toast.error("Name is required");
      return;
    }

    createKey.mutate(
      {
        name: trimmed,
        role: isAdmin ? role : "member",
        expires_at: expiresAt ? dateInputToEndOfDayISO(expiresAt) : undefined,
      },
      {
        onSuccess: (result) => {
          setCreateOpen(false);
          resetCreateForm();
          setRevealedKey(result.key);
          setKeyCopied(false);
          toast.success("API key created");
        },
      },
    );
  };

  const copyRevealedKey = () => {
    if (!revealedKey) return;
    navigator.clipboard.writeText(revealedKey);
    setKeyCopied(true);
    setTimeout(() => setKeyCopied(false), 2000);
  };

  return (
    <>
      <PanelHeader
        title="API keys"
        description="Generate keys to authenticate against the REST API. Keys inherit your permissions and are shown only once at creation."
      />

      <div className="flex items-center justify-between border-b px-5 py-3">
        <p className="text-xs text-muted-foreground">
          Use{" "}
          <code className="rounded bg-muted px-1 py-0.5 font-mono">
            Authorization: Bearer ork_...
          </code>{" "}
          on API requests.
        </p>
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          <Plus className="mr-1.5 h-3.5 w-3.5" />
          Create key
        </Button>
      </div>

      <div className="divide-y">
        {isLoading && (
          <p className="px-5 py-8 text-center text-sm text-muted-foreground">Loading keys…</p>
        )}
        {!isLoading && (!keys || keys.length === 0) && (
          <p className="px-5 py-8 text-center text-sm text-muted-foreground">
            No API keys yet. Create one to access the API programmatically.
          </p>
        )}
        {keys?.map((key) => (
          <div
            key={key.id}
            className="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center sm:justify-between"
          >
            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-medium">{key.name}</span>
                <Badge variant="secondary">{roleLabel(key.role)}</Badge>
              </div>
              <p className="font-mono text-xs text-muted-foreground">{key.key_prefix}…</p>
              <p className="text-xs text-muted-foreground">
                Created {timeAgo(key.created_at)}
                {key.last_used_at ? ` · Last used ${timeAgo(key.last_used_at)}` : " · Never used"}
                {key.expires_at ? ` · Expires ${formatOptionalDate(key.expires_at)}` : ""}
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="shrink-0 text-destructive hover:text-destructive"
              onClick={() => setRevokeTarget({ id: key.id, name: key.name })}
            >
              <Trash2 className="mr-1.5 h-3.5 w-3.5" />
              Revoke
            </Button>
          </div>
        ))}
      </div>

      <Dialog
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open);
          if (!open) resetCreateForm();
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create API key</DialogTitle>
            <DialogDescription>
              The full key is shown once after creation. Store it securely — it cannot be retrieved
              later.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="api-key-name">Name</Label>
              <Input
                id="api-key-name"
                placeholder="CI deploys, local scripts…"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            {isAdmin && (
              <div className="space-y-2">
                <Label>Scope</Label>
                <Select value={role} onValueChange={(v) => setRole(v as APIKeyRole)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="member">Common / deployments</SelectItem>
                    <SelectItem value="admin">Administrative</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  Administrative keys can manage teams, settings, and infrastructure. Common keys
                  are limited to deployment and project operations.
                </p>
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="api-key-expires">Expires (optional)</Label>
              <Input
                id="api-key-expires"
                type="date"
                min={localDateInputValue()}
                value={expiresAt}
                onChange={(e) => setExpiresAt(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreate} disabled={createKey.isPending}>
              {createKey.isPending ? "Creating…" : "Create key"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={revealedKey !== null}
        onOpenChange={(open) => {
          if (!open) setRevealedKey(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Copy your API key</DialogTitle>
            <DialogDescription>
              This is the only time the full key will be shown. Copy it now and store it securely.
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2 rounded-md border bg-muted/40 p-3">
            <code className="min-w-0 flex-1 break-all font-mono text-xs">{revealedKey}</code>
            <Button variant="outline" size="icon" className="shrink-0" onClick={copyRevealedKey}>
              {keyCopied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
            </Button>
          </div>
          <DialogFooter>
            <Button
              className={cn(keyCopied && "border-green-600/30")}
              onClick={() => setRevealedKey(null)}
            >
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={revokeTarget !== null}
        onOpenChange={(open) => {
          if (!open) setRevokeTarget(null);
        }}
        title="Revoke API key"
        description={
          revokeTarget ? (
            <>
              Revoke <strong>{revokeTarget.name}</strong>? Any integrations using this key will stop
              working immediately.
            </>
          ) : (
            ""
          )
        }
        confirmLabel="Revoke"
        loading={revokeKey.isPending}
        onConfirm={() => {
          if (!revokeTarget) return;
          revokeKey.mutate(revokeTarget.id, {
            onSuccess: () => setRevokeTarget(null),
          });
        }}
      />
    </>
  );
}
