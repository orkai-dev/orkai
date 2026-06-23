import { Link } from "@tanstack/react-router";
import {
  Box,
  ChevronLeft,
  ChevronRight,
  Cloud,
  Database,
  FolderGit2,
  Globe,
  Hand,
  Info,
  Network,
  Zap,
} from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RepoCombobox } from "@/components/ui/repo-combobox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useDatabaseVersions } from "@/features/databases";
import { type GitRepo, useDnsZones, useGitRepos, useResources } from "@/features/resources";
import type { SharedResource } from "@/features/resources/types";
import { ENGINE_LABELS } from "@/lib/constants";

export type ServiceType = "app" | "database" | "page" | "worker";

export type WorkerFormState = {
  name: string;
  git_repo: string;
  git_branch: string;
  git_provider_id: string;
  root_directory: string;
  wrangler_config: string;
  package_manager: "auto" | "npm" | "pnpm";
  install_command: string;
  build_command: string;
  deploy_command: string;
  build_env_vars: Record<string, string>;
  cloud_account_id: string;
};

export type PageFormState = {
  name: string;
  git_repo: string;
  git_branch: string;
  git_provider_id: string;
  publish_path: string;
  build_enabled: boolean;
  package_manager: "auto" | "npm" | "pnpm";
  install_command: string;
  build_command: string;
  output_dir: string;
  root_directory: string;
  node_version: string;
  build_env_vars: Record<string, string>;
  provider: "aws_cloudfront" | "cloudflare_pages";
  cloud_account_id: string;
  region: string;
  dns_mode: "none" | "manual" | "route53" | "cloudflare";
  custom_domain: string;
  subdomain: string;
  manage_dns: boolean;
  dns_account_id: string;
  dns_zone_id: string;
};

function composeCustomDomain(zoneName: string, subdomain: string): string {
  const zone = zoneName.replace(/\.$/, "");
  const sub = subdomain.trim();
  return sub ? `${sub}.${zone}` : zone;
}

function FormSection({
  title,
  description,
  icon: Icon,
  children,
}: {
  title: string;
  description?: string;
  icon?: React.ComponentType<{ className?: string }>;
  children: React.ReactNode;
}) {
  return (
    <section className="space-y-3">
      <div className="flex items-start gap-2.5">
        {Icon && (
          <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
            <Icon className="h-4 w-4" />
          </span>
        )}
        <div>
          <h3 className="text-sm font-medium">{title}</h3>
          {description && <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>}
        </div>
      </div>
      <div className="space-y-3">{children}</div>
    </section>
  );
}

/** Label + optional hint, rendered consistently above a control. */
function Field({
  label,
  hint,
  optional,
  htmlFor,
  children,
}: {
  label: string;
  hint?: React.ReactNode;
  optional?: boolean;
  htmlFor?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={htmlFor}>
        {label}
        {optional && <span className="ml-1 font-normal text-muted-foreground">(optional)</span>}
      </Label>
      {children}
      {hint && <p className="text-xs leading-relaxed text-muted-foreground">{hint}</p>}
    </div>
  );
}

/** Subtle informational callout used to explain a concept inline. */
function InfoCallout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex gap-2.5 rounded-lg border border-border bg-muted/40 p-3">
      <Info className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
      <p className="text-xs leading-relaxed text-muted-foreground">{children}</p>
    </div>
  );
}

const SERVICE_TYPES = [
  { id: "app" as const, label: "Application", icon: Box },
  { id: "database" as const, label: "Database", icon: Database },
  { id: "page" as const, label: "Page", icon: Globe },
  { id: "worker" as const, label: "Cloudflare Worker", icon: Zap },
];

