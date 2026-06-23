import { DangerZone } from "@/components/danger-zone";
import type { App } from "@/features/apps/types";
import { AutoscalingCard, ScalingSection } from "./-advanced-autoscaling";
import { BuildConfigCard, SourceProviderCard } from "./-advanced-build-config";
import { HealthCheckCard } from "./-advanced-health-check";
import {
  DeployStrategyCard,
  MetadataCard,
  NodePoolCard,
  PortsCard,
  ResourcesCard,
  TerminationCard,
} from "./-advanced-volumes-strategy";

export function SettingsTab({
  app,
  appId,
  onDelete,
}: {
  app: App;
  appId: string;
  onDelete: () => void;
}) {
  return (
    <div className="space-y-6">
      <MetadataCard app={app} appId={appId} />

      <SourceProviderCard app={app} appId={appId} />
      {app.source_type === "git" && <BuildConfigCard app={app} appId={appId} />}

      <ResourcesCard app={app} appId={appId} />
      <PortsCard app={app} appId={appId} />
      <HealthCheckCard app={app} appId={appId} />
      <DeployStrategyCard app={app} appId={appId} />
      <AutoscalingCard app={app} appId={appId} />
      <ScalingSection
        appId={appId}
        currentReplicas={app.replicas}
        hpaEnabled={app.autoscaling?.enabled || false}
      />
      <NodePoolCard app={app} appId={appId} />
      <TerminationCard app={app} appId={appId} />
      <DangerZone
        description="Delete this application and all K3s resources."
        buttonLabel="Delete"
        onDelete={onDelete}
      />
    </div>
  );
}
