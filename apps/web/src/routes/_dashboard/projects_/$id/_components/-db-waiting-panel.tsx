import { AlertTriangle, Loader2 } from "lucide-react";
import { DangerZone } from "@/components/danger-zone";
import { Card, CardContent } from "@/components/ui/card";
import type { PodInfo } from "@/shared/types";
import { DbPodsSection } from "./-db-pods-section";

export function DbWaitingPanel({
  livePhase,
  isStarting,
  pods,
  onDelete,
}: {
  livePhase: string;
  isStarting: boolean;
  pods: PodInfo[];
  onDelete: () => void;
}) {
  return (
    <div className="space-y-6">
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-12">
          {isStarting ? (
            <>
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-blue-500/10">
                <Loader2 className="h-6 w-6 animate-spin text-blue-500" />
              </div>
              <h3 className="mt-4 text-sm font-medium">Starting database...</h3>
              <p className="mt-1 max-w-sm text-center text-xs text-muted-foreground">
                Pulling image and running health checks. This may take a minute for first-time
                deployments.
              </p>
            </>
          ) : (
            <>
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-yellow-500/10">
                <AlertTriangle className="h-6 w-6 text-yellow-500" />
              </div>
              <h3 className="mt-4 text-sm font-medium">Database is {livePhase}</h3>
              <p className="mt-1 max-w-sm text-center text-xs text-muted-foreground">
                Connection info, backups, and settings will be available once the database is
                running.
              </p>
            </>
          )}
        </CardContent>
      </Card>

      {pods.length > 0 && <DbPodsSection pods={pods} />}

      <DangerZone
        description="Delete this database. All data will be permanently lost."
        buttonLabel="Delete Database"
        onDelete={onDelete}
      />
    </div>
  );
}
