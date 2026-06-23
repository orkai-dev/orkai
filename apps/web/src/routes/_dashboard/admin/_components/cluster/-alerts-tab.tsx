import { Activity } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useActiveAlerts, useResolveAlert } from "@/features/monitoring";
import type { MetricAlert } from "@/features/monitoring/types";
import { ErrorBanner } from "./-shared";

function AlertRow({ alert }: { alert: MetricAlert }) {
  const resolve = useResolveAlert();
  return (
    <Card>
      <CardContent className="flex items-center gap-4 p-4">
        <div
          className={`h-2.5 w-2.5 shrink-0 rounded-full ${alert.severity === "critical" ? "bg-destructive" : "bg-yellow-500"}`}
        />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">{alert.rule_name}</span>
            <Badge
              variant={alert.severity === "critical" ? "destructive" : "warning"}
              className="text-xs"
            >
              {alert.severity}
            </Badge>
            <span className="text-xs text-muted-foreground">{alert.source_name}</span>
          </div>
          <p className="text-xs text-muted-foreground">{alert.message}</p>
          <p className="text-xs text-muted-foreground">
            Fired {new Date(alert.fired_at).toLocaleString()}
          </p>
        </div>
        <Button
          size="sm"
          variant="outline"
          onClick={() => resolve.mutate(alert.id)}
          disabled={resolve.isPending}
        >
          {resolve.isPending ? "..." : "Resolve"}
        </Button>
      </CardContent>
    </Card>
  );
}

export function AlertsTab() {
  const { data: activeData, isError } = useActiveAlerts();
  const alerts = activeData?.alerts ?? [];

  if (isError) return <ErrorBanner message="Failed to load alerts" />;

  return (
    <div className="mt-3 space-y-3">
      {alerts.length === 0 ? (
        <EmptyState icon={Activity} message="No active alerts" />
      ) : (
        alerts.map((alert: MetricAlert) => <AlertRow key={alert.id} alert={alert} />)
      )}
    </div>
  );
}
