import { createFileRoute, Link } from "@tanstack/react-router";
import { Filter, Globe, Layers, Loader2, Search } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { useAllApps } from "@/features/apps";
import type { App } from "@/features/apps/types";
import { useAllPages } from "@/features/pages";
import type { Page } from "@/features/pages/types";
import { useProjects } from "@/features/projects";
import { statusDotColor, statusDotPulse, statusVariant } from "@/lib/constants";
import { cn } from "@/lib/utils";

type WorkloadFilter = "docker" | "cloudflare_pages";

const APP_STATUS_OPTIONS = [
  { value: "running", label: "Running" },
  { value: "building", label: "Building" },
  { value: "deploying", label: "Deploying" },
  { value: "stopped", label: "Stopped" },
  { value: "error", label: "Error" },
] as const;

const PAGE_STATUS_OPTIONS = [
  { value: "live", label: "Live" },
  { value: "deploying", label: "Deploying" },
  { value: "idle", label: "Idle" },
  { value: "error", label: "Error" },
  { value: "draining", label: "Draining" },
] as const;

export const Route = createFileRoute("/_dashboard/apps")({
  component: AppsPage,
});

function AppsPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [workloadFilter, setWorkloadFilter] = useState<WorkloadFilter>("docker");
  const { data: projects } = useProjects();
  const isDocker = workloadFilter === "docker";
  const { data: appData, isLoading: appsLoading } = useAllApps(
    page,
    20,
    debouncedSearch || undefined,
    statusFilter || undefined,
    { enabled: isDocker },
  );
  const { data: pageData, isLoading: pagesLoading } = useAllPages(
    page,
    20,
    debouncedSearch || undefined,
    statusFilter || undefined,
    "cloudflare_pages",
    { enabled: !isDocker },
  );

  const isLoading = isDocker ? appsLoading : pagesLoading;
  const apps = appData?.items ?? [];
  const pages = pageData?.items ?? [];
  const itemCount = isDocker ? apps.length : pages.length;
  const pagination = isDocker ? appData?.pagination : pageData?.pagination;
  const totalPages = pagination ? Math.ceil(pagination.total / pagination.per_page) : 1;
  const projectMap = new Map(projects?.map((p) => [p.id, p.name]) ?? []);
  const statusOptions = isDocker ? APP_STATUS_OPTIONS : PAGE_STATUS_OPTIONS;

  const debounceRef = useRef<ReturnType<typeof setTimeout>>();
  useEffect(
    () => () => {
      clearTimeout(debounceRef.current);
    },
    [],
  );
  const handleSearch = (value: string) => {
    setSearch(value);
    setPage(1);
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => setDebouncedSearch(value), 300);
  };

  const handleWorkloadChange = (value: WorkloadFilter) => {
    setWorkloadFilter(value);
    setStatusFilter("");
    setSearch("");
    setDebouncedSearch("");
    clearTimeout(debounceRef.current);
    setPage(1);
  };

  return (
    <div>
      <PageHeader
        title="Apps"
        description={
          isDocker
            ? "Docker workloads running on your cluster."
            : "Cloudflare Pages static sites across projects."
        }
      />
      <Separator className="my-5" />

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground/50" />
          <Input
            value={search}
            onChange={(e) => handleSearch(e.target.value)}
            placeholder={isDocker ? "Search apps..." : "Search pages..."}
            className="h-8 pl-8 text-sm"
          />
        </div>
        <div className="flex items-center gap-2">
          <Filter className="h-3.5 w-3.5 text-muted-foreground/50" />
          <Select
            value={workloadFilter}
            onValueChange={(v) => handleWorkloadChange(v as WorkloadFilter)}
          >
            <SelectTrigger className="h-8 w-44 text-sm">
              <SelectValue placeholder="Workload type" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="docker">Docker workloads</SelectItem>
              <SelectItem value="cloudflare_pages">Cloudflare Pages</SelectItem>
            </SelectContent>
          </Select>
          <Select
            value={statusFilter || "all"}
            onValueChange={(v) => {
              setStatusFilter(v === "all" ? "" : v);
              setPage(1);
            }}
          >
            <SelectTrigger className="h-8 w-36 text-sm">
              <SelectValue placeholder="All statuses" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              {statusOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : itemCount === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10">
              {isDocker ? (
                <Layers className="h-5 w-5 text-primary" />
              ) : (
                <Globe className="h-5 w-5 text-primary" />
              )}
            </div>
            <p className="mt-3 text-sm text-muted-foreground">
              {debouncedSearch || statusFilter
                ? "No workloads match your filters."
                : isDocker
                  ? "No apps yet."
                  : "No Cloudflare Pages yet."}
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="space-y-2">
            {isDocker
              ? apps.map((app) => (
                  <AppRow key={app.id} app={app} projectName={projectMap.get(app.project_id)} />
                ))
              : pages.map((item) => (
                  <PageRow
                    key={item.id}
                    page={item}
                    projectName={projectMap.get(item.project_id)}
                  />
                ))}
          </div>

          {pagination && totalPages > 1 && (
            <div className="mt-4 flex items-center justify-between">
              <p className="text-xs text-muted-foreground">
                Page {pagination.page} of {totalPages} ({pagination.total} total)
              </p>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={page <= 1}
                  onClick={() => setPage(page - 1)}
                >
                  Previous
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={page >= totalPages}
                  onClick={() => setPage(page + 1)}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function AppRow({ app, projectName }: { app: App; projectName?: string }) {
  return (
    <Link
      to="/projects/$id/apps/$appId"
      params={{ id: app.project_id, appId: app.id }}
      className="block"
    >
      <Card className="transition-colors hover:bg-accent/50">
        <CardContent className="flex items-center gap-3 p-4">
          <span
            className={cn(
              "status-dot",
              statusDotColor(app.status),
              statusDotPulse(app.status) && "status-dot-pulse",
            )}
          />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="truncate text-sm font-medium">{app.name}</span>
              {projectName && (
                <Badge
                  variant="outline"
                  className="shrink-0 text-xs font-normal text-muted-foreground"
                >
                  {projectName}
                </Badge>
              )}
            </div>
            <p className="mt-0.5 truncate text-xs text-muted-foreground">
              {app.source_type === "git"
                ? app.git_repo?.split("/").slice(-1)[0]
                : app.docker_image || app.source_type}
            </p>
          </div>
          <Badge variant={statusVariant(app.status)} className="shrink-0 text-xs">
            {app.status}
          </Badge>
        </CardContent>
      </Card>
    </Link>
  );
}

function PageRow({ page, projectName }: { page: Page; projectName?: string }) {
  return (
    <Link
      to="/projects/$id/pages/$pageId"
      params={{ id: page.project_id, pageId: page.id }}
      className="block"
    >
      <Card className="transition-colors hover:bg-accent/50">
        <CardContent className="flex items-center gap-3 p-4">
          <Globe className="h-4 w-4 shrink-0 text-primary" />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="truncate text-sm font-medium">{page.name}</span>
              {projectName && (
                <Badge
                  variant="outline"
                  className="shrink-0 text-xs font-normal text-muted-foreground"
                >
                  {projectName}
                </Badge>
              )}
            </div>
            <p className="mt-0.5 truncate text-xs text-muted-foreground">
              {page.git_repo} @ {page.git_branch}
            </p>
          </div>
          <Badge variant={statusVariant(page.status)} className="shrink-0 text-xs">
            {page.status}
          </Badge>
        </CardContent>
      </Card>
    </Link>
  );
}
