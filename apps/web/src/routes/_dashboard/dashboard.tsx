import { createFileRoute, Link } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import {
  AlertTriangle,
  ArrowUpRight,
  ChevronRight,
  Cpu,
  Database,
  FolderKanban,
  HardDrive,
  Layers,
  Plus,
  Server,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useCurrentUser } from "@/features/auth";
import { useClusterMetrics, useClusterNodes, useClusterPods } from "@/features/cluster";
import { useDashboardApps, useDashboardDatabases } from "@/features/dashboard";
import { useActiveAlerts } from "@/features/monitoring";
import { useProjects } from "@/features/projects";
import type { Environment, Project } from "@/features/projects/types";
import { statusDotColor, statusDotPulse, statusVariant } from "@/lib/constants";
import { timeAgo } from "@/lib/format";
import { parseToMiB, parseToMillicores } from "@/lib/resources";
import { cn } from "@/lib/utils";
import type { PodInfo } from "@/shared/types";

export const Route = createFileRoute("/_dashboard/dashboard")({
  component: DashboardPage,
});

function pct(used: number, total: number): number {
  if (total <= 0) return 0;
  return Math.min(Math.round((used / total) * 100), 100);
}

function environmentVariant(env: Environment) {
  if (env === "prod") return "destructive" as const;
  if (env === "development") return "secondary" as const;
  return "outline" as const;
}

function DashboardMetric({
  icon: Icon,
  label,
  value,
  meta,
}: {
  icon: LucideIcon;
  label: string;
  value: string | number;
  meta?: string;
}) {
  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <div className="flex h-9 w-9 items-center justify-center border border-primary/30 bg-primary/10">
        <Icon className="h-4 w-4 text-primary" />
      </div>
      <div>
        <p className="text-label-caps text-muted-foreground">{label}</p>
        <div className="mt-1 flex items-baseline gap-2">
          <span className="font-mono text-xl font-semibold">{value}</span>
          {meta && <span className="text-xs text-muted-foreground">{meta}</span>}
        </div>
      </div>
    </div>
  );
}

