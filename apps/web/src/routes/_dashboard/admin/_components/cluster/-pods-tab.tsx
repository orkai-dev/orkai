import { Box, Cpu, MemoryStick } from "lucide-react";
import { useMemo, useState } from "react";
import { EmptyState } from "@/components/empty-state";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { NamespaceInfo } from "@/features/cluster/types";
import { statusVariant } from "@/lib/constants";
import { timeAgo } from "@/lib/format";
import type { PodInfo } from "@/shared/types";

export function PodsTab({ pods, namespaces }: { pods: PodInfo[]; namespaces: NamespaceInfo[] }) {
  const [nsFilter, setNsFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");

  const filtered = useMemo(() => {
    let result = pods;
    if (nsFilter !== "all") {
      result = result.filter((p) => p.namespace === nsFilter);
    }
    if (statusFilter !== "all") {
      result = result.filter((p) => p.phase === statusFilter);
    }
    return result;
  }, [pods, nsFilter, statusFilter]);

  return (
    <div className="mt-3 space-y-3">
      <div className="flex gap-3">
        <Select value={nsFilter} onValueChange={setNsFilter}>
          <SelectTrigger className="w-[200px]">
            <SelectValue placeholder="All namespaces" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Namespaces</SelectItem>
            {namespaces.map((ns) => (
              <SelectItem key={ns.name} value={ns.name}>
                {ns.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-[160px]">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="Running">Running</SelectItem>
            <SelectItem value="Pending">Pending</SelectItem>
            <SelectItem value="Failed">Failed</SelectItem>
            <SelectItem value="Succeeded">Succeeded</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {filtered.length === 0 ? (
        <EmptyState icon={Box} message="No pods match the current filters" />
      ) : (
        <Card>
          <CardContent className="divide-y p-0">
            {filtered.map((pod) => (
              <div
                key={`${pod.namespace}/${pod.name}`}
                className="flex items-center gap-4 px-4 py-3"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="truncate font-mono text-sm">
                      {pod.namespace}/{pod.name}
                    </span>
                    <Badge variant={statusVariant(pod.phase)}>{pod.phase}</Badge>
                    {pod.restart_count > 0 && (
                      <Badge variant="warning" className="text-xs">
                        {pod.restart_count} restarts
                      </Badge>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {pod.node} · {pod.ip}
                    {pod.started_at ? ` · ${timeAgo(pod.started_at)}` : ""}
                  </p>
                </div>
                <div className="hidden gap-4 text-xs text-muted-foreground md:flex">
                  <span className="flex items-center gap-1">
                    <Cpu className="h-3 w-3" /> {pod.resources.cpu_used || "N/A"}
                  </span>
                  <span className="flex items-center gap-1">
                    <MemoryStick className="h-3 w-3" /> {pod.resources.mem_used || "N/A"}
                  </span>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
