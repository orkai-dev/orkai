import { Gauge, Minus, Plus, Zap } from "lucide-react";
import { useState } from "react";
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
import { useScaleApp, useUpdateApp } from "@/features/apps";
import type { App } from "@/features/apps/types";
import { HPA_MAX_REPLICAS, HPA_MIN_REPLICAS, HPA_TARGET_OPTIONS } from "@/lib/constants";
import { InfoBanner, SectionCard } from "./-advanced-shared";

export function AutoscalingCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const as = app.autoscaling;
  const [enabled, setEnabled] = useState(as?.enabled || false);
  const [minReplicas, setMinReplicas] = useState(String(as?.min_replicas || 1));
  const [maxReplicas, setMaxReplicas] = useState(String(as?.max_replicas || 10));
  const [cpuTarget, setCpuTarget] = useState(String(as?.cpu_target || 80));
  const [memTarget, setMemTarget] = useState(String(as?.mem_target || 0));

  const dirty =
    enabled !== (as?.enabled || false) ||
    minReplicas !== String(as?.min_replicas || 1) ||
    maxReplicas !== String(as?.max_replicas || 10) ||
    cpuTarget !== String(as?.cpu_target || 80) ||
    memTarget !== String(as?.mem_target || 0);

  function handleSave() {
    updateApp.mutate({
      autoscaling: {
        enabled,
        min_replicas: Number(minReplicas),
        max_replicas: Number(maxReplicas),
        cpu_target: Number(cpuTarget),
        mem_target: Number(memTarget),
      },
    });
  }

  return (
    <SectionCard
      icon={Zap}
      title="Autoscaling (HPA)"
      description="Automatically scale pods based on CPU or memory usage."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-4">
        <label className="flex items-center gap-2 text-sm font-medium">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            className="rounded"
          />
          Enable Horizontal Pod Autoscaler
        </label>

        {enabled && (
          <>
            <InfoBanner>Scaling is managed by HPA. Manual scaling will be disabled.</InfoBanner>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <div className="space-y-1.5">
                <Label className="text-xs">Min Replicas</Label>
                <Select value={minReplicas} onValueChange={setMinReplicas}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HPA_MIN_REPLICAS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Max Replicas</Label>
                <Select value={maxReplicas} onValueChange={setMaxReplicas}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HPA_MAX_REPLICAS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">CPU Target</Label>
                <Select value={cpuTarget} onValueChange={setCpuTarget}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HPA_TARGET_OPTIONS.filter((o) => o.value !== "0").map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Memory Target</Label>
                <Select value={memTarget} onValueChange={setMemTarget}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HPA_TARGET_OPTIONS.map((o) => (
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

export function ScalingSection({
  appId,
  currentReplicas,
  hpaEnabled,
}: {
  appId: string;
  currentReplicas: number;
  hpaEnabled: boolean;
}) {
  const scale = useScaleApp(appId);
  const [input, setInput] = useState(currentReplicas);

  return (
    <SectionCard
      icon={Gauge}
      title="Manual Scaling"
      description="Set the number of running pod replicas."
    >
      {hpaEnabled ? (
        <InfoBanner>Autoscaling (HPA) is enabled. Manual scaling is disabled.</InfoBanner>
      ) : (
        <div className="flex items-center gap-3">
          <Button
            size="icon"
            variant="outline"
            className="h-8 w-8"
            onClick={() => setInput(Math.max(0, input - 1))}
          >
            <Minus className="h-3 w-3" />
          </Button>
          <Input
            type="number"
            min={0}
            max={100}
            value={input}
            onChange={(e) => setInput(Number.parseInt(e.target.value, 10) || 0)}
            className="w-20 text-center"
          />
          <Button
            size="icon"
            variant="outline"
            className="h-8 w-8"
            onClick={() => setInput(input + 1)}
          >
            <Plus className="h-3 w-3" />
          </Button>
          <Button
            onClick={() => scale.mutate(input)}
            disabled={scale.isPending || input === currentReplicas}
            size="sm"
          >
            {scale.isPending ? "Scaling..." : "Apply"}
          </Button>
        </div>
      )}
    </SectionCard>
  );
}