function ServiceTypePicker({
  value,
  onChange,
}: {
  value: ServiceType;
  onChange: (v: ServiceType) => void;
}) {
  return (
    <div className="inline-flex rounded-lg border bg-muted/30 p-1">
      {SERVICE_TYPES.map(({ id, label, icon: Icon }) => (
        <button
          key={id}
          type="button"
          onClick={() => onChange(id)}
          className={`inline-flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors ${
            value === id
              ? "bg-background font-medium text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          <Icon className="h-4 w-4" />
          {label}
        </button>
      ))}
    </div>
  );
}

function PageStepIndicator({ step }: { step: 1 | 2 | 3 | 4 }) {
  const steps = [
    { n: 1, label: "Overall" },
    { n: 2, label: "Source" },
    { n: 3, label: "Hosting" },
    { n: 4, label: "DNS" },
  ] as const;
  return (
    <div className="mt-4 flex items-center gap-3">
      {steps.map(({ n, label }, i) => (
        <div key={n} className="flex items-center gap-3">
          {i > 0 && <div className="h-px w-8 bg-border" />}
          <div className="flex items-center gap-2">
            <span
              className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium ${
                step === n
                  ? "bg-primary text-primary-foreground"
                  : step > n
                    ? "bg-primary/15 text-primary"
                    : "bg-muted text-muted-foreground"
              }`}
            >
              {n}
            </span>
            <span className={`text-sm ${step === n ? "font-medium" : "text-muted-foreground"}`}>
              {label}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}

function WorkerStepIndicator({ step }: { step: 1 | 2 }) {
  const steps = [
    { n: 1, label: "Overall" },
    { n: 2, label: "Source & account" },
  ] as const;
  return (
    <div className="mt-4 flex items-center gap-3">
      {steps.map(({ n, label }, i) => (
        <div key={n} className="flex items-center gap-3">
          {i > 0 && <div className="h-px w-8 bg-border" />}
          <div className="flex items-center gap-2">
            <span
              className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium ${
                step === n
                  ? "bg-primary text-primary-foreground"
                  : step > n
                    ? "bg-primary/15 text-primary"
                    : "bg-muted text-muted-foreground"
              }`}
            >
              {n}
            </span>
            <span className={`text-sm ${step === n ? "font-medium" : "text-muted-foreground"}`}>
              {label}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}

export function CreateServiceDialog({
  open,
  onOpenChange,
  serviceType,
  onServiceTypeChange,
  appForm,
  onAppFormChange,
  dbForm,
  onDbFormChange,
  pageForm,
  onPageFormChange,
  workerForm,
  onWorkerFormChange,
  creating,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  serviceType: ServiceType;
  onServiceTypeChange: (v: ServiceType) => void;
  appForm: {
    name: string;
    is_critical: boolean;
    source_type: string;
    docker_image: string;
    git_repo: string;
    git_branch: string;
    registry_id: string;
  };
  onAppFormChange: (v: typeof appForm) => void;
  dbForm: {
    name: string;
    database_name: string;
    engine: string;
    version: string;
    storage_size: string;
  };
  onDbFormChange: (v: typeof dbForm) => void;
  pageForm: PageFormState;
  onPageFormChange: (v: PageFormState) => void;
  workerForm: WorkerFormState;
  onWorkerFormChange: (v: WorkerFormState) => void;
  creating: boolean;
  onSubmit: (e: React.FormEvent) => void;
}) {
  const [pageStep, setPageStep] = useState<1 | 2 | 3 | 4>(1);
  const [workerStep, setWorkerStep] = useState<1 | 2>(1);

  useEffect(() => {
    if (open) {
      setPageStep(1);
      setWorkerStep(1);
    }
  }, [open]);

  function handleServiceTypeChange(type: ServiceType) {
    setPageStep(1);
    setWorkerStep(1);
    onServiceTypeChange(type);
  }

  const step1Valid = pageForm.name.trim() !== "";
  const step2Valid = pageForm.git_repo.trim() !== "";
  const step3Valid = pageForm.cloud_account_id !== "";

  const workerStep1Valid = workerForm.name.trim() !== "";
  const workerStep2Valid = workerForm.git_repo.trim() !== "" && workerForm.cloud_account_id !== "";

  function handleFormSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (serviceType === "page") {
      if (pageStep === 1) {
        if (step1Valid) setPageStep(2);
        return;
      }
      if (pageStep === 2) {
        if (step2Valid) setPageStep(3);
        return;
      }
      if (pageStep === 3) {
        if (step3Valid) setPageStep(4);
        return;
      }
    }
    if (serviceType === "worker") {
      if (workerStep === 1) {
        if (workerStep1Valid) setWorkerStep(2);
        return;
      }
    }
    onSubmit(e);
  }

  const workerHeader =
    serviceType === "worker"
      ? {
          eyebrow: `Cloudflare Worker · Step ${workerStep} of 2`,
          title:
            workerStep === 1
              ? "Tell us about your Cloudflare Worker"
              : "Source & Cloudflare account",
          description:
            workerStep === 1
              ? "Give your Cloudflare Worker a name to get started."
              : "Pick the Git repo with your wrangler project and the Cloudflare account to deploy with.",
        }
      : null;

  const pageHeader =
    serviceType === "page"
      ? {
          eyebrow: `Static page · Step ${pageStep} of 4`,
          title:
            pageStep === 1
              ? "Tell us about your page"
              : pageStep === 2
                ? "Where do the files come from?"
                : pageStep === 3
                  ? "Where should we host it?"
                  : "How will people reach it?",
          description:
            pageStep === 1
              ? "Give your static page a name to get started."
              : pageStep === 2
                ? "Pick the Git repo with your built site and define the folder to publish."
                : pageStep === 3
                  ? pageForm.provider === "cloudflare_pages"
                    ? "Select the Cloudflare account to host your page."
                    : "Select the AWS account and region to host your page."
                  : pageForm.provider === "cloudflare_pages"
                    ? "Choose how visitors access the page — the free pages.dev URL, or your own domain."
                    : "Choose how visitors access the page — the free CloudFront URL, or your own domain.",
        }
      : null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl gap-0 overflow-hidden p-0">
        <form onSubmit={handleFormSubmit}>
          <div className="border-b bg-muted/40 px-6 py-5 pr-14">
            <p className="text-label-caps text-primary">
              {workerHeader?.eyebrow ?? pageHeader?.eyebrow ?? "New service"}
            </p>
            <DialogTitle className="mt-2">
              {workerHeader?.title ?? pageHeader?.title ?? "Deploy to your cluster"}
            </DialogTitle>
            <p className="mt-1 text-sm text-muted-foreground">
              {workerHeader?.description ??
                pageHeader?.description ??
                "Choose a service type and configure how it runs in your project."}
            </p>
          </div>

          <div className="max-h-[min(70vh,640px)] space-y-6 overflow-y-auto px-6 py-5">
            {!(serviceType === "page" && pageStep > 1) &&
              !(serviceType === "worker" && workerStep > 1) && (
                <ServiceTypePicker value={serviceType} onChange={handleServiceTypeChange} />
              )}

            {serviceType === "page" && <PageStepIndicator step={pageStep} />}
            {serviceType === "worker" && <WorkerStepIndicator step={workerStep} />}

            {serviceType === "app" ? (
              <AppFormFields form={appForm} onChange={onAppFormChange} />
            ) : serviceType === "page" ? (
              <PageFormFields form={pageForm} onChange={onPageFormChange} step={pageStep} />
            ) : serviceType === "worker" ? (
              <WorkerFormFields form={workerForm} onChange={onWorkerFormChange} step={workerStep} />
            ) : (
              <DbFormFields form={dbForm} onChange={onDbFormChange} />
            )}
          </div>

          <div className="flex flex-col-reverse gap-2 border-t px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              {serviceType === "page" && pageStep > 1 ? (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setPageStep((prev) => (prev - 1) as 1 | 2 | 3 | 4)}
                >
                  <ChevronLeft className="h-4 w-4" />
                  Back
                </Button>
              ) : serviceType === "worker" && workerStep > 1 ? (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setWorkerStep((prev) => (prev - 1) as 1 | 2)}
                >
                  <ChevronLeft className="h-4 w-4" />
                  Back
                </Button>
              ) : (
                <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                  Cancel
                </Button>
              )}
            </div>
            <div className="flex flex-col-reverse gap-2 sm:flex-row">
              {/*
                A single submit button drives both "advance step" and "create".
                Step advancement happens in the form's onSubmit (handleFormSubmit),
                never in a separate onClick. Splitting them into two buttons that
                swap at the same JSX position let React reuse the DOM node and
                mutate its type from "button" to "submit" mid-click, so the click's
                default action submitted the form and created the page before the
                DNS step could be filled in.
              */}
              <Button
                type="submit"
                disabled={
                  creating ||
                  (serviceType === "page" &&
                    ((pageStep === 1 && !step1Valid) ||
                      (pageStep === 2 && !step2Valid) ||
                      (pageStep === 3 && !step3Valid))) ||
                  (serviceType === "worker" && workerStep === 1 && !workerStep1Valid) ||
                  (serviceType === "worker" && workerStep === 2 && !workerStep2Valid)
                }
              >
                {(serviceType === "page" && pageStep < 4) ||
                (serviceType === "worker" && workerStep < 2) ? (
                  <>
                    Continue
                    <ChevronRight className="h-4 w-4" />
                  </>
                ) : creating ? (
                  "Creating..."
                ) : (
                  "Create"
                )}
              </Button>
            </div>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function ImageSourceFields({
  form,
  onChange,
  registries,
}: {
  form: { docker_image: string; [key: string]: string };
  onChange: (v: typeof form) => void;
  registries: SharedResource[];
}) {
  const update = (field: string, value: string) => onChange({ ...form, [field]: value });
  const [selectedRegistry, setSelectedRegistry] = useState("dockerhub");

  const handleRegistryChange = (registryId: string) => {
    setSelectedRegistry(registryId);
    if (registryId === "dockerhub") {
      onChange({ ...form, registry_id: "", docker_image: "" });
      return;
    }
    const reg = registries.find((r) => r.id === registryId);
    if (reg?.provider === "ecr") {
      onChange({ ...form, registry_id: registryId, docker_image: "" });
      return;
    }
    const url = (reg?.config as Record<string, string>)?.url || "";
    const host = url.replace(/^https?:\/\//, "").replace(/\/$/, "");
    onChange({ ...form, registry_id: registryId, docker_image: host ? `${host}/` : "" });
  };

  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <Label>Registry</Label>
        <Select value={selectedRegistry} onValueChange={handleRegistryChange}>
          <SelectTrigger className="border-border bg-background">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="dockerhub">Docker Hub (public)</SelectItem>
            {registries.map((r) => (
              <SelectItem key={r.id} value={r.id}>
                {r.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {registries.length === 0 && (
          <p className="text-xs text-muted-foreground">
            Add custom registries (GHCR, ECR, etc.) in{" "}
            <Link to="/admin/resources" className="text-primary underline underline-offset-4">
              Resources
            </Link>
          </p>
        )}
      </div>
      <div className="space-y-2">
        <Label>Image</Label>
        <Input
          className="border-border bg-background"
          value={form.docker_image}
          onChange={(e) => update("docker_image", e.target.value)}
          placeholder={selectedRegistry === "dockerhub" ? "nginx:latest" : "org/image:tag"}
          required
        />
        <p className="text-xs text-muted-foreground">
          Full image reference including tag, e.g. <code className="text-xs">nginx:latest</code> or{" "}
          <code className="text-xs">ghcr.io/user/app:v1.0</code>
        </p>
      </div>
    </div>
  );
}

export function GitSourceFields({
  form,
  onChange,
  gitProviders,
  showBuildType = true,
}: {
  form: { git_repo: string; git_branch: string; [key: string]: string };
  onChange: (v: typeof form) => void;
  gitProviders: { id: string; name: string; provider: string }[];
  /** Apps only — Pages sync pre-built static files, no container build. */
  showBuildType?: boolean;
}) {
  const [selectedProviderId, setSelectedProviderId] = useState("");
  const [selectedRepoName, setSelectedRepoName] = useState("");
  const { data: repos, isLoading: loadingRepos } = useGitRepos(selectedProviderId);

  function handleProviderChange(providerId: string) {
    setSelectedProviderId(providerId);
    setSelectedRepoName("");
    onChange({ ...form, git_repo: "", git_branch: "main", git_provider_id: providerId });
  }

  function handleRepoSelect(repo: GitRepo) {
    setSelectedRepoName(repo.full_name);
    onChange({
      ...form,
      git_repo: repo.clone_url,
      git_branch: repo.default_branch || form.git_branch,
      git_provider_id: selectedProviderId,
    });
  }

  function update(field: string, value: string) {
    onChange({ ...form, [field]: value });
  }

  return (
    <div className="space-y-3">
      {gitProviders.length > 0 ? (
        <div className="space-y-2">
          <Label>Git Provider</Label>
          <Select value={selectedProviderId} onValueChange={handleProviderChange}>
            <SelectTrigger className="border-border bg-background">
              <SelectValue placeholder="Select a provider..." />
            </SelectTrigger>
            <SelectContent>
              {gitProviders.map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      ) : (
        <div className="rounded-lg border border-dashed p-4 text-center">
          <p className="text-sm text-muted-foreground">No Git providers configured.</p>
          <a
            href="/admin/resources"
            className="mt-1 inline-block text-sm text-primary hover:underline"
          >
            Connect GitHub, GitLab, or Gitea →
          </a>
        </div>
      )}

      {selectedProviderId ? (
        <div className="space-y-2">
          <Label>Repository</Label>
          {loadingRepos ? (
            <div className="flex h-9 items-center rounded-md border px-3 text-sm text-muted-foreground">
              Loading repositories...
            </div>
          ) : repos && repos.length > 0 ? (
            <>
              <RepoCombobox repos={repos} value={selectedRepoName} onSelect={handleRepoSelect} />
              <p className="text-xs text-muted-foreground">
                {repos.length} repositor{repos.length === 1 ? "y" : "ies"} available
              </p>
            </>
          ) : (
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground">
                No repositories found or failed to load.
              </p>
              <Input
                className="border-border bg-background"
                value={form.git_repo}
                onChange={(e) => update("git_repo", e.target.value)}
                placeholder="https://github.com/user/repo"
                required
              />
            </div>
          )}
        </div>
      ) : gitProviders.length > 0 ? null : (
        <div className="space-y-2">
          <Label>Repository URL</Label>
          <Input
            className="border-border bg-background"
            value={form.git_repo}
            onChange={(e) => update("git_repo", e.target.value)}
            placeholder="https://github.com/user/repo"
            required
          />
        </div>
      )}

      {form.git_repo && (
        <div className={showBuildType ? "grid grid-cols-2 gap-4" : "space-y-2"}>
          <div className="space-y-2">
            <Label>Branch</Label>
            <Select
              value={form.git_branch || "main"}
              onValueChange={(v) => update("git_branch", v)}
            >
              <SelectTrigger className="border-border bg-background">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="main">main</SelectItem>
                <SelectItem value="master">master</SelectItem>
                <SelectItem value="develop">develop</SelectItem>
                <SelectItem value="staging">staging</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {showBuildType && (
            <div className="space-y-2">
              <Label>Build Type</Label>
              <Select value="dockerfile" onValueChange={() => {}}>
                <SelectTrigger className="border-border bg-background">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="dockerfile">Dockerfile</SelectItem>
                  <SelectItem value="nixpacks">Nixpacks (auto)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export function AppFormFields({
  form,
  onChange,
}: {
  form: {
    name: string;
    is_critical: boolean;
    source_type: string;
    docker_image: string;
    git_repo: string;
    git_branch: string;
    registry_id: string;
  };
  onChange: (v: typeof form) => void;
}) {
  const update = (field: string, value: string) => onChange({ ...form, [field]: value });

  const { data: gitProviders } = useResources("git_provider");
  const { data: registries } = useResources("registry");

  return (
    <>
      <div className="space-y-2">
        <Label>Name</Label>
        <Input
          className="border-border bg-background"
          value={form.name}
          onChange={(e) => update("name", e.target.value)}
          placeholder="my-app"
          required
        />
      </div>
      <label className="flex items-center gap-2 text-sm font-medium">
        <input
          type="checkbox"
          checked={form.is_critical}
          onChange={(e) => onChange({ ...form, is_critical: e.target.checked })}
          className="rounded border-border bg-background"
        />
        Mark as critical
      </label>
      <div className="space-y-2">
        <Label>Source</Label>
        <Select value={form.source_type} onValueChange={(v) => update("source_type", v)}>
          <SelectTrigger className="border-border bg-background">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="image">Docker Image</SelectItem>
            <SelectItem value="git">Git Repository</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {form.source_type === "image" ? (
        <ImageSourceFields form={form} onChange={onChange} registries={registries ?? []} />
      ) : (
        <GitSourceFields form={form} onChange={onChange} gitProviders={gitProviders ?? []} />
      )}
    </>
  );
}

export function DbFormFields({
  form,
  onChange,
}: {
  form: {
    name: string;
    database_name: string;
    engine: string;
    version: string;
    storage_size: string;
  };
  onChange: (v: typeof form) => void;
}) {
  const update = (field: string, value: string) => onChange({ ...form, [field]: value });
  const { data: rawVersions, isLoading: versionsLoading } = useDatabaseVersions(form.engine);
  const versions = rawVersions ?? [];

  if (versions.length > 0 && !versions.some((v) => v.tag === form.version)) {
    const recommended = versions.find((v) => v.is_recommended);
    const fallback = recommended?.tag ?? versions[0]?.tag ?? "";
    if (fallback && fallback !== form.version) {
      onChange({ ...form, version: fallback });
    }
  }

  return (
    <>
      <div className="space-y-2">
        <Label>Name</Label>
        <Input
          className="border-border bg-background"
          value={form.name}
          onChange={(e) => update("name", e.target.value)}
          placeholder="my-database"
          required
        />
      </div>
      <div className="space-y-2">
        <Label>
          Database Name <span className="text-muted-foreground font-normal">(optional)</span>
        </Label>
        <Input
          className="border-border bg-background"
          value={form.database_name}
          onChange={(e) => update("database_name", e.target.value)}
          placeholder={form.name || "defaults to service name"}
        />
      </div>
      <div className="space-y-2">
        <Label>Engine</Label>
        <Select
          value={form.engine}
          onValueChange={(v) => onChange({ ...form, engine: v, version: "" })}
        >
          <SelectTrigger className="border-border bg-background">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(ENGINE_LABELS).map(([key, label]) => (
              <SelectItem key={key} value={key}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Version</Label>
          <Select
            value={form.version}
            onValueChange={(v) => update("version", v)}
            disabled={versionsLoading || versions.length === 0}
          >
            <SelectTrigger className="border-border bg-background">
              <SelectValue placeholder={versionsLoading ? "Loading..." : "Select version"} />
            </SelectTrigger>
            <SelectContent>
              {versions.map((v) => (
                <SelectItem key={v.tag} value={v.tag}>
                  {v.label}
                  {v.is_recommended && " ⭐"}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Storage</Label>
          <Select value={form.storage_size} onValueChange={(v) => update("storage_size", v)}>
            <SelectTrigger className="border-border bg-background">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {["1Gi", "2Gi", "5Gi", "10Gi", "20Gi", "50Gi", "100Gi"].map((size) => (
                <SelectItem key={size} value={size}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </>
  );
}

function BuildEnvEditor({
  value,
  onChange,
}: {
  value: Record<string, string>;
  onChange: (next: Record<string, string>) => void;
}) {
  const [rows, setRows] = useState<{ key: string; value: string }[]>(() =>
    Object.entries(value ?? {}).map(([key, val]) => ({ key, value: val })),
  );

  function commit(next: { key: string; value: string }[]) {
    setRows(next);
    const record: Record<string, string> = {};
    for (const r of next) {
      const k = r.key.trim();
      if (k) record[k] = r.value;
    }
    onChange(record);
  }

  return (
    <div className="space-y-2">
      {rows.map((row, i) => (
        <div key={i} className="flex gap-2">
          <Input
            className="border-border bg-background font-mono text-xs"
            value={row.key}
            onChange={(e) =>
              commit(rows.map((r, j) => (j === i ? { ...r, key: e.target.value } : r)))
            }
            placeholder="KEY"
          />
          <Input
            className="border-border bg-background font-mono text-xs"
            value={row.value}
            onChange={(e) =>
              commit(rows.map((r, j) => (j === i ? { ...r, value: e.target.value } : r)))
            }
            placeholder="value"
          />
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => commit(rows.filter((_, j) => j !== i))}
          >
            Remove
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => commit([...rows, { key: "", value: "" }])}
      >
        Add Variable
      </Button>
    </div>
  );
}

export function WorkerFormFields({
  form,
  onChange,
  step,
}: {
  form: WorkerFormState;
  onChange: (v: WorkerFormState) => void;
  step: 1 | 2;
}) {
  const update = (field: string, value: string) => onChange({ ...form, [field]: value });

  const { data: cloudAccounts } = useResources("cloud_account");
  const { data: gitProviders } = useResources("git_provider");
  const cfAccounts = (cloudAccounts ?? []).filter((a) => a.provider === "cloudflare");
  const providers = gitProviders ?? [];

  if (step === 1) {
    return (
      <FormSection title="Basics" icon={Zap}>
        <Field
          label="Name"
          hint="A label for this Cloudflare Worker inside the project. The deployed script name comes from your wrangler.toml."
        >
          <Input
            className="border-border bg-background"
            value={form.name}
            onChange={(e) => update("name", e.target.value)}
            placeholder="my-worker"
            required
          />
        </Field>
      </FormSection>
    );
  }

  return (
    <FormSection
      title="Source & account"
      icon={FolderGit2}
      description="The repository with your wrangler.toml and the Cloudflare account used to deploy."
    >
      <GitSourceFields
        form={form}
        onChange={onChange}
        gitProviders={providers}
        showBuildType={false}
      />

      <div className="grid gap-4 sm:grid-cols-2">
        <Field
          label="Root directory"
          hint="Subfolder that contains your wrangler project (for monorepos). Defaults to the repo root."
        >
          <Input
            className="border-border bg-background"
            value={form.root_directory}
            onChange={(e) => update("root_directory", e.target.value)}
            placeholder="."
          />
        </Field>
        <Field
          label="Wrangler config"
          optional
          hint={
            <>
              Auto-detected (<code className="text-[11px]">wrangler.jsonc</code> /{" "}
              <code className="text-[11px]">.json</code> /{" "}
              <code className="text-[11px]">.toml</code>
              ). Set a path only to override.
            </>
          }
        >
          <Input
            className="border-border bg-background"
            value={form.wrangler_config}
            onChange={(e) => update("wrangler_config", e.target.value)}
            placeholder="auto-detect"
          />
        </Field>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field
          label="Package manager"
          hint="Auto-detects from the lockfile. Used to install deps before wrangler deploy."
        >
          <Select value={form.package_manager} onValueChange={(v) => update("package_manager", v)}>
            <SelectTrigger className="border-border bg-background">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="auto">Auto-detect</SelectItem>
              <SelectItem value="npm">npm</SelectItem>
              <SelectItem value="pnpm">pnpm</SelectItem>
            </SelectContent>
          </Select>
        </Field>
        <Field
          label="Install command"
          optional
          hint="Leave empty to use the package manager default (pnpm install / npm ci)."
        >
          <Input
            className="border-border bg-background"
            value={form.install_command}
            onChange={(e) => update("install_command", e.target.value)}
            placeholder="pnpm install --frozen-lockfile"
          />
        </Field>
        <Field
          label="Build command"
          optional
          hint="Runs after install, before deploy. Leave empty to skip."
        >
          <Input
            className="border-border bg-background"
            value={form.build_command}
            onChange={(e) => update("build_command", e.target.value)}
            placeholder="pnpm exec opennextjs-cloudflare build"
          />
        </Field>
        <Field
          label="Deploy command"
          optional
          hint="Leave empty to use the default wrangler deploy."
        >
          <Input
            className="border-border bg-background"
            value={form.deploy_command}
            onChange={(e) => update("deploy_command", e.target.value)}
            placeholder="pnpm deploy"
          />
        </Field>
      </div>

      <Field
        label="Cloudflare account"
        hint="Connected accounts come from Resources. Requires a token with Workers Scripts:Edit and Account:Read."
      >
        {cfAccounts.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No Cloudflare accounts configured.{" "}
            <Link
              to="/admin/resources"
              className="text-primary underline underline-offset-4 hover:text-primary/80"
            >
              Add one in Resources.
            </Link>
          </p>
        ) : (
          <Select
            value={form.cloud_account_id}
            onValueChange={(v) => update("cloud_account_id", v)}
          >
            <SelectTrigger className="border-border bg-background">
              <SelectValue placeholder="Select Cloudflare account" />
            </SelectTrigger>
            <SelectContent>
              {cfAccounts.map((a: SharedResource) => (
                <SelectItem key={a.id} value={a.id}>
                  {a.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </Field>

      <InfoCallout>
        Cloudflare Worker routes and custom domains are read from your{" "}
        <code className="text-[11px]">wrangler.toml</code>. After the first deploy the Cloudflare
        Worker is live at its <code className="text-[11px]">*.workers.dev</code> URL.
      </InfoCallout>
    </FormSection>
  );
}

export function PageFormFields({
  form,
  onChange,
  step,
}: {
  form: PageFormState;
  onChange: (v: PageFormState) => void;
  step: 1 | 2 | 3 | 4;
}) {
  const update = (field: string, value: string | boolean) => onChange({ ...form, [field]: value });

  const { data: cloudAccounts } = useResources("cloud_account");
  const { data: gitProviders } = useResources("git_provider");
  const isCloudflare = form.provider === "cloudflare_pages";
  const hostingAccounts = (cloudAccounts ?? []).filter((a) =>
    isCloudflare ? a.provider === "cloudflare" : a.provider === "aws",
  );
  const dnsAccounts = (cloudAccounts ?? []).filter((a) =>
    form.dns_mode === "cloudflare" ? a.provider === "cloudflare" : a.provider === "aws",
  );
  const providers = gitProviders ?? [];
  const { data: dnsZones } = useDnsZones(
    form.dns_mode === "route53" || form.dns_mode === "cloudflare" ? form.dns_account_id : "",
  );

  function setProvider(next: PageFormState["provider"]) {
    onChange({
      ...form,
      provider: next,
      cloud_account_id: "",
      region: next === "aws_cloudfront" ? form.region || "us-east-1" : "",
      dns_mode: "none",
      custom_domain: "",
      subdomain: "",
      manage_dns: false,
      dns_account_id: "",
      dns_zone_id: "",
    });
  }

  function setDnsMode(mode: PageFormState["dns_mode"]) {
    if (mode === "none") {
      onChange({
        ...form,
        dns_mode: "none",
        custom_domain: "",
        subdomain: "",
        manage_dns: false,
        dns_account_id: "",
        dns_zone_id: "",
      });
      return;
    }
    if (mode === "manual") {
      onChange({
        ...form,
        dns_mode: "manual",
        subdomain: "",
        manage_dns: false,
        dns_account_id: "",
        dns_zone_id: "",
      });
      return;
    }
    if (mode === "route53") {
      onChange({
        ...form,
        dns_mode: "route53",
        custom_domain: "",
        subdomain: "",
        manage_dns: true,
        dns_account_id: "",
        dns_zone_id: "",
      });
      return;
    }
    onChange({
      ...form,
      dns_mode: "cloudflare",
      custom_domain: "",
      subdomain: "",
      manage_dns: true,
      dns_account_id: "",
      dns_zone_id: "",
    });
  }

  function handleZoneChange(zoneId: string) {
    const zone = (dnsZones ?? []).find((z) => z.id === zoneId);
    if (!zone) {
      onChange({ ...form, dns_zone_id: zoneId, custom_domain: "" });
      return;
    }
    onChange({
      ...form,
      dns_zone_id: zoneId,
      custom_domain: composeCustomDomain(zone.name, form.subdomain),
    });
  }

  function handleSubdomainChange(subdomain: string) {
    const zone = (dnsZones ?? []).find((z) => z.id === form.dns_zone_id);
    onChange({
      ...form,
      subdomain,
      custom_domain: zone ? composeCustomDomain(zone.name, subdomain) : "",
    });
  }

  if (step === 1) {
    return (
      <FormSection title="Basics" icon={Globe}>
        <Field
          label="Name"
          hint={
            isCloudflare
              ? "A label for this page inside the project. Also used to name the Cloudflare Pages project."
              : "A label for this page inside the project. Also used to name the S3 bucket and CloudFront distribution."
          }
        >
          <Input
            className="border-border bg-background"
            value={form.name}
            onChange={(e) => update("name", e.target.value)}
            placeholder="marketing-site"
            required
          />
        </Field>
      </FormSection>
    );
  }

  if (step === 2) {
    return (
      <FormSection
        title="Source"
        icon={FolderGit2}
        description="The repository Orkai pulls your built static files from."
      >
        <GitSourceFields
          form={form}
          onChange={onChange}
          gitProviders={providers}
          showBuildType={false}
        />

        <label className="flex items-center gap-2 text-sm font-medium">
          <input
            type="checkbox"
            checked={form.build_enabled}
            onChange={(e) => update("build_enabled", e.target.checked)}
            className="rounded border-border bg-background"
          />
          Build from source (npm / pnpm)
        </label>

        {form.build_enabled ? (
          <div className="space-y-4">
            <p className="text-xs text-muted-foreground">
              Orkai runs your install and build commands in an isolated build container, then
              deploys the output folder. The package manager is auto-detected from your lockfile.
            </p>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field
                label="Package manager"
                hint="Auto-detects from the lockfile (pnpm-lock.yaml / package-lock.json)."
              >
                <Select
                  value={form.package_manager}
                  onValueChange={(v) => update("package_manager", v)}
                >
                  <SelectTrigger className="border-border bg-background">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="auto">Auto-detect</SelectItem>
                    <SelectItem value="npm">npm</SelectItem>
                    <SelectItem value="pnpm">pnpm</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field
                label="Output directory"
                hint={
                  <>
                    Folder produced by the build, e.g. <code className="text-[11px]">dist</code> or{" "}
                    <code className="text-[11px]">build</code>.
                  </>
                }
              >
                <Input
                  className="border-border bg-background"
                  value={form.output_dir}
                  onChange={(e) => update("output_dir", e.target.value)}
                  placeholder="dist"
                />
              </Field>
            </div>
            <Field
              label="Install command"
              hint="Leave empty to use the package manager default (pnpm install / npm ci)."
            >
              <Input
                className="border-border bg-background"
                value={form.install_command}
                onChange={(e) => update("install_command", e.target.value)}
                placeholder="pnpm install --frozen-lockfile"
              />
            </Field>
            <Field
              label="Build command"
              hint="Leave empty to use the package manager default (<pm> run build)."
            >
              <Input
                className="border-border bg-background"
                value={form.build_command}
                onChange={(e) => update("build_command", e.target.value)}
                placeholder="pnpm build"
              />
            </Field>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field
                label="Root directory"
                hint="Subfolder that contains package.json (for monorepos). Defaults to the repo root."
              >
                <Input
                  className="border-border bg-background"
                  value={form.root_directory}
                  onChange={(e) => update("root_directory", e.target.value)}
                  placeholder="."
                />
              </Field>
              <Field
                label="Node version"
                hint="Major version of the Node build image (e.g. 22). Leave empty for the default."
              >
                <Input
                  className="border-border bg-background"
                  value={form.node_version}
                  onChange={(e) => update("node_version", e.target.value)}
                  placeholder="22"
                />
              </Field>
            </div>
            <Field
              label="Build environment variables"
              hint="Injected during the build (e.g. VITE_API_URL). Available to your build command, not at runtime."
            >
              <BuildEnvEditor
                value={form.build_env_vars}
                onChange={(next) => onChange({ ...form, build_env_vars: next })}
              />
            </Field>
          </div>
        ) : (
          <Field
            label="Publish folder"
            hint={
              <>
                Folder inside the repo that holds the files to serve — usually your build output
                like <code className="text-[11px]">dist</code>,{" "}
                <code className="text-[11px]">build</code>, or{" "}
                <code className="text-[11px]">public</code>. Use{" "}
                <code className="text-[11px]">.</code> if the files live at the repo root.
              </>
            }
          >
            <Input
              className="border-border bg-background"
              value={form.publish_path}
              onChange={(e) => update("publish_path", e.target.value)}
              placeholder="dist"
            />
          </Field>
        )}
      </FormSection>
    );
  }

  if (step === 3) {
    return (
      <FormSection
        title={isCloudflare ? "Cloudflare hosting" : "AWS hosting"}
        icon={Cloud}
        description={
          isCloudflare
            ? "The Cloudflare account where the Pages project is created."
            : "The account where the S3 bucket and CloudFront distribution are created."
        }
      >
        <Field
          label="Hosting provider"
          hint="Where static files are published. This cannot be changed after the first deploy."
        >
          <Select
            value={form.provider}
            onValueChange={(v) => setProvider(v as PageFormState["provider"])}
          >
            <SelectTrigger className="border-border bg-background">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="aws_cloudfront">AWS CloudFront + S3</SelectItem>
              <SelectItem value="cloudflare_pages">Cloudflare Pages</SelectItem>
            </SelectContent>
          </Select>
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field
            label={isCloudflare ? "Cloudflare account" : "AWS account"}
            hint={
              isCloudflare
                ? "Connected accounts come from Resources. Requires a token with Pages:Edit."
                : "Connected accounts come from Resources. Billing for S3 and CloudFront lands here."
            }
          >
            {hostingAccounts.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                No {isCloudflare ? "Cloudflare" : "AWS"} accounts configured.{" "}
                <Link
                  to="/admin/resources"
                  className="text-primary underline underline-offset-4 hover:text-primary/80"
                >
                  Add one in Resources.
                </Link>
              </p>
            ) : (
              <Select
                value={form.cloud_account_id}
                onValueChange={(v) => update("cloud_account_id", v)}
              >
                <SelectTrigger className="border-border bg-background">
                  <SelectValue
                    placeholder={`Select ${isCloudflare ? "Cloudflare" : "AWS"} account`}
                  />
                </SelectTrigger>
                <SelectContent>
                  {hostingAccounts.map((a: SharedResource) => (
                    <SelectItem key={a.id} value={a.id}>
                      {a.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </Field>
          {!isCloudflare && (
            <Field
              label="Region"
              hint="AWS region for the S3 bucket. CloudFront is global regardless of this choice."
            >
              <Input
                className="border-border bg-background"
                value={form.region}
                onChange={(e) => update("region", e.target.value)}
                placeholder="us-east-1"
              />
            </Field>
          )}
        </div>
      </FormSection>
    );
  }

  const dnsOptions =
    form.provider === "cloudflare_pages"
      ? ([
          {
            mode: "none" as const,
            icon: Globe,
            title: "pages.dev URL",
            description: "Use the free Cloudflare URL",
          },
          {
            mode: "manual" as const,
            icon: Hand,
            title: "Custom domain",
            description: "Add your own DNS record",
          },
          {
            mode: "cloudflare" as const,
            icon: Network,
            title: "Cloudflare DNS",
            description: "Orkai manages DNS for you",
          },
        ] as const)
      : ([
          {
            mode: "none" as const,
            icon: Globe,
            title: "CloudFront URL",
            description: "Use the free URL AWS gives you",
          },
          {
            mode: "manual" as const,
            icon: Hand,
            title: "Custom domain",
            description: "Add your own DNS record",
          },
          {
            mode: "route53" as const,
            icon: Network,
            title: "Route 53",
            description: "Orkai manages DNS for you",
          },
        ] as const);

  return (
    <>
      <InfoCallout>
        {form.provider === "cloudflare_pages" ? (
          <>
            Every page is reachable at its <code className="text-[11px]">*.pages.dev</code> URL out
            of the box. Pick a custom domain only if you want visitors to use your own address.
          </>
        ) : (
          <>
            Every page is reachable at its CloudFront URL out of the box. Pick a custom domain only
            if you want visitors to use your own address — Orkai requests a free HTTPS certificate
            (ACM in <code className="text-[11px]">us-east-1</code>) automatically.
          </>
        )}
      </InfoCallout>

      <div className="grid gap-2 sm:grid-cols-3">
        {dnsOptions.map(({ mode, icon: Icon, title, description }) => {
          const active = form.dns_mode === mode;
          return (
            <button
              key={mode}
              type="button"
              onClick={() => setDnsMode(mode)}
              className={`flex flex-col items-center gap-2 rounded-lg border px-3 py-4 text-center transition-colors ${
                active ? "border-primary bg-primary/5 ring-1 ring-primary/20" : "hover:bg-accent"
              }`}
            >
              <Icon className={`h-5 w-5 ${active ? "text-primary" : "text-muted-foreground"}`} />
              <div>
                <p className="text-sm font-medium">{title}</p>
                <p className="mt-0.5 text-xs leading-snug text-muted-foreground">{description}</p>
              </div>
            </button>
          );
        })}
      </div>

      {form.dns_mode === "none" && (
        <div className="rounded-lg border border-dashed bg-muted/20 p-4 text-sm text-muted-foreground">
          {form.provider === "cloudflare_pages" ? (
            <>
              Nothing else to set up. After the first deploy your page will be live at its{" "}
              <code className="text-[11px]">*.pages.dev</code> URL. You can attach a custom domain
              later from the page&apos;s settings.
            </>
          ) : (
            <>
              Nothing else to set up. After the first deploy your page will be live at its
              CloudFront URL (something like{" "}
              <code className="text-[11px]">d1234abcd.cloudfront.net</code>). You can attach a
              custom domain later from the page&apos;s settings.
            </>
          )}
        </div>
      )}

      {form.dns_mode === "manual" && (
        <div className="space-y-4 rounded-lg border bg-muted/20 p-4">
          <FormSection
            title="Your domain"
            description={
              form.provider === "cloudflare_pages"
                ? "You'll point this domain at Cloudflare Pages yourself using your existing DNS provider."
                : "You'll point this domain at CloudFront yourself using your existing DNS provider."
            }
          >
            <Field
              label="Domain"
              hint="After you create the page, Orkai shows the exact CNAME / certificate-validation records to add at your DNS provider. The page goes live once those records resolve."
            >
              <Input
                className="border-border bg-background"
                value={form.custom_domain}
                onChange={(e) => update("custom_domain", e.target.value)}
                placeholder="app.example.com"
              />
            </Field>
          </FormSection>
        </div>
      )}

      {(form.dns_mode === "route53" || form.dns_mode === "cloudflare") && (
        <div className="space-y-4 rounded-lg border bg-muted/20 p-4">
          <FormSection
            title={form.dns_mode === "cloudflare" ? "Cloudflare DNS zone" : "Route 53 hosted zone"}
            description={
              form.dns_mode === "cloudflare"
                ? "Orkai creates the CNAME record automatically. The zone can live in a different Cloudflare account than hosting."
                : "Orkai creates the DNS and certificate records automatically — no manual steps. The zone can live in a different AWS account than your hosting."
            }
          >
            <div className="space-y-4">
              <Field
                label={
                  form.dns_mode === "cloudflare" ? "DNS Cloudflare account" : "DNS AWS account"
                }
                hint={
                  form.dns_mode === "cloudflare"
                    ? "The account that owns the DNS zone for your domain."
                    : "The account that owns the Route 53 hosted zone for your domain."
                }
              >
                {dnsAccounts.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    No {form.dns_mode === "cloudflare" ? "Cloudflare" : "AWS"} accounts configured.{" "}
                    <Link
                      to="/admin/resources"
                      className="text-primary underline underline-offset-4 hover:text-primary/80"
                    >
                      Add one in Resources.
                    </Link>
                  </p>
                ) : (
                  <Select
                    value={form.dns_account_id}
                    onValueChange={(v) =>
                      onChange({
                        ...form,
                        dns_account_id: v,
                        dns_zone_id: "",
                        subdomain: "",
                        custom_domain: "",
                      })
                    }
                  >
                    <SelectTrigger className="border-border bg-background">
                      <SelectValue
                        placeholder={`Select ${form.dns_mode === "cloudflare" ? "Cloudflare" : "AWS"} account`}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {dnsAccounts.map((a: SharedResource) => (
                        <SelectItem key={a.id} value={a.id}>
                          {a.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </Field>

              {form.dns_account_id && (
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field label="Hosted zone" hint="The root domain managed in DNS.">
                    <Select value={form.dns_zone_id} onValueChange={handleZoneChange}>
                      <SelectTrigger className="border-border bg-background">
                        <SelectValue placeholder="Select domain" />
                      </SelectTrigger>
                      <SelectContent>
                        {(dnsZones ?? []).map((z) => (
                          <SelectItem key={z.id} value={z.id}>
                            {z.name.replace(/\.$/, "")}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field label="Subdomain" optional hint="Leave blank to serve on the root domain.">
                    <Input
                      className="border-border bg-background"
                      value={form.subdomain}
                      onChange={(e) => handleSubdomainChange(e.target.value)}
                      placeholder="www or app"
                      disabled={!form.dns_zone_id}
                    />
                  </Field>
                </div>
              )}

              {form.custom_domain && (
                <div className="flex items-center gap-2 rounded-md border bg-background px-3 py-2 text-sm">
                  <Globe className="h-4 w-4 shrink-0 text-primary" />
                  <span className="text-muted-foreground">Your page will be served at</span>
                  <code className="text-xs font-medium text-foreground">{form.custom_domain}</code>
                </div>
              )}
            </div>
          </FormSection>
        </div>
      )}
    </>
  );
}
