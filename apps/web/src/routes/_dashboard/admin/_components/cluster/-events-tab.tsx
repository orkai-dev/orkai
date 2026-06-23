import { Activity } from "lucide-react";
import { useState } from "react";
import { EmptyState } from "@/components/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useMonitoringEvents } from "@/features/monitoring";
import { timeAgo } from "@/lib/format";
import { ErrorBanner, eventVariant } from "./-shared";

export function EventsTab() {
  const [page, setPage] = useState(1);
  const { data, isError } = useMonitoringEvents(page);
  if (isError) return <ErrorBanner message="Failed to load events" />;
  const events = data?.items ?? [];
  const total = data?.pagination?.total ?? 0;
  const totalPages = Math.ceil(total / 50);

  return (
    <div className="mt-3 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">{total} events (last 30 days)</p>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            Prev
          </Button>
          <span className="text-xs text-muted-foreground">
            {page} / {totalPages || 1}
          </span>
          <Button
            size="sm"
            variant="outline"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            Next
          </Button>
        </div>
      </div>

      {events.length === 0 ? (
        <EmptyState icon={Activity} message="No events recorded" />
      ) : (
        <Card>
          <CardContent className="divide-y p-0">
            {events.map((event) => (
              <div key={event.id} className="px-4 py-3">
                <div className="flex items-center gap-2">
                  <span className="shrink-0 text-xs text-muted-foreground">
                    {timeAgo(event.last_seen)}
                  </span>
                  <Badge variant={eventVariant(event.event_type)}>{event.event_type}</Badge>
                  <span className="text-sm font-medium">{event.reason}</span>
                  {event.count > 1 && (
                    <span className="text-xs text-muted-foreground">x{event.count}</span>
                  )}
                </div>
                <p className="mt-1 text-sm text-muted-foreground">{event.message}</p>
                <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                  <span className="font-mono">{event.involved_object}</span>
                  <span>·</span>
                  <Badge variant="outline" className="text-xs">
                    {event.namespace}
                  </Badge>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
