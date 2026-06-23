import { Activity, Cpu, Network, Plus, Server, Tag, Timer, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
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
import { useUpdateApp } from "@/features/apps";
import type { App, PortMapping } from "@/features/apps/types";
import { useNodePools } from "@/features/cluster";
import { CPU_OPTIONS, GRACE_PERIOD_OPTIONS, MEMORY_OPTIONS, SURGE_OPTIONS } from "@/lib/constants";
import { SectionCard } from "./-advanced-shared";

export function MetadataCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const [isCritical, setIsCritical] = useState(app.is_critical || false);

  const dirty = isCritical !== (app.is_critical || false);

  function handleSave() {
    updateApp.mutate({ is_critical: isCritical });
  }

  return (
    <SectionCard
      icon={Tag}
      title="Metadata"
      description="Mark whether this application is business-critical. Team and environment are set on the project."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <label className="flex items-center gap-2 text-sm font-medium">
        <input
          type="checkbox"
          checked={isCritical}
          onChange={(e) => setIsCritical(e.target.checked)}
          className="rounded"
        />
        Mark as critical
      </label>
    </SectionCard>
  );
}

export function ResourcesCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const [cpuReq, setCpuReq] = useState(app.cpu_request || "250m");
  const [cpuLim, setCpuLim] = useState(app.cpu_limit || "500m");
  const [memReq, setMemReq] = useState(app.mem_request || "256Mi");
  const [memLim, setMemLim] = useState(app.mem_limit || "512Mi");

  const dirty =
    cpuReq !== (app.cpu_request || "250m") ||
    cpuLim !== (app.cpu_limit || "500m") ||
    memReq !== (app.mem_request || "256Mi") ||
    memLim !== (app.mem_limit || "512Mi");

  function handleSave() {
    updateApp.mutate({
      cpu_request: cpuReq,
      cpu_limit: cpuLim,
      mem_request: memReq,
      mem_limit: memLim,
    });
  }

  function ResourceSelect({
    label,
    value,
    onChange,
    options,
  }: {
    label: string;
    value: string;
    onChange: (v: string) => void;
    options: { value: string; label: string }[];
  }) {
    return (
      <div className="space-y-1.5">
        <Label className="text-xs">{label}</Label>
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {options.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    );
  }

  return (
    <SectionCard
      icon={Cpu}
      title="Resources"
      description="Set CPU and memory requests (guaranteed) and limits (maximum)."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-3 rounded-lg border p-3">
          <p className="text-xs font-medium text-muted-foreground">Request (guaranteed)</p>
          <ResourceSelect label="CPU" value={cpuReq} onChange={setCpuReq} options={CPU_OPTIONS} />
          <ResourceSelect
            label="Memory"
            value={memReq}
            onChange={setMemReq}
            options={MEMORY_OPTIONS}
          />
        </div>
        <div className="space-y-3 rounded-lg border p-3">
          <p className="text-xs font-medium text-muted-foreground">Limit (maximum)</p>
          <ResourceSelect label="CPU" value={cpuLim} onChange={setCpuLim} options={CPU_OPTIONS} />
          <ResourceSelect
            label="Memory"
            value={memLim}
            onChange={setMemLim}
            options={MEMORY_OPTIONS}
          />
        </div>
      </div>
    </SectionCard>
  );
}