function PanelSection({
  title,
  subtitle,
  action,
  children,
}: {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="border-b last:border-b-0">
      <div className="flex items-center justify-between gap-3 border-b bg-muted/20 px-4 py-3">
        <div>
          <p className="text-label-caps text-muted-foreground">{title}</p>
          {subtitle && <p className="mt-0.5 text-sm text-muted-foreground">{subtitle}</p>}
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}

function EmptyPanelRow({ message }: { message: string }) {
  return <p className="px-4 py-6 text-sm text-muted-foreground">{message}</p>;
}

function DashboardPage() {
  const { data: currentUser } = useCurrentUser();
  const isAdmin = currentUser?.role?.toLowerCase() === "admin";
  const { data: projects, isLoading } = useProjects();
  const { data: apps, isError: appsError } = useDashboardApps();
  const { data: databases, isError: dbsError } = useDashboardDatabases();
  const { data: cluster } = useClusterMetrics(isAdmin);
  const { data: nodes } = useClusterNodes(isAdmin);
  const { data: pods } = useClusterPods(isAdmin);
  const { data: alertsData } = useActiveAlerts();

  const allApps = apps ?? [];
  const allDbs = databases ?? [];
  const allPods = pods ?? [];
  const allNodes = nodes ?? [];
  const allProjects = projects ?? [];
  const alertCount = alertsData?.count ?? 0;
  const alerts = alertsData?.alerts ?? [];

  const cpuUsed = parseToMillicores(cluster?.resources.cpu_used ?? "");
  const cpuTotal = parseToMillicores(cluster?.resources.cpu_total ?? "");
  const memUsed = parseToMiB(cluster?.resources.mem_used ?? "");
  const memTotal = parseToMiB(cluster?.resources.mem_total ?? "");
  const runningPods = allPods.filter((p: PodInfo) => p.phase === "Running").length;
  const readyNodes = allNodes.filter((n) => n.status === "Ready").length;
  const prodProjects = allProjects.filter((p) => p.environment === "prod").length;
  const runningApps = allApps.filter((a) => a.status === "running").length;

  const greetingName =
    currentUser?.first_name?.trim() || currentUser?.display_name?.split(" ")[0] || "there";

  if (isLoading) {
    return (
      <div className="space-y-5">
        <Skeleton className="h-40 w-full" />
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
          <Skeleton className="h-96" />
          <Skeleton className="h-96" />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <div className="border bg-card">
        <div className="flex flex-col gap-4 border-b px-4 py-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p className="text-label-caps text-primary">Control plane</p>
            <h1 className="mt-2 text-display-lg">Welcome back, {greetingName}</h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              A live snapshot of your workspace — projects, workloads, and{" "}
              {isAdmin ? "cluster capacity" : "service health"} in one place.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" asChild>
              <Link to="/projects">
                <FolderKanban className="h-3.5 w-3.5" />
                Projects
              </Link>
            </Button>
            <Button variant="outline" size="sm" asChild>
              <Link to="/apps">
                <Layers className="h-3.5 w-3.5" />
                Apps
              </Link>
            </Button>
            {isAdmin && (
              <Button variant="outline" size="sm" asChild>
                <Link to="/admin/cluster">
                  <Server className="h-3.5 w-3.5" />
                  Cluster
                </Link>
              </Button>
            )}
          </div>
        </div>

        <div className="grid divide-y sm:grid-cols-2 sm:divide-x sm:divide-y-0 lg:grid-cols-4">
          {isAdmin ? (
            <>
              <DashboardMetric
                icon={Cpu}
                label="CPU allocated"
                value={`${Math.round(cpuUsed)}m`}
                meta={`${pct(cpuUsed, cpuTotal)}% of ${Math.round(cpuTotal)}m`}
              />
              <DashboardMetric
                icon={HardDrive}
                label="Memory allocated"
                value={`${Math.round(memUsed)}Mi`}
                meta={`${pct(memUsed, memTotal)}% of ${Math.round(memTotal)}Mi`}
              />
              <DashboardMetric
                icon={Layers}
                label="Running pods"
                value={runningPods}
                meta={`of ${allPods.length} total`}
              />
              <DashboardMetric
                icon={Server}
                label="Ready nodes"
                value={readyNodes}
                meta={`of ${allNodes.length} in cluster`}
              />
            </>
          ) : (
            <>
              <DashboardMetric
                icon={FolderKanban}
                label="Projects"
                value={allProjects.length}
                meta={prodProjects > 0 ? `${prodProjects} production` : undefined}
              />
              <DashboardMetric
                icon={Layers}
                label="Applications"
                value={allApps.length}
                meta={runningApps > 0 ? `${runningApps} running` : undefined}
              />
              <DashboardMetric icon={Database} label="Databases" value={allDbs.length} />
              <DashboardMetric
                icon={AlertTriangle}
                label="Active alerts"
                value={alertCount}
                meta={alertCount === 0 ? "all clear" : undefined}
              />
            </>
          )}
        </div>
      </div>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
        <div className="border bg-card">
          <PanelSection
            title="Projects"
            subtitle={`${allProjects.length} workspace${allProjects.length === 1 ? "" : "s"}`}
            action={
              <div className="flex items-center gap-2">
                {isAdmin && (
                  <Button variant="outline" size="sm" asChild>
                    <Link to="/projects" search={{ create: true }}>
                      <Plus className="h-3.5 w-3.5" />
                      New
                    </Link>
                  </Button>
                )}
                <Link
                  to="/projects"
                  className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                >
                  View all
                  <ArrowUpRight className="h-3 w-3" />
                </Link>
              </div>
            }
          >
            {!allProjects.length ? (
              <EmptyPanelRow message="No projects yet. Create one to start deploying workloads." />
            ) : (
              <div>
                {allProjects.slice(0, 8).map((project) => (
                  <ProjectRow key={project.id} project={project} />
                ))}
              </div>
            )}
          </PanelSection>
        </div>

        <div className="border bg-card">
          <PanelSection
            title="Alerts"
            subtitle={alertCount === 0 ? "No incidents" : `${alertCount} active`}
            action={
              isAdmin ? (
                <Link
                  to="/admin/cluster"
                  className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                >
                  Cluster
                  <ArrowUpRight className="h-3 w-3" />
                </Link>
              ) : undefined
            }
          >
            {alerts.length === 0 ? (
              <div className="flex items-center gap-3 px-4 py-5">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center border border-success/30 bg-success/10">
                  <AlertTriangle className="h-4 w-4 text-success" />
                </div>
                <p className="text-sm text-muted-foreground">All clear — no active alerts</p>
              </div>
            ) : (
              <div>
                {alerts.slice(0, 5).map((alert) => (
                  <div
                    key={alert.id}
                    className="flex items-start gap-3 border-b px-4 py-3 last:border-b-0"
                  >
                    <div
                      className={cn(
                        "mt-1.5 h-2 w-2 shrink-0 rounded-full",
                        alert.severity === "critical" ? "bg-destructive" : "bg-warning",
                      )}
                    />
                    <div className="min-w-0 flex-1">
                      <p className="text-sm leading-snug">{alert.message}</p>
                      <p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
                        {timeAgo(alert.fired_at)}
                      </p>
                    </div>
                    <Badge
                      variant={alert.severity === "critical" ? "destructive" : "warning"}
                      className="shrink-0"
                    >
                      {alert.severity}
                    </Badge>
                  </div>
                ))}
              </div>
            )}
          </PanelSection>

          <PanelSection
            title="Applications"
            subtitle={`${allApps.length} deployed`}
            action={
              <Link
                to="/apps"
                className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
              >
                View all
                <ArrowUpRight className="h-3 w-3" />
              </Link>
            }
          >
            {appsError ? (
              <EmptyPanelRow message="Failed to load applications." />
            ) : allApps.length === 0 ? (
              <EmptyPanelRow message="No applications deployed yet." />
            ) : (
              <div>
                {allApps.slice(0, 5).map((app) => (
                  <Link
                    key={app.id}
                    to="/projects/$id/apps/$appId"
                    params={{ id: app.project_id, appId: app.id }}
                    className="block"
                  >
                    <div className="flex items-center gap-3 border-b px-4 py-2.5 transition-colors last:border-b-0 hover:bg-muted/30">
                      <span
                        className={cn(
                          "status-dot",
                          statusDotColor(app.status),
                          statusDotPulse(app.status) && "status-dot-pulse",
                        )}
                      />
                      <span className="min-w-0 flex-1 truncate text-sm font-medium">
                        {app.name}
                      </span>
                      <Badge variant={statusVariant(app.status)}>{app.status}</Badge>
                    </div>
                  </Link>
                ))}
              </div>
            )}
          </PanelSection>

          <PanelSection title="Databases" subtitle={`${allDbs.length} managed`}>
            {dbsError ? (
              <EmptyPanelRow message="Failed to load databases." />
            ) : allDbs.length === 0 ? (
              <EmptyPanelRow message="No managed databases yet." />
            ) : (
              <div>
                {allDbs.slice(0, 5).map((db) => (
                  <Link
                    key={db.id}
                    to="/projects/$id/databases/$dbId"
                    params={{ id: db.project_id, dbId: db.id }}
                    className="block"
                  >
                    <div className="flex items-center gap-3 border-b px-4 py-2.5 transition-colors last:border-b-0 hover:bg-muted/30">
                      <Database className="h-3.5 w-3.5 shrink-0 text-primary" />
                      <span className="min-w-0 flex-1 truncate text-sm font-medium">{db.name}</span>
                      <Badge variant={statusVariant(db.status)}>{db.status}</Badge>
                    </div>
                  </Link>
                ))}
              </div>
            )}
          </PanelSection>
        </div>
      </div>
    </div>
  );
}

function ProjectRow({ project }: { project: Project }) {
  return (
    <Link to="/projects/$id" params={{ id: project.id }} className="block">
      <div className="flex items-center gap-3 border-b px-4 py-3 transition-colors last:border-b-0 hover:bg-muted/30">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center border border-primary/30 bg-primary/10">
          <FolderKanban className="h-4 w-4 text-primary" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="font-medium tracking-tight">{project.name}</p>
            <Badge variant={environmentVariant(project.environment)}>
              env:{project.environment}
            </Badge>
          </div>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">
            {project.description || project.namespace}
          </p>
        </div>
        <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
      </div>
    </Link>
  );
}
