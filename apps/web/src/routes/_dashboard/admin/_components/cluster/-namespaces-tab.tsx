import { FolderOpen } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import type { NamespaceInfo } from "@/features/cluster/types";

export function NamespacesTab({ namespaces }: { namespaces: NamespaceInfo[] }) {
  if (namespaces.length === 0)
    return <EmptyState icon={FolderOpen} message="No namespaces found" />;

  return (
    <div className="mt-3 space-y-3">
      {namespaces.map((ns) => (
        <Card key={ns.name}>
          <CardContent className="flex items-center gap-4 p-4">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
              <FolderOpen className="h-4 w-4 text-primary" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">{ns.name}</span>
                <Badge variant={ns.status === "Active" ? "success" : "secondary"}>
                  {ns.status}
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground">
                {ns.pod_count} pods · {ns.svc_count} services
              </p>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
