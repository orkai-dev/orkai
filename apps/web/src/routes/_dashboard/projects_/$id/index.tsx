import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Container, Plus, Settings2 } from "lucide-react";
import { useEffect, useState } from "react";
import { DangerZone } from "@/components/danger-zone";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useCreateApp, useProjectApps } from "@/features/apps";
import { useCreateDatabase, useProjectDatabases } from "@/features/databases";
import { useCreatePage, useProjectPages } from "@/features/pages/queries";
import { useDeleteProject, useProject, useUpdateProject } from "@/features/projects";
import { useCreateWorker, useProjectWorkers } from "@/features/workers/queries";
import { ENVIRONMENTS } from "@/lib/constants";
import { CreateServiceDialog, type ServiceType } from "./_components/-create-service-dialog";
import { DeleteProjectDialog } from "./_components/-delete-project-dialog";
import { DeleteServiceDialog } from "./_components/-delete-service-dialog";
import { ProjectEnvEditor } from "./_components/-project-env-editor";
import { ServiceList } from "./_components/-service-list";
import type { ServiceItem } from "./_components/-service-types";

export const Route = createFileRoute("/_dashboard/projects_/$id/")({
  component: ProjectDetailPage,
});

function ProjectDetailPage() {
  const { id: projectId } = Route.useParams();
  const navigate = useNavigate();

  const { data: project, isLoading: projectLoading } = useProject(projectId);
  const { data: rawApps, isLoading: appsLoading } = useProjectApps(projectId);
  const { data: rawDatabases, isLoading: dbsLoading } = useProjectDatabases(projectId);
  const { data: rawPages, isLoading: pagesLoading } = useProjectPages(projectId);
  const { data: rawWorkers, isLoading: workersLoading } = useProjectWorkers(projectId);
  const apps = rawApps ?? [];
  const databases = rawDatabases ?? [];
  const pages = rawPages ?? [];
  const workers = rawWorkers ?? [];
  const loading = projectLoading || appsLoading || dbsLoading || pagesLoading || workersLoading;

  const updateProject = useUpdateProject(projectId);
  const deleteProject = useDeleteProject(projectId);
  const createApp = useCreateApp(projectId);
  const createDatabase = useCreateDatabase(projectId);
  const createPage = useCreatePage(projectId);
  const createWorker = useCreateWorker(projectId);

  const [showCreate, setShowCreate] = useState(false);
  const [showDeleteProject, setShowDeleteProject] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ServiceItem | null>(null);
  const [serviceType, setServiceType] = useState<ServiceType>("app");
  const [appForm, setAppForm] = useState({
    name: "",
    is_critical: false,
    source_type: "image",
    docker_image: "",
    git_repo: "",
    git_branch: "main",
    registry_id: "",
  });
  const [dbForm, setDbForm] = useState({
    name: "",
    database_name: "",
    engine: "postgres",
    version: "",
    storage_size: "1Gi",
  });
  const [pageForm, setPageForm] = useState({
    name: "",
    git_repo: "",
    git_branch: "main",
    git_provider_id: "",
    publish_path: ".",
    build_enabled: false,
    package_manager: "auto" as "auto" | "npm" | "pnpm",
    install_command: "",
    build_command: "",
    output_dir: "",
    root_directory: ".",
    node_version: "",
    build_env_vars: {} as Record<string, string>,
    provider: "aws_cloudfront" as "aws_cloudfront" | "cloudflare_pages",
    cloud_account_id: "",
    region: "us-east-1",
    dns_mode: "none" as "none" | "manual" | "route53" | "cloudflare",
    custom_domain: "",
    subdomain: "",
    manage_dns: false,
    dns_account_id: "",
    dns_zone_id: "",
  });
  const [workerForm, setWorkerForm] = useState({
    name: "",
    git_repo: "",
    git_branch: "main",
    git_provider_id: "",
    root_directory: ".",
    wrangler_config: "wrangler.toml",
    package_manager: "auto" as "auto" | "npm" | "pnpm",
    install_command: "",
    build_command: "",
    deploy_command: "",
    build_env_vars: {} as Record<string, string>,
    cloud_account_id: "",
  });
  const [editName, setEditName] = useState(project?.name ?? "");
  const [editDesc, setEditDesc] = useState(project?.description ?? "");
  const [editEnv, setEditEnv] = useState(project?.environment ?? "development");

  // biome-ignore lint/correctness/useExhaustiveDependencies: only sync on metadata value change
  useEffect(() => {
    if (project) {
      setEditName(project.name);
      setEditDesc(project.description);
      setEditEnv(project.environment);
    }
  }, [project?.name, project?.description, project?.environment]);

  const services: ServiceItem[] = [
    ...apps.map((a): ServiceItem => ({ type: "app", data: a })),
    ...databases.map((d): ServiceItem => ({ type: "database", data: d })),
    ...pages.map((p): ServiceItem => ({ type: "page", data: p })),
    ...workers.map((w): ServiceItem => ({ type: "worker", data: w })),
  ];

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (serviceType === "app") {
      if (!appForm.name.trim()) {
        const { toast } = await import("sonner");
        toast.error("Name is required");
        return;
      }
      if (appForm.source_type === "git" && !appForm.git_repo.trim()) {
        const { toast } = await import("sonner");
        toast.error("Please select a repository");
        return;
      }
      if (appForm.source_type === "image" && !appForm.docker_image.trim()) {
        const { toast } = await import("sonner");
        toast.error("Docker image is required");
        return;
      }
      const { registry_id, ...rest } = appForm;
      await createApp.mutateAsync(registry_id ? { ...rest, registry_id } : rest);
      setAppForm({
        name: "",
        is_critical: false,
        source_type: "image",
        docker_image: "",
        git_repo: "",
        git_branch: "main",
        registry_id: "",
      });
    } else if (serviceType === "database") {
      await createDatabase.mutateAsync(dbForm);
      setDbForm({
        name: "",
        database_name: "",
        engine: "postgres",
        version: "",
        storage_size: "1Gi",
      });
    } else if (serviceType === "worker") {
      if (!workerForm.name.trim()) {
        const { toast } = await import("sonner");
        toast.error("Name is required");
        return;
      }
      if (!workerForm.git_repo.trim()) {
        const { toast } = await import("sonner");
        toast.error("Git repository is required");
        return;
      }
      if (!workerForm.cloud_account_id) {
        const { toast } = await import("sonner");
        toast.error("Select a Cloudflare account");
        return;
      }
      const workerName = workerForm.name.trim();
      if (workers.some((w) => w.name === workerName)) {
        const { toast } = await import("sonner");
        toast.error(`A Cloudflare Worker named "${workerName}" already exists in this project`);
        return;
      }
      await createWorker.mutateAsync(workerForm);
      setWorkerForm({
        name: "",
        git_repo: "",
        git_branch: "main",
        git_provider_id: "",
        root_directory: ".",
        wrangler_config: "wrangler.toml",
        package_manager: "auto",
        install_command: "",
        build_command: "",
        deploy_command: "",
        build_env_vars: {},
        cloud_account_id: "",
      });
    } else {
      if (!pageForm.name.trim()) {
        const { toast } = await import("sonner");
        toast.error("Name is required");
        return;
      }
      if (!pageForm.git_repo.trim()) {
        const { toast } = await import("sonner");
        toast.error("Git repository is required");
        return;
      }
      const pageName = pageForm.name.trim();
      if (pages.some((p) => p.name === pageName)) {
        const { toast } = await import("sonner");
        toast.error(`A page named "${pageName}" already exists in this project`);
        return;
      }
      const {
        dns_mode,
        subdomain: _subdomain,
        manage_dns: _manageDns,
        custom_domain: _customDomain,
        dns_account_id: _dnsAccountId,
        dns_zone_id: _dnsZoneId,
        ...pagePayload
      } = pageForm;
      let domainFields: {
        custom_domain?: string;
        manage_dns?: boolean;
        dns_account_id?: string;
        dns_zone_id?: string;
      };
      if (dns_mode === "none") {
        domainFields = { manage_dns: false };
      } else if (dns_mode === "manual") {
        domainFields = {
          custom_domain: pageForm.custom_domain.trim() || undefined,
          manage_dns: false,
        };
      } else {
        if (!pageForm.dns_account_id || !pageForm.dns_zone_id) {
          const { toast } = await import("sonner");
          toast.error(
            dns_mode === "cloudflare"
              ? "Select a Cloudflare DNS account and hosted zone"
              : "Select a DNS account and hosted zone for Route53",
          );
          return;
        }
        domainFields = {
          custom_domain: pageForm.custom_domain.trim() || undefined,
          manage_dns: true,
          dns_account_id: pageForm.dns_account_id,
          dns_zone_id: pageForm.dns_zone_id,
        };
      }
      await createPage.mutateAsync({ ...pagePayload, ...domainFields });
      setPageForm({
        name: "",
        git_repo: "",
        git_branch: "main",
        git_provider_id: "",
        publish_path: ".",
        build_enabled: false,
        package_manager: "auto",
        install_command: "",
        build_command: "",
        output_dir: "",
        root_directory: ".",
        node_version: "",
        build_env_vars: {},
        provider: "aws_cloudfront",
        cloud_account_id: "",
        region: "us-east-1",
        dns_mode: "none",
        custom_domain: "",
        subdomain: "",
        manage_dns: false,
        dns_account_id: "",
        dns_zone_id: "",
      });
    }
    setShowCreate(false);
  }

  if (loading) return <LoadingScreen variant="detail" />;
  if (!project) return null;

  return (
    <div>
      <PageHeader
        title={project.name}
        description={project.description || undefined}
        badges={
          project.namespace ? (
            <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
              {project.namespace}
            </code>
          ) : undefined
        }
        backTo="/projects"
      />
      <Separator className="my-5" />

      <Tabs defaultValue="services">
        <div className="flex items-center justify-between">
          <TabsList>
            <TabsTrigger value="services">Services</TabsTrigger>
            <TabsTrigger value="environment">Environment</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>
          <Button size="sm" onClick={() => setShowCreate(true)}>
            <Plus className="h-4 w-4" /> New Service
          </Button>
        </div>

        <TabsContent value="services" className="mt-4">
          {services.length === 0 ? (
            <EmptyState
              icon={Container}
              message="No services yet."
              actionLabel="New Service"
              onAction={() => setShowCreate(true)}
            />
          ) : (
            <ServiceList services={services} projectId={projectId} onDelete={setDeleteTarget} />
          )}
        </TabsContent>

        <TabsContent value="environment" className="mt-4">
          <ProjectEnvEditor
            key={JSON.stringify(project.env_vars)}
            projectId={projectId}
            envVars={project.env_vars}
          />
        </TabsContent>

        <TabsContent value="settings" className="mt-4 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm font-medium">
                <Settings2 className="h-4 w-4" /> General
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label>Project Name</Label>
                <Input
                  className="border-border bg-background"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input
                  className="border-border bg-background"
                  value={editDesc}
                  onChange={(e) => setEditDesc(e.target.value)}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>Team</Label>
                  <Input
                    className="border-border bg-background"
                    value={project.team?.name ?? "Unassigned"}
                    readOnly
                  />
                </div>
                <div className="space-y-2">
                  <Label>Environment</Label>
                  <Select value={editEnv} onValueChange={(v) => setEditEnv(v as typeof editEnv)}>
                    <SelectTrigger className="border-border bg-background">
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
              <Button
                onClick={() => {
                  updateProject.mutate({
                    name: editName,
                    description: editDesc,
                    environment: editEnv,
                  });
                }}
                disabled={updateProject.isPending}
              >
                {updateProject.isPending ? "Saving..." : "Save Changes"}
              </Button>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium">Service Account</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="font-mono text-sm">
                {project.service_account || (
                  <span className="text-muted-foreground">Auto-created on deploy</span>
                )}
              </p>
            </CardContent>
          </Card>
          <DangerZone
            description="Permanently delete this project and all services."
            buttonLabel="Delete Project"
            onDelete={() => setShowDeleteProject(true)}
          />
        </TabsContent>
      </Tabs>

      <CreateServiceDialog
        open={showCreate}
        onOpenChange={setShowCreate}
        serviceType={serviceType}
        onServiceTypeChange={setServiceType}
        appForm={appForm}
        onAppFormChange={setAppForm}
        dbForm={dbForm}
        onDbFormChange={setDbForm}
        pageForm={pageForm}
        onPageFormChange={setPageForm}
        workerForm={workerForm}
        onWorkerFormChange={setWorkerForm}
        creating={
          createApp.isPending ||
          createDatabase.isPending ||
          createPage.isPending ||
          createWorker.isPending
        }
        onSubmit={handleCreate}
      />

      {deleteTarget && (
        <DeleteServiceDialog service={deleteTarget} onClose={() => setDeleteTarget(null)} />
      )}

      <DeleteProjectDialog
        open={showDeleteProject}
        onOpenChange={setShowDeleteProject}
        project={project}
        apps={apps}
        databases={databases}
        loading={deleteProject.isPending}
        onConfirm={() =>
          deleteProject.mutate(undefined, {
            onSuccess: () => navigate({ to: "/projects" }),
          })
        }
      />
    </div>
  );
}
