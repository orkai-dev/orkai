import { Database, HardDrive } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import type { PVCInfo } from "@/features/cluster/types";
import { pvcStatusVariant } from "./-shared";

export function StorageTab({ pvcs }: { pvcs: PVCInfo[] }) {
  if (pvcs.length === 0)
    return <EmptyState icon={HardDrive} message="No persistent volume claims found" />;

  return (
    <div className="mt-3 space-y-3">
      {pvcs.map((pvc) => (
        <Card key={`${pvc.namespace}/${pvc.name}`}>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
              <Database className="h-4 w-4 text-primary" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="font-mono text-sm">{pvc.name}</span>
                <Badge variant={pvcStatusVariant(pvc.status)}>{pvc.status}</Badge>
                <Badge variant="outline" className="text-xs">
                  {pvc.namespace}
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground">
                {pvc.capacity && `${pvc.capacity} · `}
                {pvc.storage_class && `${pvc.storage_class}`}
                {pvc.volume_name && ` · ${pvc.volume_name}`}
              </p>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
