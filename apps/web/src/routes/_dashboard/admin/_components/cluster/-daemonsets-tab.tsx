import { Server } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { useDaemonSets } from "@/features/cluster";
import type { DaemonSetInfo } from "@/features/cluster/types";
import { timeAgo } from "@/lib/format";
import { ErrorBanner } from "./-shared";

export function DaemonSetsTab() {
  const { data: daemonsets, isLoading, isError } = useDaemonSets();

  if (isError) return <ErrorBanner message="Failed to load DaemonSets" />;
  if (isLoading) return <LoadingScreen variant="detail" />;

  if (!daemonsets || daemonsets.length === 0) {
    return <EmptyState icon={Server} message="No DaemonSets found" />;
  }

  return (
    <div className="mt-3 space-y-3">
      {daemonsets.map((ds: DaemonSetInfo) => (
        <Card key={`${ds.namespace}/${ds.name}`}>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold">{ds.name}</span>
                <Badge variant="outline" className="text-xs">
                  {ds.namespace}
                </Badge>
                <span className="text-xs text-muted-foreground">
                  {ds.ready}/{ds.desired_scheduled} ready
                </span>
              </div>
              <p className="truncate text-xs text-muted-foreground">
                {ds.node_selector && `selector: ${ds.node_selector} · `}
                {ds.images &&
                  `${ds.images.length > 80 ? `${ds.images.slice(0, 80)}...` : ds.images}`}
                {ds.created_at && ` · ${timeAgo(ds.created_at)}`}
              </p>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
