import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import {
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  GitBranch,
  Loader2,
  Rocket,
  Save,
  Zap,
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
import { useResources } from "@/features/resources";
import type { SharedResource } from "@/features/resources/types";
import {
  useConfirmWorkerR2,
  useDeleteWorker,
  useDeployWorker,
  useUpdateWorker,
  useWorker,
  useWorkerDeployments,
} from "@/features/workers/queries";
import type { Worker, WorkerDeployment } from "@/features/workers/types";
import { statusVariant } from "@/lib/constants";
import { formatDurationBetween } from "@/lib/format";

export const Route = createFileRoute("/_dashboard/projects_/$id/workers/$workerId")({
  component: WorkerDetailPage,
});

// ── Helpers ──────────────────────────────────────────────────────────

function workerStatusVariant(status: string) {
  return status === "live" ? "success" : statusVariant(status);
}

function deployStatusVariant(status: string) {
  return status === "success" ? "success" : statusVariant(status);
}

// The cloud account is locked once the script has been deployed (changing it
// would orphan the script under the old Cloudflare account).
function workerCloudAccountLocked(worker: Worker) {
  return !!worker.runtime?.script_name;
}

// ── Deployment Row ───────────────────────────────────────────────────

function DeploymentRow({ deployment }: { deployment: WorkerDeployment }) {
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

// ── R2 Confirmation Banner ───────────────────────────────────────────

function R2ConfirmBanner({
  workerId,
  deployment,
}: {
  workerId: string;
  deployment: WorkerDeployment;
}) {
  const confirmR2 = useConfirmWorkerR2(workerId);
  const buckets = deployment.r2_pending ?? [];

  return (
    <div className="mb-6 rounded-md border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm">
      <div className="flex items-start gap-2 text-amber-500">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
        <div className="space-y-2">
          <p className="font-medium text-amber-400">Confirm R2 bucket reuse</p>
          <p className="text-muted-foreground">
            {buckets.length === 1 ? "This bucket" : "These buckets"} referenced by your wrangler
            config already exist in your Cloudflare account. They may belong to another app —
            confirm to reuse {buckets.length === 1 ? "it" : "them"} and continue deploying.
          </p>
          <ul className="space-y-1">
            {buckets.map((b) => (
              <li key={b.name} className="flex items-center gap-2">
                <code className="text-xs">{b.name}</code>
                <Badge variant={b.empty ? "outline" : "secondary"} className="text-[10px]">
                  {b.empty ? "empty" : "not empty"}
                </Badge>
              </li>
            ))}
          </ul>
          <Button size="sm" onClick={() => confirmR2.mutate()} disabled={confirmR2.isPending}>
            {confirmR2.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Rocket className="h-3.5 w-3.5" />
            )}{" "}
            Use {buckets.length === 1 ? "this bucket" : "these buckets"} & deploy
          </Button>
        </div>
      </div>
    </div>
  );
}

// ── Settings Card ────────────────────────────────────────────────────

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

function SettingsCard({ worker }: { worker: Worker }) {
  const updateWorker = useUpdateWorker(worker.id);
  const { data: cloudAccounts } = useResources("cloud_account");
  const cfAccounts = (cloudAccounts ?? []).filter((a) => a.provider === "cloudflare");
  const cloudAccountLocked = workerCloudAccountLocked(worker);

  const [description, setDescription] = useState(worker.description);
  const [gitRepo, setGitRepo] = useState(worker.git_repo);
  const [gitBranch, setGitBranch] = useState(worker.git_branch);
  const [rootDirectory, setRootDirectory] = useState(worker.root_directory || ".");
  const [wranglerConfig, setWranglerConfig] = useState(worker.wrangler_config || "wrangler.toml");
  const [packageManager, setPackageManager] = useState(worker.package_manager || "auto");
  const [installCommand, setInstallCommand] = useState(worker.install_command);
  const [buildCommand, setBuildCommand] = useState(worker.build_command);
  const [deployCommand, setDeployCommand] = useState(worker.deploy_command);
  const [cloudAccountId, setCloudAccountId] = useState(worker.cloud_account_id || "");
  const [envRows, setEnvRows] = useState<EnvRow[]>(envToRows(worker.build_env_vars));

  // build_env_vars is an object, so React Query returns a new reference on every
  // background refetch. Key the resync on the serialized value so it only fires
  // when the server data actually changed (avoids wiping unsaved edits).
  const envKey = JSON.stringify(worker.build_env_vars ?? {});

  useEffect(() => {
    setDescription(worker.description);
    setGitRepo(worker.git_repo);
    setGitBranch(worker.git_branch);
    setRootDirectory(worker.root_directory || ".");
    setWranglerConfig(worker.wrangler_config || "wrangler.toml");
    setPackageManager(worker.package_manager || "auto");
    setInstallCommand(worker.install_command);
    setBuildCommand(worker.build_command);
    setDeployCommand(worker.deploy_command);
    setCloudAccountId(worker.cloud_account_id || "");
    setEnvRows(envToRows(JSON.parse(envKey) as Record<string, string>));
  }, [
    worker.description,
    worker.git_repo,
    worker.git_branch,
    worker.root_directory,
    worker.wrangler_config,
    worker.package_manager,
    worker.install_command,
    worker.build_command,
    worker.deploy_command,
    worker.cloud_account_id,
    envKey,
  ]);

  const nextEnv = rowsToEnv(envRows);
  const envChanged = JSON.stringify(nextEnv) !== envKey;
  const dirty =
    description !== worker.description ||
    gitRepo !== worker.git_repo ||
    gitBranch !== worker.git_branch ||
    rootDirectory !== (worker.root_directory || ".") ||
    wranglerConfig !== (worker.wrangler_config || "wrangler.toml") ||
    packageManager !== (worker.package_manager || "auto") ||
    installCommand !== worker.install_command ||
    buildCommand !== worker.build_command ||
    deployCommand !== worker.deploy_command ||
    (cloudAccountId || "") !== (worker.cloud_account_id || "") ||
    envChanged;

  function handleSave() {
    updateWorker.mutate({
      description,
      git_repo: gitRepo,
      git_branch: gitBranch,
      root_directory: rootDirectory,
      wrangler_config: wranglerConfig,
      package_manager: packageManager,
      install_command: installCommand,
      build_command: buildCommand,
      deploy_command: deployCommand,
      cloud_account_id: cloudAccountLocked ? undefined : cloudAccountId || undefined,
      build_env_vars: nextEnv,
    });
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
            placeholder="https://github.com/acme/my-worker"
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
            <Label>Root Directory</Label>
            <Input
              className="border-border bg-background"
              value={rootDirectory}
              onChange={(e) => setRootDirectory(e.target.value)}
              placeholder="."
            />
          </div>
        </div>
        <div className="space-y-2">
          <Label>Wrangler Config</Label>
          <Input
            className="border-border bg-background"
            value={wranglerConfig}
            onChange={(e) => setWranglerConfig(e.target.value)}
            placeholder="auto-detect"
          />
          <p className="text-xs text-muted-foreground">
            Auto-detected (<code className="text-[11px]">wrangler.jsonc</code> /{" "}
            <code className="text-[11px]">.json</code> / <code className="text-[11px]">.toml</code>
            ). Set a path only to override.
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label>Package Manager</Label>
            <Select
              value={packageManager}
              onValueChange={(v) => setPackageManager(v as Worker["package_manager"])}
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
              placeholder="pnpm exec opennextjs-cloudflare build"
            />
            <p className="text-xs text-muted-foreground">
              Runs after install, before deploy. Leave empty to skip.
            </p>
          </div>
          <div className="space-y-2">
            <Label>Deploy Command</Label>
            <Input
              className="border-border bg-background"
              value={deployCommand}
              onChange={(e) => setDeployCommand(e.target.value)}
              placeholder="pnpm deploy"
            />
            <p className="text-xs text-muted-foreground">
              Leave empty to use the default <code>wrangler deploy</code>.
            </p>
          </div>
        </div>
        <div className="space-y-2">
          <Label>Cloudflare Account</Label>
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
              value={cloudAccountId}
              onValueChange={setCloudAccountId}
              disabled={cloudAccountLocked}
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
          {cloudAccountLocked && (
            <p className="text-xs text-muted-foreground">
              Cloud account cannot be changed after the first deploy.
            </p>
          )}
        </div>

        <Separator />
        <div className="space-y-2">
          <Label>Build Environment Variables</Label>
          <p className="text-xs text-muted-foreground">
            Injected during install + <code className="text-[11px]">wrangler deploy</code>. Use
            wrangler secrets for runtime values.
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

        {dirty && (
          <Button size="sm" onClick={handleSave} disabled={updateWorker.isPending}>
            {updateWorker.isPending ? (
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

// ── Runtime Card ─────────────────────────────────────────────────────

function RuntimeCard({ worker }: { worker: Worker }) {
  const rt = worker.runtime;
  if (!rt?.script_name && !rt?.deployed_url && !(rt?.routes && rt.routes.length > 0)) {
    return null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Zap className="h-4 w-4" /> Cloudflare Worker
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {rt?.script_name && (
          <div>
            <span className="text-muted-foreground">Script: </span>
            <code className="text-xs">{rt.script_name}</code>
          </div>
        )}
        {rt?.deployed_url && (
          <div>
            <span className="text-muted-foreground">URL: </span>
            <a
              href={rt.deployed_url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
            >
              {rt.deployed_url.replace(/^https?:\/\//, "")}
              <ExternalLink className="h-3 w-3" />
            </a>
          </div>
        )}
        {rt?.routes && rt.routes.length > 0 && (
          <div>
            <span className="text-muted-foreground">Routes: </span>
            <div className="mt-1 space-y-1 font-mono text-xs">
              {rt.routes.map((r) => (
                <p key={r} className="break-all">
                  {r}
                </p>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ── Main Page ────────────────────────────────────────────────────────

function WorkerDetailPage() {
  const { id: projectId, workerId } = Route.useParams();
  const navigate = useNavigate();

  const { data: worker, isLoading } = useWorker(workerId);
  const { data: rawDeployments } = useWorkerDeployments(workerId);
  const deployments = rawDeployments ?? [];

  const deployWorker = useDeployWorker(workerId);
  const deleteWorker = useDeleteWorker(workerId);

  const [showDelete, setShowDelete] = useState(false);
  const [confirmName, setConfirmName] = useState("");

  if (isLoading) return <LoadingScreen variant="detail" />;
  if (!worker) return null;

  const isDeploying = worker.status === "deploying";
  const liveUrl = worker.runtime?.deployed_url;
  const pendingR2 = deployments[0]?.status === "needs_confirmation" ? deployments[0] : null;

  return (
    <div>
      <PageHeader
        title={worker.name}
        useBack
        description={
          <span className="flex items-center gap-1">
            <Zap className="h-3 w-3" />
            {worker.git_repo} @ {worker.git_branch}
          </span>
        }
        badges={
          <>
            <Badge variant="outline" className="capitalize">
              Cloudflare Worker
            </Badge>
            <Badge variant={workerStatusVariant(worker.status)}>
              {isDeploying && <Loader2 className="mr-1 h-3 w-3 animate-spin" />}
              {worker.status}
            </Badge>
            {liveUrl && worker.status === "live" && (
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
            onClick={() => deployWorker.mutate()}
            disabled={deployWorker.isPending || isDeploying}
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
          Deploying to Cloudflare Workers…
        </div>
      )}

      {pendingR2 && <R2ConfirmBanner workerId={worker.id} deployment={pendingR2} />}

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
                  No deployments yet. Click <strong>Deploy</strong> to publish this Cloudflare
                  Worker.
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

        <TabsContent value="settings" className="mt-4 space-y-6">
          <RuntimeCard worker={worker} />
          <SettingsCard worker={worker} />
          <DangerZone
            description="Delete this Cloudflare Worker. Orkai will run wrangler delete to tear down the script."
            buttonLabel="Delete Cloudflare Worker"
            onDelete={() => setShowDelete(true)}
          />
        </TabsContent>
      </Tabs>

      <Dialog
        open={showDelete}
        onOpenChange={(open) => {
          if (!open) setConfirmName("");
          setShowDelete(open);
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Delete Cloudflare Worker</DialogTitle>
            <DialogDescription>
              Permanently delete <strong>{worker.name}</strong>? Orkai will run{" "}
              <code className="text-[11px]">wrangler delete</code> to tear down the Cloudflare
              Worker script.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5 py-2">
            <Label htmlFor="confirm-worker-name" className="text-sm">
              Type <strong className="font-mono">{worker.name}</strong> to confirm
            </Label>
            <Input
              id="confirm-worker-name"
              placeholder={worker.name}
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
              disabled={confirmName !== worker.name || deleteWorker.isPending}
              onClick={() =>
                deleteWorker.mutate(undefined, {
                  onSuccess: () => navigate({ to: "/projects/$id", params: { id: projectId } }),
                })
              }
            >
              {deleteWorker.isPending ? "Deleting..." : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
