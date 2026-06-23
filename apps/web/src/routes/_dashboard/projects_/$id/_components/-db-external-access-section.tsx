import { AlertTriangle, Copy, Globe, Loader2 } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateExternalAccess, useUsedPorts } from "@/features/databases";
import type { ManagedDB } from "@/features/databases/types";
import { copyToClipboard, ENGINE_DEFAULT_PORT, ENGINE_PROTOCOL } from "./-db-helpers";

export function DbExternalAccessSection({ db }: { db: ManagedDB }) {
  const updateExternal = useUpdateExternalAccess(db.id);
  const { data: rawUsedPorts } = useUsedPorts();
  const usedPorts = rawUsedPorts ?? [];
  const externalHost = window.location.hostname;
  const protocol = ENGINE_PROTOCOL[db.engine] || db.engine;
  const defaultPort = ENGINE_DEFAULT_PORT[db.engine] || 30000;
  const [port, setPort] = useState(
    db.external_port > 0 ? String(db.external_port) : String(defaultPort),
  );

  const portNum = Number(port);
  const portInRange = !port || (portNum >= 30000 && portNum <= 32767);
  const portConflict = usedPorts.find((p) => p.port === portNum && p.database_id !== db.id);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Globe className="h-4 w-4" /> External Access
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">
          Allow external connections to this database via NodePort.
        </p>

        {!db.external_enabled && (
          <div className="space-y-3">
            <div className="space-y-2">
              <Label className="text-sm">Port</Label>
              <div className="flex items-center gap-3">
                <Input
                  type="number"
                  min={30000}
                  max={32767}
                  value={port}
                  onChange={(e) => setPort(e.target.value)}
                  placeholder={`e.g. ${defaultPort}`}
                  className="max-w-[200px] font-mono"
                />
                <Button
                  onClick={() =>
                    updateExternal.mutate({
                      enabled: true,
                      port: port ? Number(port) : undefined,
                    })
                  }
                  disabled={updateExternal.isPending || !port || !portInRange || !!portConflict}
                >
                  {updateExternal.isPending ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Globe className="h-3.5 w-3.5" />
                  )}
                  Enable
                </Button>
              </div>
              {port && !portInRange && (
                <p className="text-xs text-destructive">
                  Port must be between 30000–32767 (K8s NodePort range)
                </p>
              )}
              {portConflict && (
                <p className="text-xs text-destructive">
                  Port {portNum} is already used by &quot;{portConflict.database_name}&quot; (
                  {portConflict.engine})
                </p>
              )}
              <p className="text-xs text-muted-foreground">
                NodePort range: 30000–32767
                {usedPorts.length > 0 && (
                  <>
                    {" "}
                    &middot; In use:{" "}
                    {usedPorts.map((p) => `${p.port} (${p.database_name})`).join(", ")}
                  </>
                )}
              </p>
            </div>
          </div>
        )}

        {db.external_enabled && db.external_port > 0 && (
          <div className="space-y-3">
            <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
              <span className="text-sm text-muted-foreground">External URL</span>
              <div className="flex items-center gap-2">
                <span className="truncate font-mono text-sm">
                  {protocol}://{externalHost}:{db.external_port}
                </span>
                <Button
                  size="icon"
                  variant="ghost"
                  className="h-7 w-7 shrink-0"
                  aria-label="Copy to clipboard"
                  onClick={() =>
                    copyToClipboard(`${protocol}://${externalHost}:${db.external_port}`)
                  }
                >
                  <Copy className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>

            <div className="flex items-center gap-2 rounded-md border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-sm text-yellow-400">
              <AlertTriangle className="h-4 w-4 shrink-0" />
              Exposing database to public network. Ensure strong passwords are set.
            </div>

            <Button
              variant="outline"
              size="sm"
              onClick={() => updateExternal.mutate({ enabled: false })}
              disabled={updateExternal.isPending}
            >
              {updateExternal.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              Disable External Access
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
