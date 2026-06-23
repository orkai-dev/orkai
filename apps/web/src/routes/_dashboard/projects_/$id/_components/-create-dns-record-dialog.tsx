import { Loader2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
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
import { useDnsZones, useResources, useUpsertDnsRecord } from "@/features/resources";

export function CreateDnsRecordDialog({
  open,
  onOpenChange,
  host,
  serverIP,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  host: string;
  serverIP: string;
}) {
  const { data: accounts } = useResources("cloud_account");
  const dnsAccounts = useMemo(
    () => (accounts ?? []).filter((a) => a.provider === "aws" || a.provider === "cloudflare"),
    [accounts],
  );
  const [accountId, setAccountId] = useState("");
  const { data: zones } = useDnsZones(accountId);
  const [zoneId, setZoneId] = useState("");
  const upsert = useUpsertDnsRecord(accountId);

  const [name, setName] = useState(host);
  const [type, setType] = useState("A");
  const [ttl, setTtl] = useState("300");
  const [value, setValue] = useState(serverIP);

  useEffect(() => {
    if (open) {
      setName(host);
      setType("A");
      setTtl("300");
      setValue(serverIP);
      setAccountId(dnsAccounts[0]?.id ?? "");
      setZoneId("");
    }
  }, [open, host, serverIP, dnsAccounts]);

  useEffect(() => {
    if (zones?.length === 1 && !zoneId) {
      setZoneId(zones[0].id);
    }
  }, [zones, zoneId]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!accountId || !zoneId || !name.trim() || !value.trim()) return;
    upsert.mutate(
      {
        zone_id: zoneId,
        name: name.trim(),
        type,
        ttl: Number.parseInt(ttl, 10) || 300,
        values: [value.trim()],
      },
      { onSuccess: () => onOpenChange(false) },
    );
  }

  const noAccounts = dnsAccounts.length === 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create DNS Record</DialogTitle>
          <DialogDescription>
            Create a DNS record pointing this domain at your server.
          </DialogDescription>
        </DialogHeader>

        {noAccounts ? (
          <p className="text-sm text-muted-foreground">
            Add a cloud account (AWS or Cloudflare) under Admin → Resources first.
          </p>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <Label className="text-xs">Cloud Account</Label>
              <Select
                value={accountId}
                onValueChange={(v) => {
                  setAccountId(v);
                  // Reset the zone: it belongs to the previous account.
                  setZoneId("");
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select cloud account" />
                </SelectTrigger>
                <SelectContent>
                  {dnsAccounts.map((a) => (
                    <SelectItem key={a.id} value={a.id}>
                      {a.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">Hosted Zone</Label>
              <Select value={zoneId} onValueChange={setZoneId} disabled={!accountId}>
                <SelectTrigger>
                  <SelectValue placeholder="Select hosted zone" />
                </SelectTrigger>
                <SelectContent>
                  {(zones ?? []).map((z) => (
                    <SelectItem key={z.id} value={z.id}>
                      {z.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5 sm:col-span-2">
                <Label className="text-xs">Name</Label>
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="font-mono text-sm"
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Type</Label>
                <Select value={type} onValueChange={setType}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {["A", "AAAA", "CNAME", "TXT"].map((t) => (
                      <SelectItem key={t} value={t}>
                        {t}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">TTL</Label>
                <Input
                  value={ttl}
                  onChange={(e) => setTtl(e.target.value)}
                  type="number"
                  min={1}
                  className="font-mono text-sm"
                />
              </div>
              <div className="space-y-1.5 sm:col-span-2">
                <Label className="text-xs">Value</Label>
                <Input
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  className="font-mono text-sm"
                  required
                />
              </div>
            </div>

            <DialogFooter>
              <Button type="submit" disabled={upsert.isPending || !zoneId}>
                {upsert.isPending && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                {upsert.isPending ? "Creating..." : "Create Record"}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
