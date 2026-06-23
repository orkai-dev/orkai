import { Loader2, Plus, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  useDeleteDnsRecord,
  useDnsRecords,
  useDnsZones,
  useUpsertDnsRecord,
} from "@/features/resources";
import type { DnsRecord, SharedResource } from "@/features/resources/types";

const RECORD_TYPES = ["A", "AAAA", "CNAME", "TXT"] as const;

function RecordRow({
  record,
  zoneId,
  resourceId,
}: {
  record: DnsRecord;
  zoneId: string;
  resourceId: string;
}) {
  const del = useDeleteDnsRecord(resourceId);

  return (
    <div className="flex items-start justify-between gap-3 rounded-md border p-3 text-sm">
      <div className="min-w-0 space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="truncate font-mono font-medium">{record.name}</span>
          <Badge variant="outline" className="text-xs">
            {record.type}
          </Badge>
          <span className="text-xs text-muted-foreground">TTL {record.ttl}</span>
        </div>
        <p className="break-all font-mono text-xs text-muted-foreground">
          {record.values.join(", ")}
        </p>
      </div>
      <Button
        size="icon"
        variant="ghost"
        className="h-8 w-8 shrink-0 text-destructive"
        disabled={del.isPending}
        onClick={() =>
          del.mutate({
            zone_id: zoneId,
            name: record.name,
            type: record.type,
            ttl: record.ttl,
            values: record.values,
          })
        }
      >
        <Trash2 className="h-3.5 w-3.5" />
      </Button>
    </div>
  );
}

function dnsProviderLabel(provider: string | undefined): string {
  if (provider === "cloudflare") {
    return "Cloudflare DNS";
  }
  return "Route53";
}

export function DnsManagerSheet({
  open,
  onOpenChange,
  resource,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  resource: SharedResource | null;
}) {
  const resourceId = resource?.id ?? "";
  const { data: zones, isLoading: zonesLoading } = useDnsZones(resourceId);
  const [zoneId, setZoneId] = useState("");
  const { data: records, isLoading: recordsLoading } = useDnsRecords(resourceId, zoneId);
  const upsert = useUpsertDnsRecord(resourceId);

  const [name, setName] = useState("");
  const [type, setType] = useState<(typeof RECORD_TYPES)[number]>("A");
  const [ttl, setTtl] = useState("300");
  const [value, setValue] = useState("");

  useEffect(() => {
    if (open) {
      setZoneId("");
      setName("");
      setType("A");
      setTtl("300");
      setValue("");
    }
  }, [open]);

  useEffect(() => {
    if (zones?.length === 1 && !zoneId) {
      setZoneId(zones[0].id);
    }
  }, [zones, zoneId]);

  function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!zoneId || !name.trim() || !value.trim()) return;
    upsert.mutate(
      {
        zone_id: zoneId,
        name: name.trim(),
        type,
        ttl: Number.parseInt(ttl, 10) || 300,
        values: [value.trim()],
      },
      {
        onSuccess: () => {
          setName("");
          setValue("");
        },
      },
    );
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="flex w-full flex-col sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Manage DNS</SheetTitle>
          <SheetDescription>
            {dnsProviderLabel(resource?.provider)} records for{" "}
            <span className="font-medium">{resource?.name}</span>
          </SheetDescription>
        </SheetHeader>

        <div className="mt-4 flex flex-1 flex-col gap-4 overflow-hidden">
          <div className="space-y-1.5">
            <Label className="text-xs">Hosted Zone</Label>
            {zonesLoading ? (
              <p className="text-xs text-muted-foreground">Loading zones...</p>
            ) : (
              <Select value={zoneId} onValueChange={setZoneId}>
                <SelectTrigger>
                  <SelectValue placeholder="Select a hosted zone" />
                </SelectTrigger>
                <SelectContent>
                  {(zones ?? []).map((z) => (
                    <SelectItem key={z.id} value={z.id}>
                      {z.name}
                      {z.private ? " (private)" : ""}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>

          {zoneId && (
            <>
              <form onSubmit={handleCreate} className="space-y-3 rounded-md border p-3">
                <p className="text-xs font-medium">Create Record</p>
                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="space-y-1 sm:col-span-2">
                    <Label className="text-xs">Name</Label>
                    <Input
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="app.example.com"
                      className="font-mono text-sm"
                      required
                    />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Type</Label>
                    <Select value={type} onValueChange={(v) => setType(v as typeof type)}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {RECORD_TYPES.map((t) => (
                          <SelectItem key={t} value={t}>
                            {t}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">TTL</Label>
                    <Input
                      value={ttl}
                      onChange={(e) => setTtl(e.target.value)}
                      type="number"
                      min={1}
                      className="font-mono text-sm"
                    />
                  </div>
                  <div className="space-y-1 sm:col-span-2">
                    <Label className="text-xs">Value</Label>
                    <Input
                      value={value}
                      onChange={(e) => setValue(e.target.value)}
                      placeholder="203.0.113.5"
                      className="font-mono text-sm"
                      required
                    />
                  </div>
                </div>
                <Button type="submit" size="sm" disabled={upsert.isPending}>
                  {upsert.isPending ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Plus className="h-3.5 w-3.5" />
                  )}
                  {upsert.isPending ? "Saving..." : "Create Record"}
                </Button>
              </form>

              <div className="flex min-h-0 flex-1 flex-col gap-2">
                <p className="text-xs font-medium text-muted-foreground">
                  Records in zone ({records?.length ?? 0})
                </p>
                <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
                  {recordsLoading ? (
                    <p className="py-4 text-center text-xs text-muted-foreground">
                      Loading records...
                    </p>
                  ) : (records ?? []).length === 0 ? (
                    <p className="py-4 text-center text-xs text-muted-foreground">
                      No records yet.
                    </p>
                  ) : (
                    (records ?? []).map((rec) => (
                      <RecordRow
                        key={`${rec.name}-${rec.type}-${rec.values.join(",")}`}
                        record={rec}
                        zoneId={zoneId}
                        resourceId={resourceId}
                      />
                    ))
                  )}
                </div>
              </div>
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
