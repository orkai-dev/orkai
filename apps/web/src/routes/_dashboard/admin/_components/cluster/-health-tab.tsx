import { Link } from "@tanstack/react-router";
import { CheckCircle2, ChevronDown, ChevronRight, HeartPulse, Loader2, Trash2 } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  useCleanupCompletedJobs,
  useCleanupCompletedPods,
  useCleanupEvictedPods,
  useCleanupFailedPods,
  useCleanupOrphanIngresses,
  useCleanupStaleReplicaSets,
  useCleanupStats,
} from "@/features/cluster";

interface CleanupRowProps {
  label: string;
  description: string;
  count: number;
  names: string[];
  variant: "red" | "yellow" | "green-zero";
  mutation: { mutate: () => void; isPending: boolean };
}

function CleanupRow({ label, description, count, names = [], variant, mutation }: CleanupRowProps) {
  const [expanded, setExpanded] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [confirmText, setConfirmText] = useState("");

  const dotColor = count === 0 ? "bg-green-500" : variant === "red" ? "bg-red-500" : "bg-amber-500";

  return (
    <>
      <div className="flex items-center justify-between gap-4 py-3">
        <div className="flex items-center gap-3 min-w-0">
          <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${dotColor}`} />
          <div className="min-w-0">
            <p className="text-sm font-medium">{label}</p>
            <p className="text-xs text-muted-foreground">{description}</p>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {count === 0 ? (
            <span className="flex items-center gap-1 text-xs text-green-500">
              <CheckCircle2 className="h-3.5 w-3.5" /> All clean
            </span>
          ) : (
            <>
              <Badge variant="secondary" className="text-xs">
                {count}
              </Badge>
              <Button
                size="sm"
                variant="ghost"
                className="h-7 px-2 text-xs"
                onClick={() => setExpanded(!expanded)}
              >
                {expanded ? (
                  <ChevronDown className="h-3.5 w-3.5" />
                ) : (
                  <ChevronRight className="h-3.5 w-3.5" />
                )}
                View
              </Button>
              <Button
                size="sm"
                variant="destructive"
                className="h-7 px-2 text-xs"
                onClick={() => {
                  setConfirmText("");
                  setDialogOpen(true);
                }}
              >
                <Trash2 className="mr-1 h-3 w-3" /> Clean up
              </Button>
            </>
          )}
        </div>
      </div>

      {expanded && count > 0 && (
        <div className="mb-2 ml-5.5 rounded-md border bg-muted/50 p-3">
          <div className="max-h-40 space-y-1 overflow-y-auto">
            {names.map((name) => (
              <p key={name} className="font-mono text-xs text-muted-foreground">
                {name}
              </p>
            ))}
          </div>
        </div>
      )}

      <Dialog
        open={dialogOpen}
        onOpenChange={(v) => {
          setDialogOpen(v);
          if (!v) setConfirmText("");
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Clean up {label.toLowerCase()}</DialogTitle>
            <DialogDescription>
              This will permanently delete {count} {label.toLowerCase()}.
            </DialogDescription>
          </DialogHeader>
          <div className="max-h-48 overflow-y-auto rounded-md border bg-muted/50 p-3">
            {names.map((name) => (
              <p key={name} className="font-mono text-xs text-muted-foreground">
                {name}
              </p>
            ))}
          </div>
          <div className="space-y-2">
            <Label className="text-sm">
              Type <span className="font-mono font-bold">DELETE</span> to confirm
            </Label>
            <Input
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder="DELETE"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={confirmText !== "DELETE" || mutation.isPending}
              onClick={() => {
                mutation.mutate();
                setDialogOpen(false);
              }}
            >
              {mutation.isPending && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
              Confirm cleanup
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function ClusterHealthTab() {
  const { data: stats, isLoading } = useCleanupStats();
  const cleanEvicted = useCleanupEvictedPods();
  const cleanFailed = useCleanupFailedPods();
  const cleanCompleted = useCleanupCompletedPods();
  const cleanStaleRS = useCleanupStaleReplicaSets();
  const cleanJobs = useCleanupCompletedJobs();
  const cleanOrphanIngresses = useCleanupOrphanIngresses();

  if (isLoading || !stats) {
    return (
      <div className="mt-6 flex items-center justify-center py-12">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const totalIssues =
    stats.evicted_pods +
    stats.failed_pods +
    stats.completed_pods +
    stats.stale_replicasets +
    stats.orphan_ingresses +
    stats.completed_jobs +
    stats.unbound_pvcs;

  return (
    <div className="mt-3 space-y-4">
      <Card>
        <CardContent className="flex items-center gap-3 py-4">
          <div
            className={`flex h-9 w-9 items-center justify-center rounded-full ${
              totalIssues === 0
                ? "bg-green-500/10"
                : totalIssues <= 5
                  ? "bg-amber-500/10"
                  : "bg-red-500/10"
            }`}
          >
            <HeartPulse
              className={`h-5 w-5 ${
                totalIssues === 0
                  ? "text-green-500"
                  : totalIssues <= 5
                    ? "text-amber-500"
                    : "text-red-500"
              }`}
            />
          </div>
          <div>
            <p className="text-sm font-medium">
              {totalIssues === 0
                ? "All healthy"
                : `${totalIssues} issue${totalIssues !== 1 ? "s" : ""} found`}
            </p>
            <p className="text-xs text-muted-foreground">
              {totalIssues === 0
                ? "No stale or orphaned resources detected"
                : "Review and clean up stale resources below"}
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-sm">Pods</CardTitle>
          <CardDescription className="text-xs">Evicted, failed, and completed pods</CardDescription>
        </CardHeader>
        <CardContent className="divide-y">
          <CleanupRow
            label="Evicted pods"
            description="Pods evicted by the kubelet due to resource pressure"
            count={stats.evicted_pods}
            names={stats.evicted_pod_names}
            variant="red"
            mutation={cleanEvicted}
          />
          <CleanupRow
            label="Failed pods"
            description="Pods in a permanently failed state"
            count={stats.failed_pods}
            names={stats.failed_pod_names}
            variant="red"
            mutation={cleanFailed}
          />
          <CleanupRow
            label="Completed pods"
            description="Pods that have completed their execution"
            count={stats.completed_pods}
            names={stats.completed_pod_names}
            variant="yellow"
            mutation={cleanCompleted}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-sm">Workloads</CardTitle>
          <CardDescription className="text-xs">
            Stale ReplicaSets and completed Jobs
          </CardDescription>
        </CardHeader>
        <CardContent className="divide-y">
          <CleanupRow
            label="Stale ReplicaSets"
            description="Old ReplicaSets with zero replicas from previous rollouts"
            count={stats.stale_replicasets}
            names={stats.stale_rs_names}
            variant="yellow"
            mutation={cleanStaleRS}
          />
          <CleanupRow
            label="Completed Jobs"
            description="Jobs that have finished execution successfully"
            count={stats.completed_jobs}
            names={stats.completed_job_names}
            variant="yellow"
            mutation={cleanJobs}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-sm">Networking</CardTitle>
          <CardDescription className="text-xs">Orphan ingress resources</CardDescription>
        </CardHeader>
        <CardContent>
          <CleanupRow
            label="Orphan Ingresses"
            description="Ingresses without a matching domain record in the database"
            count={stats.orphan_ingresses}
            names={stats.orphan_ingress_names}
            variant="red"
            mutation={cleanOrphanIngresses}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-sm">Storage</CardTitle>
          <CardDescription className="text-xs">Unbound Persistent Volume Claims</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between gap-4 py-3">
            <div className="flex items-center gap-3 min-w-0">
              <span
                className={`h-2.5 w-2.5 shrink-0 rounded-full ${stats.unbound_pvcs === 0 ? "bg-green-500" : "bg-amber-500"}`}
              />
              <div className="min-w-0">
                <p className="text-sm font-medium">Unbound PVCs</p>
                <p className="text-xs text-muted-foreground">
                  PVCs stuck in Pending state without a matching volume
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              {stats.unbound_pvcs === 0 ? (
                <span className="flex items-center gap-1 text-xs text-green-500">
                  <CheckCircle2 className="h-3.5 w-3.5" /> All clean
                </span>
              ) : (
                <>
                  <Badge variant="secondary" className="text-xs">
                    {stats.unbound_pvcs}
                  </Badge>
                  <Link to="/admin/volumes" className="text-xs text-primary hover:underline">
                    Manage →
                  </Link>
                </>
              )}
            </div>
          </div>
          {stats.unbound_pvcs > 0 && stats.unbound_pvc_names?.length > 0 && (
            <div className="border-t px-3 py-2">
              <div className="max-h-24 overflow-y-auto space-y-0.5">
                {(stats.unbound_pvc_names ?? []).map((name) => (
                  <p key={name} className="font-mono text-xs text-muted-foreground">
                    {name}
                  </p>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