export function PortsCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const [ports, setPorts] = useState<PortMapping[]>(
    app.ports?.length
      ? app.ports
      : [{ container_port: 3000, service_port: 3000, protocol: "tcp" as const }],
  );

  const serialize = (p: PortMapping[]) =>
    p.map((x) => `${x.container_port}:${x.service_port}:${x.protocol}`).join(",");
  const dirty = serialize(ports) !== serialize(app.ports || []);

  function updatePort(index: number, field: keyof PortMapping, value: string | number) {
    setPorts(
      ports.map((p, i) =>
        i === index ? { ...p, [field]: typeof p[field] === "number" ? Number(value) : value } : p,
      ),
    );
  }

  function addPort() {
    setPorts([...ports, { container_port: 8080, service_port: 80, protocol: "tcp" }]);
  }

  function handleSave() {
    const valid = ports.filter((p) => p.container_port > 0 && p.service_port > 0);
    if (valid.length === 0) {
      toast.error("At least one port mapping is required");
      return;
    }
    updateApp.mutate({ ports: valid });
  }

  return (
    <SectionCard
      icon={Network}
      title="Port Configuration"
      description="Map container ports to service ports for external access."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-3">
        {ports.map((p, i) => (
          <div key={`${p.container_port}-${p.service_port}-${i}`} className="flex items-end gap-3">
            <div className="space-y-1.5">
              {i === 0 && <Label className="text-xs">Container Port</Label>}
              <Input
                type="number"
                min={1}
                max={65535}
                value={p.container_port || ""}
                onChange={(e) => updatePort(i, "container_port", e.target.value)}
                placeholder="3000"
                className="w-28 font-mono text-xs"
              />
            </div>
            <span className="mb-2 text-muted-foreground">&rarr;</span>
            <div className="space-y-1.5">
              {i === 0 && <Label className="text-xs">Service Port</Label>}
              <Input
                type="number"
                min={1}
                max={65535}
                value={p.service_port || ""}
                onChange={(e) => updatePort(i, "service_port", e.target.value)}
                placeholder="3000"
                className="w-28 font-mono text-xs"
              />
            </div>
            <div className="space-y-1.5">
              {i === 0 && <Label className="text-xs">Protocol</Label>}
              <Select value={p.protocol} onValueChange={(v) => updatePort(i, "protocol", v)}>
                <SelectTrigger className="w-20">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="tcp">TCP</SelectItem>
                  <SelectItem value="udp">UDP</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <Button
              size="icon"
              variant="ghost"
              className="mb-0.5 h-8 w-8 shrink-0 text-destructive"
              onClick={() => setPorts(ports.filter((_, j) => j !== i))}
              disabled={ports.length <= 1}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        ))}

        <Button size="sm" variant="outline" onClick={addPort}>
          <Plus className="h-3.5 w-3.5" /> Add Port
        </Button>
      </div>
    </SectionCard>
  );
}

export function DeployStrategyCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const [strategy, setStrategy] = useState(app.deploy_strategy || "rolling");
  const [maxSurge, setMaxSurge] = useState(app.deploy_strategy_config?.max_surge || "25%");
  const [maxUnavailable, setMaxUnavailable] = useState(
    app.deploy_strategy_config?.max_unavailable || "25%",
  );

  const dirty =
    strategy !== (app.deploy_strategy || "rolling") ||
    maxSurge !== (app.deploy_strategy_config?.max_surge || "25%") ||
    maxUnavailable !== (app.deploy_strategy_config?.max_unavailable || "25%");

  function handleSave() {
    updateApp.mutate({
      deploy_strategy: strategy,
      deploy_strategy_config:
        strategy === "rolling"
          ? { max_surge: maxSurge, max_unavailable: maxUnavailable }
          : { max_surge: "", max_unavailable: "" },
    });
  }

  return (
    <SectionCard
      icon={Activity}
      title="Deployment Strategy"
      description="Control how new versions are rolled out."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label className="text-xs">Strategy</Label>
          <Select value={strategy} onValueChange={setStrategy}>
            <SelectTrigger className="w-56">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="rolling">Rolling Update (zero downtime)</SelectItem>
              <SelectItem value="recreate">Recreate (brief downtime)</SelectItem>
            </SelectContent>
          </Select>
        </div>
        {strategy === "rolling" && (
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label className="text-xs">Max Surge</Label>
              <Select value={maxSurge} onValueChange={setMaxSurge}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SURGE_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">Max Unavailable</Label>
              <Select value={maxUnavailable} onValueChange={setMaxUnavailable}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SURGE_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        )}
      </div>
    </SectionCard>
  );
}

export function TerminationCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const [seconds, setSeconds] = useState(String(app.termination_grace_period || 30));

  const dirty = seconds !== String(app.termination_grace_period || 30);

  function handleSave() {
    updateApp.mutate({ termination_grace_period: Number(seconds) });
  }

  return (
    <SectionCard
      icon={Timer}
      title="Graceful Termination"
      description="Time to wait for the process to exit before force-killing."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <Select value={seconds} onValueChange={setSeconds}>
        <SelectTrigger className="w-56">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {GRACE_PERIOD_OPTIONS.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </SectionCard>
  );
}

export function NodePoolCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const { data: pools } = useNodePools();
  const [nodePool, setNodePool] = useState(app.node_pool || "");
  const dirty = nodePool !== (app.node_pool || "");

  return (
    <SectionCard
      icon={Server}
      title="Node Pool"
      description="Deploy to a specific group of nodes."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={() => updateApp.mutate({ node_pool: nodePool })}
    >
      <div className="space-y-2">
        <Select value={nodePool || "any"} onValueChange={(v) => setNodePool(v === "any" ? "" : v)}>
          <SelectTrigger className="w-56">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="any">Any node (default)</SelectItem>
            {pools?.map((p) => (
              <SelectItem key={p} value={p}>
                {p}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">
          Assign node pools via Cluster → Nodes. Only nodes with the matching{" "}
          <code className="rounded bg-muted px-1">orkai/pool</code> label will be selected.
        </p>
      </div>
    </SectionCard>
  );
}
