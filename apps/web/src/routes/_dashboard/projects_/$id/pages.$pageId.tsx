import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import {
  ChevronDown,
  ChevronRight,
  ExternalLink,
  GitBranch,
  Globe,
  Loader2,
  Rocket,
  Save,
} from "lucide-react";
import { useEffect, useState } from "react";
import { DangerZone } from "@/components/danger-zone";
import { LoadingScreen } from "@/components/loading-screen";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  useDeletePage,
  useDeployPage,
  usePage,
  usePageDeployments,
  useUpdatePage,
} from "@/features/pages/queries";
import type { Page, PageDeployment } from "@/features/pages/types";
import { useDnsZones, useResources } from "@/features/resources";
import type { SharedResource } from "@/features/resources/types";
import { statusVariant } from "@/lib/constants";
import { formatDurationBetween } from "@/lib/format";

export const Route = createFileRoute("/_dashboard/projects_/$id/pages/$pageId")({
  component: PageDetailPage,
});

// ── Helpers ──────────────────────────────────────────────────────────

function pageStatusVariant(status: string) {
  return status === "live" ? "success" : statusVariant(status);
}

function isCloudflarePage(page: Page) {
  return page.provider === "cloudflare_pages";
}

function pageCloudAccountLocked(page: Page) {
  if (isCloudflarePage(page)) {
    return !!page.runtime?.cf_project_id;
  }
  return !!(page.runtime?.bucket_name || page.runtime?.distribution_id);
}

function pageDomainLocked(page: Page) {
  if (isCloudflarePage(page)) {
    return !!page.runtime?.cf_project_id;
  }
  return !!(page.runtime?.certificate_arn || page.runtime?.distribution_id);
}

function deployStatusVariant(status: string) {
  return status === "success" ? "success" : statusVariant(status);
}

// ── Deployment Row ───────────────────────────────────────────────────

function DeploymentRow({ deployment }: { deployment: PageDeployment }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="rounded-md border">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm"
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
          )}
          <Badge variant={deployStatusVariant(deployment.status)} className="text-xs">
            {deployment.status}
          </Badge>
          {deployment.commit_sha && (
            <span className="font-mono text-xs text-muted-foreground">
              {deployment.commit_sha.slice(0, 7)}
            </span>
          )}
          <span className="text-xs text-muted-foreground">
            {new Date(deployment.created_at).toLocaleString()}
          </span>
        </div>
        <span className="text-xs text-muted-foreground">
          {formatDurationBetween(deployment.started_at, deployment.finished_at)}
        </span>
      </button>
      {expanded && (
        <div className="border-t px-3 py-2">
          {deployment.deploy_log ? (
            <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded bg-muted p-3 font-mono text-xs">
              {deployment.deploy_log}
            </pre>
          ) : (
            <p className="text-xs text-muted-foreground">No log output yet.</p>
          )}
        </div>
      )}
    </div>
  );
}

// ── Custom Domain Status Card ──────────────────────────────────────────

function CustomDomainCard({ page }: { page: Page }) {
  const rt = page.runtime;
  const cloudflare = isCloudflarePage(page);
  if (!page.custom_domain && !rt?.certificate_arn && !cloudflare) return null;
  if (cloudflare && !page.custom_domain) return null;

  const validation = rt?.validation_record;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Globe className="h-4 w-4" /> Custom Domain
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {page.custom_domain && (
          <div>
            <span className="text-muted-foreground">Domain: </span>
            <code className="text-xs">{page.custom_domain}</code>
          </div>
        )}
        {!cloudflare && rt?.cert_status && (
          <div>
            <span className="text-muted-foreground">Certificate: </span>
            <Badge variant="outline">{rt.cert_status}</Badge>
          </div>
        )}
        {!cloudflare && validation?.name && (
          <div className="rounded-md border bg-muted/30 p-3 font-mono text-xs">
            <p className="mb-2 font-sans text-muted-foreground">ACM validation record:</p>
            <p>
              {validation.type} {validation.name}
            </p>
            <p className="break-all">{validation.value}</p>
          </div>
        )}
        {rt?.alias_target && (
          <div className="rounded-md border bg-muted/30 p-3 font-mono text-xs">
            <p className="mb-2 font-sans text-muted-foreground">
              {cloudflare ? "Pages CNAME target:" : "CloudFront alias target:"}
            </p>
            <p className="break-all">{rt.alias_target}</p>
            {!cloudflare && (
              <p className="mt-1 font-sans text-muted-foreground">Hosted zone ID: Z2FDTNDATAQYW2</p>
            )}
          </div>
        )}
        {!cloudflare && !page.manage_dns && rt?.cert_status === "PENDING_VALIDATION" && (
          <p className="text-xs text-muted-foreground">
            Add the validation CNAME above to your DNS provider, then re-deploy.
          </p>
        )}
      </CardContent>
    </Card>
  );
}

