import { Badge } from "@/components/ui/badge";
import { statusDotColor, statusVariant } from "@/lib/constants";
import type { PodInfo } from "@/shared/types";
import { formatCPU, formatMem } from "./-db-helpers";

export function DbPodRow({ pod }: { pod: PodInfo }) {
  return (
    <div className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
      <div className="flex items-center gap-3">
        <Badge variant={statusVariant(pod.phase)} className="text-xs">
          {pod.phase}
        </Badge>
        <span className="font-mono text-xs">{pod.name}</span>
        <span
          className={`inline-block h-2 w-2 rounded-full ${pod.ready ? statusDotColor("running") : statusDotColor("error")}`}
          title={pod.ready ? "Ready" : "Not ready"}
        />
      </div>
      {pod.resources && (
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          <span>
            CPU: {formatCPU(pod.resources.cpu_used)}/{formatCPU(pod.resources.cpu_total)}
          </span>
          <span>
            Mem: {formatMem(pod.resources.mem_used)}/{formatMem(pod.resources.mem_total)}
          </span>
        </div>
      )}
    </div>
  );
}
