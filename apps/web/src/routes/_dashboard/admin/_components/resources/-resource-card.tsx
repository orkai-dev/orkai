import { Check, Copy, Edit, ExternalLink, Globe, Play, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import type { SharedResource } from "@/features/resources/types";
import { statusVariant } from "@/lib/constants";
import type { ResourceType } from "./-resources.config";

export function ResourceCard({
  resource: r,
  type,
  icon: Icon,
  onTest,
  onEdit,
  onDelete,
  onManageDNS,
  testing,
  installUrl,
}: {
  resource: SharedResource;
  type: ResourceType;
  icon: React.ComponentType<{ className?: string }>;
  onTest: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onManageDNS?: () => void;
  testing: boolean;
  installUrl?: string;
}) {
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const config = r.config as Record<string, string>;
  const publicKey = type === "ssh_key" ? config?.public_key : null;
  const privateKey = type === "ssh_key" ? config?.private_key : null;

  function copyToClipboard(text: string, label: string) {
    navigator.clipboard.writeText(text);
    setCopiedField(label);
    toast.success(`${label} copied!`);
    setTimeout(() => setCopiedField(null), 2000);
  }

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-4">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <Icon className="h-4 w-4 text-primary" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">{r.name}</span>
              <Badge variant="outline" className="text-xs">
                {r.provider}
              </Badge>
              <Badge variant={statusVariant(r.status)}>{r.status}</Badge>
            </div>
            <p className="text-xs text-muted-foreground">
              Created {new Date(r.created_at).toLocaleDateString()}
            </p>
          </div>
          <div className="flex items-center gap-1">
            {installUrl && r.provider === "github" && (
              <a
                href={installUrl}
                target="_blank"
                rel="noopener noreferrer"
                title="Manage repository access on GitHub"
              >
                <Button variant="ghost" size="icon">
                  <ExternalLink className="h-4 w-4" />
                </Button>
              </a>
            )}
            {type === "cloud_account" && onManageDNS && (
              <Button variant="ghost" size="icon" title="Manage DNS" onClick={onManageDNS}>
                <Globe className="h-4 w-4" />
              </Button>
            )}
            {type !== "ssh_key" && (
              <Button variant="ghost" size="icon" title="Test" disabled={testing} onClick={onTest}>
                <Play className="h-4 w-4" />
              </Button>
            )}
            <Button variant="ghost" size="icon" title="Edit" onClick={onEdit}>
              <Edit className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" title="Delete" onClick={onDelete}>
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          </div>
        </div>

        {/* SSH Key: public + private key with copy buttons */}
        {(publicKey || privateKey) && (
          <div className="mt-3 space-y-2">
            {publicKey && (
              <div className="space-y-1">
                <p className="text-xs font-medium text-muted-foreground">Public Key</p>
                <div className="flex items-start gap-2 rounded-lg border bg-muted p-3">
                  <code className="flex-1 break-all font-mono text-xs text-foreground">
                    {publicKey}
                  </code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 shrink-0"
                    onClick={() => copyToClipboard(publicKey, "Public key")}
                    title="Copy public key"
                  >
                    {copiedField === "Public key" ? (
                      <Check className="h-3.5 w-3.5 text-green-500" />
                    ) : (
                      <Copy className="h-3.5 w-3.5" />
                    )}
                  </Button>
                </div>
              </div>
            )}
            {privateKey && (
              <div className="space-y-1">
                <p className="text-xs font-medium text-muted-foreground">Private Key</p>
                <div className="flex items-start gap-2 rounded-lg border bg-muted p-3">
                  <code className="flex-1 break-all font-mono text-xs text-foreground">
                    {privateKey.slice(0, 60)}...
                  </code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 shrink-0"
                    onClick={() => copyToClipboard(privateKey, "Private key")}
                    title="Copy private key"
                  >
                    {copiedField === "Private key" ? (
                      <Check className="h-3.5 w-3.5 text-green-500" />
                    ) : (
                      <Copy className="h-3.5 w-3.5" />
                    )}
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
