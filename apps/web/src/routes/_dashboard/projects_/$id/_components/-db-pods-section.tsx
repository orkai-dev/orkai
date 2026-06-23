import { HardDrive } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { PodInfo } from "@/shared/types";
import { DbPodRow } from "./-db-pod-row";

export function DbPodsSection({ pods }: { pods: PodInfo[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <HardDrive className="h-4 w-4" /> Pods
        </CardTitle>
      </CardHeader>
      <CardContent>
        {pods.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No pods running yet. Pods will appear once the database is deployed.
          </p>
        ) : (
          <div className="space-y-2">
            {pods.map((pod) => (
              <DbPodRow key={pod.name} pod={pod} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