// ── Settings Card ────────────────────────────────────────────────────

function SettingsCard({ page }: { page: Page }) {
  const updatePage = useUpdatePage(page.id);
  const { data: cloudAccounts } = useResources("cloud_account");
  const cloudflare = isCloudflarePage(page);
  const hostingAccounts = (cloudAccounts ?? []).filter((a) =>
    cloudflare ? a.provider === "cloudflare" : a.provider === "aws",
  );
  const dnsAccounts = (cloudAccounts ?? []).filter((a) =>
    cloudflare ? a.provider === "cloudflare" : a.provider === "aws",
  );
  const regionLocked = !!page.runtime?.bucket_name;
  const cloudAccountLocked = pageCloudAccountLocked(page);
  const domainLocked = pageDomainLocked(page);

  const [description, setDescription] = useState(page.description);
  const [gitRepo, setGitRepo] = useState(page.git_repo);
  const [gitBranch, setGitBranch] = useState(page.git_branch);
  const [publishPath, setPublishPath] = useState(page.publish_path);
  const [cloudAccountId, setCloudAccountId] = useState(page.cloud_account_id || "");
  const [region, setRegion] = useState(page.region);
  const [customDomain, setCustomDomain] = useState(page.custom_domain || "");
  const [manageDns, setManageDns] = useState(page.manage_dns);
  const [dnsAccountId, setDnsAccountId] = useState(page.dns_account_id || "");
  const [dnsZoneId, setDnsZoneId] = useState(page.dns_zone_id || "");
  const { data: dnsZones } = useDnsZones(manageDns && !domainLocked ? dnsAccountId : "");

  useEffect(() => {
    setDescription(page.description);
    setGitRepo(page.git_repo);
    setGitBranch(page.git_branch);
    setPublishPath(page.publish_path);
    setCloudAccountId(page.cloud_account_id || "");
    setRegion(page.region);
    setCustomDomain(page.custom_domain || "");
    setManageDns(page.manage_dns);
    setDnsAccountId(page.dns_account_id || "");
    setDnsZoneId(page.dns_zone_id || "");
  }, [
    page.description,
    page.git_repo,
    page.git_branch,
    page.publish_path,
    page.cloud_account_id,
    page.region,
    page.custom_domain,
    page.manage_dns,
    page.dns_account_id,
    page.dns_zone_id,
  ]);

  const dirty =
    description !== page.description ||
    gitRepo !== page.git_repo ||
    gitBranch !== page.git_branch ||
    publishPath !== page.publish_path ||
    (cloudAccountId || "") !== (page.cloud_account_id || "") ||
    region !== page.region ||
    customDomain !== (page.custom_domain || "") ||
    manageDns !== page.manage_dns ||
    (dnsAccountId || "") !== (page.dns_account_id || "") ||
    (dnsZoneId || "") !== (page.dns_zone_id || "");

  function handleSave() {
    const body: {
      description?: string;
      git_repo?: string;
      git_branch?: string;
      publish_path?: string;
      cloud_account_id?: string;
      region?: string;
      custom_domain?: string;
      manage_dns?: boolean;
      dns_account_id?: string;
      dns_zone_id?: string;
    } = {
      description,
      git_repo: gitRepo,
      git_branch: gitBranch,
      publish_path: publishPath,
      cloud_account_id: cloudAccountLocked ? undefined : cloudAccountId || undefined,
      region: regionLocked ? undefined : region,
    };

    // Only send domain fields that actually changed. Including manage_dns/dns_*
    // on every save would trip the backend's domain re-validation (a live
    // Route53 ListRecords call when manage_dns is on) for unrelated edits.
    if (!domainLocked) {
      const trimmedDomain = customDomain.trim();
      if (trimmedDomain !== (page.custom_domain || "")) {
        // Send the explicit value (including "") so a cleared domain clears
        // server-side instead of being coerced to undefined and ignored.
        body.custom_domain = trimmedDomain;
      }
      if (manageDns !== page.manage_dns) {
        body.manage_dns = manageDns;
      }
      if ((dnsAccountId || "") !== (page.dns_account_id || "")) {
        body.dns_account_id = manageDns ? dnsAccountId || undefined : undefined;
      }
      if ((dnsZoneId || "") !== (page.dns_zone_id || "")) {
        body.dns_zone_id = manageDns ? dnsZoneId || undefined : undefined;
      }
    }

    updatePage.mutate(body);
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <GitBranch className="h-4 w-4" /> Settings
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label>Description</Label>
          <Input
            className="border-border bg-background"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description"
          />
        </div>
        <div className="space-y-2">
          <Label>Git Repository URL</Label>
          <Input
            className="border-border bg-background"
            value={gitRepo}
            onChange={(e) => setGitRepo(e.target.value)}
            placeholder="https://github.com/acme/my-page"
          />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label>Branch</Label>
            <Input
              className="border-border bg-background"
              value={gitBranch}
              onChange={(e) => setGitBranch(e.target.value)}
              placeholder="main"
            />
          </div>
          <div className="space-y-2">
            <Label>Publish Folder</Label>
            <Input
              className="border-border bg-background"
              value={publishPath}
              onChange={(e) => setPublishPath(e.target.value)}
              placeholder="output"
            />
          </div>
        </div>
        <div className="space-y-2">
          <Label>{cloudflare ? "Cloudflare Account" : "AWS Account"}</Label>
          {hostingAccounts.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No {cloudflare ? "Cloudflare" : "AWS"} accounts configured.{" "}
              <Link
                to="/admin/resources"
                className="text-primary underline underline-offset-4 hover:text-primary/80"
              >
                Add one in Resources.
              </Link>
            </p>
          ) : (
            <Select
              value={cloudAccountId}
              onValueChange={setCloudAccountId}
              disabled={cloudAccountLocked}
            >
              <SelectTrigger className="border-border bg-background">
                <SelectValue placeholder={`Select ${cloudflare ? "Cloudflare" : "AWS"} account`} />
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
          {cloudAccountLocked && (
            <p className="text-xs text-muted-foreground">
              Cloud account cannot be changed after the first deploy.
            </p>
          )}
        </div>
        {!cloudflare && (
          <div className="space-y-2">
            <Label>Region</Label>
            <Input
              className="border-border bg-background"
              value={region}
              onChange={(e) => setRegion(e.target.value)}
              placeholder="us-east-1"
              disabled={regionLocked}
            />
            {regionLocked && (
              <p className="text-xs text-muted-foreground">
                Region cannot be changed after the bucket has been provisioned.
              </p>
            )}
          </div>
        )}
        <Separator />
        <div className="space-y-2">
          <Label>Custom Domain</Label>
          <Input
            className="border-border bg-background"
            value={customDomain}
            onChange={(e) => {
              const v = e.target.value;
              setCustomDomain(v);
              if (!v.trim()) {
                setManageDns(false);
                setDnsAccountId("");
                setDnsZoneId("");
              }
            }}
            placeholder="app.example.com"
            disabled={domainLocked}
          />
          {domainLocked && (
            <p className="text-xs text-muted-foreground">
              Custom domain cannot be changed after the page has been provisioned.
            </p>
          )}
        </div>
        {customDomain.trim() && !domainLocked && (
          <>
            <label className="flex items-center gap-2 text-sm font-medium">
              <input
                type="checkbox"
                checked={manageDns}
                onChange={(e) => {
                  setManageDns(e.target.checked);
                  if (!e.target.checked) {
                    setDnsAccountId("");
                    setDnsZoneId("");
                  }
                }}
                className="rounded border-border bg-background"
              />
              Manage DNS automatically ({cloudflare ? "Cloudflare" : "Route53"})
            </label>
            {manageDns && (
              <>
                <div className="space-y-2">
                  <Label>{cloudflare ? "DNS Cloudflare Account" : "DNS AWS Account"}</Label>
                  <Select
                    value={dnsAccountId}
                    onValueChange={(v) => {
                      setDnsAccountId(v);
                      setDnsZoneId("");
                    }}
                  >
                    <SelectTrigger className="border-border bg-background">
                      <SelectValue
                        placeholder={`Select ${cloudflare ? "Cloudflare" : "AWS"} account for DNS`}
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
                </div>
                {dnsAccountId && (
                  <div className="space-y-2">
                    <Label>Hosted Zone</Label>
                    <Select value={dnsZoneId} onValueChange={setDnsZoneId}>
                      <SelectTrigger className="border-border bg-background">
                        <SelectValue placeholder="Select hosted zone" />
                      </SelectTrigger>
                      <SelectContent>
                        {(dnsZones ?? []).map((z) => (
                          <SelectItem key={z.id} value={z.id}>
                            {z.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}
              </>
            )}
          </>
        )}
        {dirty && (
          <Button size="sm" onClick={handleSave} disabled={updatePage.isPending}>
            {updatePage.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Save className="h-3.5 w-3.5" />
            )}{" "}
            Save
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

// ── Build Settings Card ──────────────────────────────────────────────

type EnvRow = { key: string; value: string };

function envToRows(env: Record<string, string> | undefined): EnvRow[] {
  return Object.entries(env ?? {}).map(([key, value]) => ({ key, value }));
}

function rowsToEnv(rows: EnvRow[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const r of rows) {
    const k = r.key.trim();
    if (k) out[k] = r.value;
  }
  return out;
}

function BuildSettingsCard({ page }: { page: Page }) {
  const updatePage = useUpdatePage(page.id);

  const [buildEnabled, setBuildEnabled] = useState(page.build_enabled);
  const [packageManager, setPackageManager] = useState(page.package_manager || "auto");
  const [installCommand, setInstallCommand] = useState(page.install_command);
  const [buildCommand, setBuildCommand] = useState(page.build_command);
  const [outputDir, setOutputDir] = useState(page.output_dir);
  const [rootDirectory, setRootDirectory] = useState(page.root_directory || ".");
  const [nodeVersion, setNodeVersion] = useState(page.node_version);
  const [envRows, setEnvRows] = useState<EnvRow[]>(envToRows(page.build_env_vars));

  // build_env_vars is an object, so React Query returns a new reference on every
  // background refetch. Depending on that reference would re-run the effect and
  // wipe out unsaved edits. Key the resync on the serialized value instead so it
  // only fires when the server data actually changed.
  const envKey = JSON.stringify(page.build_env_vars ?? {});

  useEffect(() => {
    setBuildEnabled(page.build_enabled);
    setPackageManager(page.package_manager || "auto");
    setInstallCommand(page.install_command);
    setBuildCommand(page.build_command);
    setOutputDir(page.output_dir);
    setRootDirectory(page.root_directory || ".");
    setNodeVersion(page.node_version);
    setEnvRows(envToRows(JSON.parse(envKey) as Record<string, string>));
  }, [
    page.build_enabled,
    page.package_manager,
    page.install_command,
    page.build_command,
    page.output_dir,
    page.root_directory,
    page.node_version,
    envKey,
  ]);

  const nextEnv = rowsToEnv(envRows);
  const envChanged = JSON.stringify(nextEnv) !== envKey;
  const dirty =
    buildEnabled !== page.build_enabled ||
    packageManager !== (page.package_manager || "auto") ||
    installCommand !== page.install_command ||
    buildCommand !== page.build_command ||
    outputDir !== page.output_dir ||
    rootDirectory !== (page.root_directory || ".") ||
    nodeVersion !== page.node_version ||
    envChanged;

  function handleSave() {
    updatePage.mutate({
      build_enabled: buildEnabled,
      package_manager: packageManager,
      install_command: installCommand,
      build_command: buildCommand,
      output_dir: outputDir,
      root_directory: rootDirectory,
      node_version: nodeVersion,
      build_env_vars: nextEnv,
    });
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <GitBranch className="h-4 w-4" /> Build
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <label className="flex items-center gap-2 text-sm font-medium">
          <input
            type="checkbox"
            checked={buildEnabled}
            onChange={(e) => setBuildEnabled(e.target.checked)}
            className="rounded border-border bg-background"
          />
          Build from source (npm / pnpm)
        </label>

        {buildEnabled && (
          <>
            <p className="text-xs text-muted-foreground">
              Orkai runs install + build in an isolated build container, then deploys the output
              folder. The package manager is auto-detected from your lockfile.
            </p>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Package Manager</Label>
                <Select
                  value={packageManager}
                  onValueChange={(v) => setPackageManager(v as Page["package_manager"])}
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
              </div>
              <div className="space-y-2">
                <Label>Output Directory</Label>
                <Input
                  className="border-border bg-background"
                  value={outputDir}
                  onChange={(e) => setOutputDir(e.target.value)}
                  placeholder="dist"
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label>Install Command</Label>
              <Input
                className="border-border bg-background"
                value={installCommand}
                onChange={(e) => setInstallCommand(e.target.value)}
                placeholder="pnpm install --frozen-lockfile"
              />
            </div>
            <div className="space-y-2">
              <Label>Build Command</Label>
              <Input
                className="border-border bg-background"
                value={buildCommand}
                onChange={(e) => setBuildCommand(e.target.value)}
                placeholder="pnpm build"
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Root Directory</Label>
                <Input
                  className="border-border bg-background"
                  value={rootDirectory}
                  onChange={(e) => setRootDirectory(e.target.value)}
                  placeholder="."
                />
              </div>
              <div className="space-y-2">
                <Label>Node Version</Label>
                <Input
                  className="border-border bg-background"
                  value={nodeVersion}
                  onChange={(e) => setNodeVersion(e.target.value)}
                  placeholder="22"
                />
              </div>
            </div>

            <Separator />
            <div className="space-y-2">
              <Label>Build Environment Variables</Label>
              <p className="text-xs text-muted-foreground">
                Injected during the build (e.g. <code className="text-[11px]">VITE_API_URL</code>).
                Available to your build command, not at runtime.
              </p>
              <div className="space-y-2">
                {envRows.map((row, i) => (
                  <div key={i} className="flex gap-2">
                    <Input
                      className="border-border bg-background font-mono text-xs"
                      value={row.key}
                      onChange={(e) =>
                        setEnvRows((rows) =>
                          rows.map((r, j) => (j === i ? { ...r, key: e.target.value } : r)),
                        )
                      }
                      placeholder="KEY"
                    />
                    <Input
                      className="border-border bg-background font-mono text-xs"
                      value={row.value}
                      onChange={(e) =>
                        setEnvRows((rows) =>
                          rows.map((r, j) => (j === i ? { ...r, value: e.target.value } : r)),
                        )
                      }
                      placeholder="value"
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setEnvRows((rows) => rows.filter((_, j) => j !== i))}
                    >
                      Remove
                    </Button>
                  </div>
                ))}
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setEnvRows((rows) => [...rows, { key: "", value: "" }])}
                >
                  Add Variable
                </Button>
              </div>
            </div>
          </>
        )}

        {dirty && (
          <Button size="sm" onClick={handleSave} disabled={updatePage.isPending}>
            {updatePage.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Save className="h-3.5 w-3.5" />
            )}{" "}
            Save
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

// ── Main Page ────────────────────────────────────────────────────────

function PageDetailPage() {
  const { id: projectId, pageId } = Route.useParams();
  const navigate = useNavigate();

  // ── Data ──────────────────────────────────────────────────────
  const { data: page, isLoading } = usePage(pageId);
  const { data: rawDeployments } = usePageDeployments(pageId);
  const deployments = rawDeployments ?? [];

  // ── Mutations ─────────────────────────────────────────────────
  const deployPage = useDeployPage(pageId);
  const deletePage = useDeletePage(pageId);

  // ── Local state ───────────────────────────────────────────────
  const [showDelete, setShowDelete] = useState(false);
  const [confirmName, setConfirmName] = useState("");

  if (isLoading) return <LoadingScreen variant="detail" />;
  if (!page) return null;

  const isDeploying = page.status === "deploying";
  // Only point the live link at the custom domain when orka'i manages DNS (it
  // creates the CloudFront alias itself). For manual DNS the alias may not exist
  // yet right after the first deploy, so the custom-domain link would 404 — fall
  // back to the always-resolvable CloudFront URL until the user wires it up.
  const liveUrl =
    page.custom_domain && page.manage_dns
      ? `https://${page.custom_domain}`
      : page.runtime?.default_url;

  return (
    <div>
      {/* ── Header ── */}
      <PageHeader
        title={page.name}
        useBack
        description={
          <span className="flex items-center gap-1">
            <Globe className="h-3 w-3" />
            {page.git_repo} @ {page.git_branch}
          </span>
        }
        badges={
          <>
            <Badge variant={pageStatusVariant(page.status)}>
              {isDeploying && <Loader2 className="mr-1 h-3 w-3 animate-spin" />}
              {page.status}
            </Badge>
            {liveUrl && page.status === "live" && (
              <a
                href={liveUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
              >
                {liveUrl.replace(/^https?:\/\//, "")}
                <ExternalLink className="h-3 w-3" />
              </a>
            )}
          </>
        }
        actions={
          <Button
            onClick={() => deployPage.mutate()}
            disabled={deployPage.isPending || isDeploying}
          >
            {isDeploying ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" /> Deploying…
              </>
            ) : (
              <>
                <Rocket className="h-3.5 w-3.5" /> Deploy
              </>
            )}
          </Button>
        }
      />
      <Separator className="my-5" />

      {isDeploying && (
        <div className="mb-6 flex items-center gap-2 rounded-md border border-blue-500/30 bg-blue-500/10 px-4 py-3 text-sm text-blue-400">
          <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
          {isCloudflarePage(page)
            ? "Deploying to Cloudflare Pages…"
            : "Provisioning CDN — the first deploy can take ~10 minutes."}
        </div>
      )}

      <Tabs defaultValue="deployments">
        <TabsList>
          <TabsTrigger value="deployments" className="gap-1.5">
            Deployments
            {deployments.length > 0 && (
              <Badge variant="outline" className="ml-0.5 h-5 px-1.5 text-xs">
                {deployments.length}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="settings">Settings</TabsTrigger>
        </TabsList>

        {/* ── Deployments Tab ── */}
        <TabsContent value="deployments" className="mt-4 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-sm font-medium">
                <Rocket className="h-4 w-4" /> Deployment History
              </CardTitle>
            </CardHeader>
            <CardContent>
              {deployments.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No deployments yet. Click <strong>Deploy</strong> to publish this page.
                </p>
              ) : (
                <div className="space-y-2">
                  {deployments.map((d) => (
                    <DeploymentRow key={d.id} deployment={d} />
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* ── Settings Tab ── */}
        <TabsContent value="settings" className="mt-4 space-y-6">
          <CustomDomainCard page={page} />
          <SettingsCard page={page} />
          <BuildSettingsCard page={page} />
          <DangerZone
            description={
              isCloudflarePage(page)
                ? "Delete this page. The Cloudflare Pages project will be removed."
                : "Delete this page. The S3 bucket and CloudFront distribution will be removed."
            }
            buttonLabel="Delete Page"
            onDelete={() => setShowDelete(true)}
          />
        </TabsContent>
      </Tabs>

      {/* ── Delete Confirmation (type name to confirm) ── */}
      <Dialog
        open={showDelete}
        onOpenChange={(open) => {
          if (!open) setConfirmName("");
          setShowDelete(open);
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Delete Page</DialogTitle>
            <DialogDescription>
              Permanently delete <strong>{page.name}</strong>?{" "}
              {isCloudflarePage(page)
                ? "The Cloudflare Pages project will be removed."
                : "The S3 bucket and CloudFront distribution will be removed."}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5 py-2">
            <Label htmlFor="confirm-page-name" className="text-sm">
              Type <strong className="font-mono">{page.name}</strong> to confirm
            </Label>
            <Input
              id="confirm-page-name"
              placeholder={page.name}
              value={confirmName}
              onChange={(e) => setConfirmName(e.target.value)}
              autoComplete="off"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDelete(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={confirmName !== page.name || deletePage.isPending}
              onClick={() =>
                deletePage.mutate(undefined, {
                  onSuccess: () => navigate({ to: "/projects/$id", params: { id: projectId } }),
                })
              }
            >
              {deletePage.isPending ? "Deleting..." : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
