import { createFileRoute, Link, useNavigate, useSearch } from "@tanstack/react-router";
import {
  CalendarDays,
  ChevronRight,
  FolderKanban,
  Layers3,
  Plus,
  Search,
  Shield,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge, type BadgeProps } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useCurrentUser } from "@/features/auth";
import { useCreateProject, useProjects } from "@/features/projects";
import type { Environment, Project } from "@/features/projects/types";
import { useTeams } from "@/features/teams";
import { BRAND_NAME } from "@/lib/brand";
import { ENVIRONMENTS } from "@/lib/constants";
import { timeAgo } from "@/lib/format";

export const Route = createFileRoute("/_dashboard/projects")({
  component: ProjectsPage,
  validateSearch: (search: Record<string, unknown>) => ({
    create: search.create === true || search.create === "true",
  }),
});

function ProjectsPage() {
  const { create } = useSearch({ from: "/_dashboard/projects" });
  const navigate = useNavigate();
  const { data: projects, isLoading } = useProjects();
  const { data: currentUser } = useCurrentUser();
  const createProject = useCreateProject();
  const role = currentUser?.role?.toLowerCase();
  const canCreate = role === "admin";
  const { data: teams } = useTeams({ enabled: canCreate });
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [environment, setEnvironment] = useState<Environment>("development");
  const [teamId, setTeamId] = useState("");
  const [query, setQuery] = useState("");
  const [environmentFilter, setEnvironmentFilter] = useState<"all" | Environment>("all");

  useEffect(() => {
    if (create) {
      setOpen(true);
      navigate({ to: "/projects", search: {}, replace: true });
    }
  }, [create, navigate]);

  const safeProjects = projects ?? [];
  const filteredProjects = useMemo(() => {
    const term = query.trim().toLowerCase();

    return safeProjects.filter((project) => {
      const matchesEnvironment =
        environmentFilter === "all" || project.environment === environmentFilter;
      const matchesQuery =
        !term ||
        [project.name, project.description, project.team?.name, project.namespace]
          .filter(Boolean)
          .some((value) => value.toLowerCase().includes(term));

      return matchesEnvironment && matchesQuery;
    });
  }, [environmentFilter, query, safeProjects]);

  const productionCount = safeProjects.filter((project) => project.environment === "prod").length;
  const policyCount = safeProjects.filter((project) => project.network_policy_enabled).length;
  const teamCount = new Set(safeProjects.map((project) => project.team?.name).filter(Boolean)).size;

  function resetForm() {
    setName("");
    setDescription("");
    setEnvironment("development");
    setTeamId("");
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!teamId) {
      toast.error("Team is required");
      return;
    }
    await createProject.mutateAsync({
      name: name.trim(),
      description: description.trim(),
      environment,
      team_id: teamId,
    });
    resetForm();
    setOpen(false);
  }

  if (isLoading) return <LoadingScreen />;

  return (
    <div className="space-y-5">
      <div className="border bg-card">
        <div className="flex flex-col gap-4 border-b px-4 py-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p className="text-label-caps text-primary">Workspace inventory</p>
            <h1 className="mt-2 text-display-lg">Projects</h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              Group applications, databases, cron jobs, and policies into deployable environments.
            </p>
          </div>
          {canCreate && (
            <Button onClick={() => setOpen(true)}>
              <Plus className="h-4 w-4" /> New Project
            </Button>
          )}
        </div>
        <div className="grid divide-y sm:grid-cols-3 sm:divide-x sm:divide-y-0">
          <ProjectMetric icon={FolderKanban} label="Total projects" value={safeProjects.length} />
          <ProjectMetric icon={Layers3} label="Production" value={productionCount} />
          <ProjectMetric
            icon={Shield}
            label="Network policies"
            value={policyCount}
            meta={`${teamCount} teams`}
          />
        </div>
      </div>

      <Dialog
        open={open}
        onOpenChange={(v) => {
          setOpen(v);
          if (!v) resetForm();
        }}
      >
        <DialogContent className="max-w-2xl gap-0 overflow-hidden p-0">
          <form onSubmit={handleCreate}>
            <div className="border-b bg-muted/40 px-6 py-5 pr-14">
              <p className="text-label-caps text-primary">Project setup</p>
              <DialogTitle className="mt-2">New Project</DialogTitle>
              <p className="mt-1 text-sm text-muted-foreground">
                Create a namespace boundary for services, environment variables, and cluster policy.
              </p>
            </div>
            <div className="grid gap-4 px-6 py-5">
              <div className="space-y-2">
                <Label htmlFor="project-name">Name</Label>
                <Input
                  id="project-name"
                  className="border-border bg-background"
                  placeholder="api-platform"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                  autoFocus
                />
                <p className="text-xs text-muted-foreground">
                  Use a short, memorable name. It will identify this project across the control
                  plane.
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="project-description">Description</Label>
                <Input
                  id="project-description"
                  className="border-border bg-background"
                  placeholder="Optional context for the team"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="project-environment">Environment</Label>
                  <Select
                    value={environment}
                    onValueChange={(value) => setEnvironment(value as Environment)}
                  >
                    <SelectTrigger id="project-environment" className="border-border bg-background">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {ENVIRONMENTS.map((env) => (
                        <SelectItem key={env} value={env}>
                          {env}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="project-team">Team</Label>
                <Select value={teamId} onValueChange={setTeamId}>
                  <SelectTrigger id="project-team" className="border-border bg-background">
                    <SelectValue placeholder="Select a team" />
                  </SelectTrigger>
                  <SelectContent>
                    {(teams ?? []).map((team) => (
                      <SelectItem key={team.id} value={team.id}>
                        {team.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {teams && teams.length === 0 && (
                  <p className="text-xs text-muted-foreground">
                    No teams yet. Create a team on the Team page first.
                  </p>
                )}
              </div>
              <div className="border bg-background px-3 py-2 text-xs text-muted-foreground">
                {BRAND_NAME} will use this metadata to organize the project view and apply
                environment-aware defaults.
              </div>
            </div>
            <div className="flex flex-col-reverse gap-2 border-t px-6 py-4 sm:flex-row sm:justify-end">
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={createProject.isPending || !name.trim() || !teamId}>
                {createProject.isPending ? "Creating..." : "Create Project"}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      {!safeProjects.length ? (
        <div>
          <EmptyState
            icon={FolderKanban}
            message={
              canCreate
                ? "No projects yet. Create your first project to get started."
                : "No projects yet. Ask an admin to add you to a team."
            }
            actionLabel={canCreate ? "New Project" : undefined}
            onAction={canCreate ? () => setOpen(true) : undefined}
          />
        </div>
      ) : (
        <div className="space-y-3">
          <div className="grid gap-3 border bg-card p-3 md:grid-cols-[1fr_220px]">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-9"
                placeholder="Search projects by name, team, namespace..."
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            <Select
              value={environmentFilter}
              onValueChange={(value) => setEnvironmentFilter(value as "all" | Environment)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All environments</SelectItem>
                {ENVIRONMENTS.map((env) => (
                  <SelectItem key={env} value={env}>
                    {env}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {filteredProjects.length ? (
            <div className="grid gap-2">
              {filteredProjects.map((project) => (
                <ProjectRow key={project.id} project={project} />
              ))}
            </div>
          ) : (
            <Card>
              <CardContent className="flex flex-col items-center px-4 py-14 text-center">
                <FolderKanban className="h-10 w-10 text-muted-foreground/30" />
                <p className="mt-3 font-medium">No projects match this view</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  Clear the search or switch environments to see more projects.
                </p>
                <Button
                  className="mt-4"
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setQuery("");
                    setEnvironmentFilter("all");
                  }}
                >
                  Reset filters
                </Button>
              </CardContent>
            </Card>
          )}
        </div>
      )}
    </div>
  );
}

function ProjectMetric({
  icon: Icon,
  label,
  value,
  meta,
}: {
  icon: React.ElementType;
  label: string;
  value: number;
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
          <span className="text-xl font-semibold tracking-tight">{value}</span>
          {meta && <span className="text-xs text-muted-foreground">{meta}</span>}
        </div>
      </div>
    </div>
  );
}

function ProjectRow({ project }: { project: Project }) {
  return (
    <Link to="/projects/$id" params={{ id: project.id }} className="block">
      <Card className="group cursor-pointer transition-colors hover:border-primary/50 hover:bg-accent/40">
        <CardContent className="grid gap-4 p-4 lg:grid-cols-[1fr_auto] lg:items-center">
          <div className="flex min-w-0 gap-3">
            <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center border border-primary/30 bg-primary/10">
              <FolderKanban className="h-4 w-4 text-primary" />
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <p className="font-medium tracking-tight">{project.name}</p>
                <Badge variant={environmentVariant(project.environment)}>
                  env:{project.environment}
                </Badge>
              </div>
              <p className="mt-1 line-clamp-1 text-sm text-muted-foreground">
                {project.description || "No description added yet"}
              </p>
              <div className="mt-3 flex flex-wrap gap-2 text-code-sm text-muted-foreground">
                <span className="border bg-background px-2 py-1">
                  team:{project.team?.name || "unassigned"}
                </span>
                {project.namespace && (
                  <span className="border bg-background px-2 py-1">ns:{project.namespace}</span>
                )}
                {project.network_policy_enabled && (
                  <span className="border border-success/30 bg-success/10 px-2 py-1 text-success">
                    network-policy:on
                  </span>
                )}
              </div>
            </div>
          </div>
          <div className="flex items-center justify-between gap-4 lg:justify-end">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <CalendarDays className="h-4 w-4" />
              <span>{timeAgo(project.created_at)}</span>
            </div>
            <ChevronRight className="h-4 w-4 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-primary" />
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}

function environmentVariant(environment: Environment): NonNullable<BadgeProps["variant"]> {
  switch (environment) {
    case "prod":
      return "success";
    case "testing":
    case "qa":
      return "warning";
    case "sandbox":
    case "poc":
      return "outline";
    case "development":
      return "secondary";
  }
}
