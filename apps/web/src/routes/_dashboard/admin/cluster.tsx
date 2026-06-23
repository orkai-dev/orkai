import { createFileRoute } from "@tanstack/react-router";
import { ClusterTopologyView } from "@/components/cluster-topology";
import { LoadingScreen } from "@/components/loading-screen";
import { StatCardCompact } from "@/components/stat-card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  useClusterMetrics,
  useClusterNamespaces,
  useClusterNodes,
  useClusterPods,
  useClusterPVCs,
  useClusterTopology,
  useNodeMetrics,
} from "@/features/cluster";
import { useActiveAlerts } from "@/features/monitoring";
import { parseResourceValue, pctString } from "@/lib/resources";
import { AlertsTab } from "./_components/cluster/-alerts-tab";
import { DaemonSetsTab } from "./_components/cluster/-daemonsets-tab";
import { EventsTab } from "./_components/cluster/-events-tab";
import { ClusterHealthTab } from "./_components/cluster/-health-tab";
import { HelmTab } from "./_components/cluster/-helm-tab";
import { NamespacesTab } from "./_components/cluster/-namespaces-tab";
import { NodeTrendCharts } from "./_components/cluster/-node-trend-charts";
import { NodesTab } from "./_components/cluster/-nodes-tab";
import { PodsTab } from "./_components/cluster/-pods-tab";
import { ErrorBanner } from "./_components/cluster/-shared";
import { StorageTab } from "./_components/cluster/-storage-tab";

export const Route = createFileRoute("/_dashboard/admin/cluster")({
  component: ClusterPage,
});

function ClusterPage() {
  const { data: nodes, isError: nodesError } = useClusterNodes();
  const { data: metrics, isLoading: metricsLoading } = useClusterMetrics();
  const { data: nodeMetrics, isLoading: nodeMetricsLoading } = useNodeMetrics();
  const { data: pods, isError: podsError } = useClusterPods();
  const { data: pvcs, isError: pvcsError } = useClusterPVCs();
  const { data: namespaces, isError: nsError } = useClusterNamespaces();
  const { data: activeAlertsData } = useActiveAlerts();
  const { data: topology, isError: topoError } = useClusterTopology();
  const activeAlertCount = activeAlertsData?.count ?? 0;

  const loading = metricsLoading || nodeMetricsLoading;

  if (loading) return <LoadingScreen variant="detail" />;

  const cpuPct = nodeMetrics?.length
    ? pctString(
        `${nodeMetrics.reduce((a, n) => a + parseResourceValue(n.cpu_used), 0)}m`,
        `${nodeMetrics.reduce((a, n) => a + parseResourceValue(n.cpu_total), 0)}m`,
      )
    : "N/A";

  const memPct = nodeMetrics?.length
    ? pctString(
        `${nodeMetrics.reduce((a, n) => a + parseResourceValue(n.mem_used), 0)}Mi`,
        `${nodeMetrics.reduce((a, n) => a + parseResourceValue(n.mem_total), 0)}Mi`,
      )
    : "N/A";

  return (
    <div>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Cluster</h1>
        <p className="text-sm text-muted-foreground">K3s cluster overview and monitoring</p>
      </div>

      <div className="mt-6 grid gap-4 md:grid-cols-5">
        <StatCardCompact label="Nodes" value={metrics?.nodes ?? 0} />
        <StatCardCompact
          label="Pods"
          value={metrics ? `${metrics.running_pods}/${metrics.total_pods}` : "0"}
        />
        <StatCardCompact label="CPU" value={cpuPct} />
        <StatCardCompact label="Memory" value={memPct} />
        <StatCardCompact label="PVCs" value={pvcs?.length ?? 0} />
      </div>

      <NodeTrendCharts />

      <Tabs defaultValue="nodes" className="mt-6">
        <TabsList>
          <TabsTrigger value="topology">Topology</TabsTrigger>
          <TabsTrigger value="nodes">Nodes</TabsTrigger>
          <TabsTrigger value="pods">Pods</TabsTrigger>
          <TabsTrigger value="events">Events</TabsTrigger>
          <TabsTrigger value="alerts" className="relative">
            Alerts
            {activeAlertCount > 0 && (
              <span className="ml-1.5 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-xs font-bold text-white">
                {activeAlertCount}
              </span>
            )}
          </TabsTrigger>
          <TabsTrigger value="storage">Storage</TabsTrigger>
          <TabsTrigger value="namespaces">Namespaces</TabsTrigger>
          <TabsTrigger value="helm">Helm</TabsTrigger>
          <TabsTrigger value="daemonsets">DaemonSets</TabsTrigger>
          <TabsTrigger value="health">Health</TabsTrigger>
        </TabsList>

        <TabsContent value="topology">
          {topoError ? (
            <ErrorBanner message="Failed to load cluster topology" />
          ) : (
            topology && <ClusterTopologyView data={topology} />
          )}
        </TabsContent>
        <TabsContent value="nodes">
          {nodesError ? (
            <ErrorBanner message="Failed to load nodes" />
          ) : (
            <NodesTab nodes={nodes ?? []} nodeMetrics={nodeMetrics ?? []} />
          )}
        </TabsContent>
        <TabsContent value="pods">
          {podsError ? (
            <ErrorBanner message="Failed to load pods" />
          ) : (
            <PodsTab pods={pods ?? []} namespaces={namespaces ?? []} />
          )}
        </TabsContent>
        <TabsContent value="events">
          <EventsTab />
        </TabsContent>
        <TabsContent value="alerts">
          <AlertsTab />
        </TabsContent>
        <TabsContent value="storage">
          {pvcsError ? (
            <ErrorBanner message="Failed to load volumes" />
          ) : (
            <StorageTab pvcs={pvcs ?? []} />
          )}
        </TabsContent>
        <TabsContent value="namespaces">
          {nsError ? (
            <ErrorBanner message="Failed to load namespaces" />
          ) : (
            <NamespacesTab namespaces={namespaces ?? []} />
          )}
        </TabsContent>
        <TabsContent value="helm">
          <HelmTab />
        </TabsContent>
        <TabsContent value="daemonsets">
          <DaemonSetsTab />
        </TabsContent>
        <TabsContent value="health">
          <ClusterHealthTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
