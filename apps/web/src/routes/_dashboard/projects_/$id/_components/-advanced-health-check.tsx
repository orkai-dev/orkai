import { Shield } from "lucide-react";
import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { useUpdateApp } from "@/features/apps";
import type { App } from "@/features/apps/types";
import {
  COMMON_PORTS,
  FAILURE_THRESHOLD_OPTIONS,
  HEALTH_CHECK_PATHS,
  INITIAL_DELAY_OPTIONS,
  PERIOD_OPTIONS,
  TIMEOUT_OPTIONS,
} from "@/lib/constants";
import { SectionCard } from "./-advanced-shared";

export function HealthCheckCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const hc = app.health_check;
  const defaultPort = app.ports?.[0]?.container_port || 80;
  const [type, setType] = useState(hc?.type || "");
  const [path, setPath] = useState(hc?.path || "/healthz");
  const [port, setPort] = useState(String(hc?.port || defaultPort));
  const [command, setCommand] = useState(hc?.command || "");
  const [initialDelay, setInitialDelay] = useState(String(hc?.initial_delay_seconds || 0));
  const [period, setPeriod] = useState(String(hc?.period_seconds || 10));
  const [timeout, setTimeoutVal] = useState(String(hc?.timeout_seconds || 3));
  const [failureThreshold, setFailureThreshold] = useState(String(hc?.failure_threshold || 3));

  const dirty =
    type !== (hc?.type || "") ||
    path !== (hc?.path || "/healthz") ||
    port !== String(hc?.port || defaultPort) ||
    command !== (hc?.command || "") ||
    initialDelay !== String(hc?.initial_delay_seconds || 0) ||
    period !== String(hc?.period_seconds || 10) ||
    timeout !== String(hc?.timeout_seconds || 3) ||
    failureThreshold !== String(hc?.failure_threshold || 3);

  function handleSave() {
    updateApp.mutate({
      health_check: {
        type,
        path,
        port: Number(port),
        command,
        initial_delay_seconds: Number(initialDelay),
        period_seconds: Number(period),
        timeout_seconds: Number(timeout),
        failure_threshold: Number(failureThreshold),
      },
    });
  }

  return (
    <SectionCard
      icon={Shield}
      title="Health Checks"
      description="Configure liveness and readiness probes to monitor your application."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label className="text-xs">Probe Type</Label>
          <Select value={type || "none"} onValueChange={(v) => setType(v === "none" ? "" : v)}>
            <SelectTrigger className="w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">None (disabled)</SelectItem>
              <SelectItem value="http">HTTP GET</SelectItem>
              <SelectItem value="tcp">TCP Socket</SelectItem>
              <SelectItem value="exec">Exec Command</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {type === "http" && (
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label className="text-xs">Path</Label>
              <Input
                value={path}
                onChange={(e) => setPath(e.target.value)}
                placeholder="/healthz"
                list="hc-paths"
              />
              <datalist id="hc-paths">
                {HEALTH_CHECK_PATHS.map((p) => (
                  <option key={p.value} value={p.value} />
                ))}
              </datalist>
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">Port</Label>
              <Input
                type="number"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                placeholder={String(defaultPort)}
                list="hc-ports"
              />
              <datalist id="hc-ports">
                {COMMON_PORTS.map((p) => (
                  <option key={p.value} value={p.value} label={p.label} />
                ))}
              </datalist>
            </div>
          </div>
        )}

        {type === "tcp" && (
          <div className="space-y-1.5">
            <Label className="text-xs">Port</Label>
            <Input
              type="number"
              value={port}
              onChange={(e) => setPort(e.target.value)}
              placeholder={String(defaultPort)}
              list="hc-ports-tcp"
              className="w-48"
            />
            <datalist id="hc-ports-tcp">
              {COMMON_PORTS.map((p) => (
                <option key={p.value} value={p.value} label={p.label} />
              ))}
            </datalist>
          </div>
        )}

        {type === "exec" && (
          <div className="space-y-1.5">
            <Label className="text-xs">Command</Label>
            <Input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              placeholder="cat /tmp/healthy"
              className="font-mono text-sm"
            />
          </div>
        )}

        {type && type !== "none" && (
          <>
            <Separator />
            <div className="grid gap-4 sm:grid-cols-4">
              <div className="space-y-1.5">
                <Label className="text-xs">Initial Delay</Label>
                <Select value={initialDelay} onValueChange={setInitialDelay}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {INITIAL_DELAY_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Period</Label>
                <Select value={period} onValueChange={setPeriod}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PERIOD_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Timeout</Label>
                <Select value={timeout} onValueChange={setTimeoutVal}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {TIMEOUT_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Failures</Label>
                <Select value={failureThreshold} onValueChange={setFailureThreshold}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FAILURE_THRESHOLD_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          </>
        )}
      </div>
    </SectionCard>
  );
}
