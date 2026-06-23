import {
  Edit,
  ExternalLink,
  GitBranch,
  GitFork,
  Loader2,
  Play,
  Server,
  Trash2,
} from "lucide-react";
import { useCallback, useState } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  useCreateResource,
  useDeleteResource,
  useGitHubStatus,
  useResources,
  useTestResource,
} from "@/features/resources";
import type { SharedResource } from "@/features/resources/types";
import { api } from "@/lib/api";
import { statusVariant } from "@/lib/constants";
import { ResourceSheet } from "./-resource-sheet";

export function GitProviderTab() {
  const { data, isLoading } = useResources("git_provider");
  const { data: ghStatus } = useGitHubStatus();
  const [editing, setEditing] = useState<SharedResource | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SharedResource | null>(null);
  const deleteMutation = useDeleteResource();
  const testMutation = useTestResource();

  // GitHub OAuth dialog
  const [showGitHubDialog, setShowGitHubDialog] = useState(false);
  const [gitHubConnectType, setGitHubConnectType] = useState<"personal" | "org">("personal");
  const [gitHubOrg, setGitHubOrg] = useState("");
  const [connectingGitHub, setConnectingGitHub] = useState(false);

  // GitLab sheet
  const [showGitLab, setShowGitLab] = useState(false);
  const [gitlabToken, setGitlabToken] = useState("");
  const [gitlabName, setGitlabName] = useState("");
  const [showGitlabName, setShowGitlabName] = useState(false);

  // Gitea sheet
  const [showGitea, setShowGitea] = useState(false);
  const [giteaToken, setGiteaToken] = useState("");
  const [giteaUrl, setGiteaUrl] = useState("");
  const [giteaName, setGiteaName] = useState("");
  const [showGiteaName, setShowGiteaName] = useState(false);

  // Edit sheet
  const [editSheetOpen, setEditSheetOpen] = useState(false);

  const createMutation = useCreateResource();

  const connectGitHub = useCallback(() => {
    setGitHubConnectType("personal");
    setGitHubOrg("");
    setShowGitHubDialog(true);
  }, []);

  // Manifest Flow: auto-create GitHub App with one click on GitHub
  const handleGitHubSetup = useCallback(async () => {
    try {
      const org = gitHubConnectType === "org" ? gitHubOrg : "";
      const result = await api.get<{
        manifest: Record<string, unknown>;
        github_url: string;
        state: string;
      }>(`/api/v1/auth/github/setup${org ? `?org=${encodeURIComponent(org)}` : ""}`);

      // POST manifest to GitHub via hidden form
      const form = document.createElement("form");
      form.method = "POST";
      form.action = `${result.github_url}?state=${result.state}`;
      const input = document.createElement("input");
      input.type = "hidden";
      input.name = "manifest";
      input.value = JSON.stringify(result.manifest);
      form.appendChild(input);
      document.body.appendChild(form);
      form.submit();
    } catch (err: any) {
      toast.error(err?.detail || "Failed to start GitHub setup");
      setConnectingGitHub(false);
    }
  }, [gitHubConnectType, gitHubOrg]);

  const handleGitHubConnect = useCallback(async () => {
    setConnectingGitHub(true);
    try {
      const params =
        gitHubConnectType === "org"
          ? `?type=org&org=${encodeURIComponent(gitHubOrg)}`
          : "?type=personal";
      const result = await api.get<{ url: string }>(`/api/v1/auth/github/connect${params}`);
      window.location.href = result.url;
    } catch (err: any) {
      const errCode = err?.body?.error || err?.error || "";
      if (errCode === "not_configured") {
        setShowGitHubDialog(false);
        handleGitHubSetup();
      } else {
        toast.error(
          err?.body?.message || err?.message || err?.detail || "Failed to connect GitHub",
        );
        setConnectingGitHub(false);
      }
    }
  }, [gitHubConnectType, gitHubOrg, handleGitHubSetup]);

  const handleGitLabConnect = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      createMutation.mutate(
        {
          name: gitlabName,
          type: "git_provider",
          provider: "gitlab",
          config: { token: gitlabToken },
        },
        {
          onSuccess: () => {
            setShowGitLab(false);
            setGitlabToken("");
            setGitlabName("");
            setShowGitlabName(false);
          },
        },
      );
    },
    [gitlabToken, gitlabName, createMutation],
  );

  const handleGiteaConnect = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      createMutation.mutate(
        {
          name: giteaName,
          type: "git_provider",
          provider: "gitea",
          config: { token: giteaToken, api_url: giteaUrl },
        },
        {
          onSuccess: () => {
            setShowGitea(false);
            setGiteaToken("");
            setGiteaUrl("");
            setGiteaName("");
            setShowGiteaName(false);
          },
        },
      );
    },
    [giteaToken, giteaUrl, giteaName, createMutation],
  );

  const openEdit = useCallback((r: SharedResource) => {
    setEditing(r);
    setEditSheetOpen(true);
  }, []);

  if (isLoading) return <LoadingScreen variant="detail" />;

  const resources = data ?? [];

  return (
    <div className="space-y-4">
      {/* Provider connection cards */}
      <div className="grid gap-4 md:grid-cols-3">
        {/* GitHub - OAuth */}
        <Card
          className="cursor-pointer hover:border-primary transition-colors"
          onClick={connectGitHub}
        >
          <CardContent className="flex flex-col items-center gap-3 py-6">
            <GitBranch className="h-8 w-8" />
            <span className="font-medium">GitHub</span>
            <span className="text-xs text-muted-foreground">OAuth &middot; One-click</span>
          </CardContent>
        </Card>

        {/* GitLab - PAT */}
        <Card
          className="cursor-pointer hover:border-primary transition-colors"
          onClick={() => setShowGitLab(true)}
        >
          <CardContent className="flex flex-col items-center gap-3 py-6">
            <GitFork className="h-8 w-8" />
            <span className="font-medium">GitLab</span>
            <span className="text-xs text-muted-foreground">Personal Access Token</span>
          </CardContent>
        </Card>

        {/* Gitea - PAT */}
        <Card
          className="cursor-pointer hover:border-primary transition-colors"
          onClick={() => setShowGitea(true)}
        >
          <CardContent className="flex flex-col items-center gap-3 py-6">
            <Server className="h-8 w-8" />
            <span className="font-medium">Gitea</span>
            <span className="text-xs text-muted-foreground">Personal Access Token</span>
          </CardContent>
        </Card>
      </div>

      {/* Existing resources list */}
      {resources.length === 0 ? (
        <EmptyState icon={GitBranch as any} message="No git provider resources yet" />
      ) : (
        resources.map((r) => (
          <Card key={r.id}>
            <CardContent className="flex items-center gap-4 p-4">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                <GitBranch className="h-4 w-4 text-primary" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">{r.name}</span>
                  <Badge variant="outline" className="text-xs">
                    {r.provider}
                  </Badge>
                  <Badge variant={statusVariant(r.status)}>{r.status}</Badge>
                </div>
                <p className="text-xs text-muted-foreground">
                  Created {new Date(r.created_at).toLocaleDateString()}
                </p>
              </div>
              <div className="flex items-center gap-1">
                {r.provider === "github" && ghStatus?.install_url && (
                  <a
                    href={ghStatus.install_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    title="Manage repository access"
                  >
                    <Button variant="ghost" size="icon">
                      <ExternalLink className="h-4 w-4" />
                    </Button>
                  </a>
                )}
                <Button
                  variant="ghost"
                  size="icon"
                  title="Test"
                  disabled={testMutation.isPending && testMutation.variables === r.id}
                  onClick={() => testMutation.mutate(r.id)}
                >
                  {testMutation.isPending && testMutation.variables === r.id ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Play className="h-4 w-4" />
                  )}
                </Button>
                <Button variant="ghost" size="icon" title="Edit" onClick={() => openEdit(r)}>
                  <Edit className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  title="Delete"
                  onClick={() => setDeleteTarget(r)}
                >
                  <Trash2 className="h-4 w-4 text-destructive" />
                </Button>
              </div>
            </CardContent>
          </Card>
        ))
      )}

      {/* GitHub OAuth Dialog */}
      <Dialog
        open={showGitHubDialog}
        onOpenChange={(v) => {
          setShowGitHubDialog(v);
          if (!v) {
            setGitHubConnectType("personal");
            setGitHubOrg("");
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Connect GitHub</DialogTitle>
            <DialogDescription>
              Choose whether to connect a personal account or an organization.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="flex gap-3">
              <Button
                variant={gitHubConnectType === "personal" ? "default" : "outline"}
                className="flex-1"
                onClick={() => setGitHubConnectType("personal")}
              >
                Personal Account
              </Button>
              <Button
                variant={gitHubConnectType === "org" ? "default" : "outline"}
                className="flex-1"
                onClick={() => setGitHubConnectType("org")}
              >
                Organization
              </Button>
            </div>
            {gitHubConnectType === "org" && (
              <div className="space-y-2">
                <Label>Organization Name</Label>
                <Input
                  value={gitHubOrg}
                  onChange={(e) => setGitHubOrg(e.target.value)}
                  placeholder="my-org"
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button
              onClick={handleGitHubConnect}
              disabled={connectingGitHub || (gitHubConnectType === "org" && !gitHubOrg.trim())}
            >
              <GitBranch className="h-4 w-4" />
              {connectingGitHub ? "Connecting..." : "Connect with GitHub"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* GitLab Sheet */}
      <Sheet
        open={showGitLab}
        onOpenChange={(open) => {
          setShowGitLab(open);
          if (!open) {
            setGitlabToken("");
            setGitlabName("");
            setShowGitlabName(false);
          }
        }}
      >
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Connect GitLab</SheetTitle>
            <SheetDescription>
              Provide a Personal Access Token to connect your GitLab account.
            </SheetDescription>
          </SheetHeader>
          <form
            onSubmit={handleGitLabConnect}
            className="flex flex-1 flex-col gap-4 overflow-y-auto"
          >
            <Button
              type="button"
              variant="outline"
              onClick={() =>
                window.open("https://gitlab.com/-/user_settings/personal_access_tokens", "_blank")
              }
            >
              <ExternalLink className="h-4 w-4" />
              Generate Token on GitLab
            </Button>
            <div className="space-y-1">
              <Label>Token</Label>
              <Input
                type="password"
                value={gitlabToken}
                onChange={(e) => setGitlabToken(e.target.value)}
                placeholder="glpat-..."
                required
              />
            </div>
            {showGitlabName ? (
              <div className="space-y-1">
                <Label>Name</Label>
                <Input
                  value={gitlabName}
                  onChange={(e) => setGitlabName(e.target.value)}
                  placeholder="Auto-generated"
                />
                <p className="text-xs text-muted-foreground">Leave empty to auto-generate</p>
              </div>
            ) : (
              <button
                type="button"
                className="text-xs text-primary hover:underline text-left"
                onClick={() => setShowGitlabName(true)}
              >
                Custom name
              </button>
            )}
            <div className="mt-auto pt-4">
              <Button type="submit" className="w-full" disabled={createMutation.isPending}>
                {createMutation.isPending ? "Connecting..." : "Connect"}
              </Button>
            </div>
          </form>
        </SheetContent>
      </Sheet>

      {/* Gitea Sheet */}
      <Sheet
        open={showGitea}
        onOpenChange={(open) => {
          setShowGitea(open);
          if (!open) {
            setGiteaToken("");
            setGiteaUrl("");
            setGiteaName("");
            setShowGiteaName(false);
          }
        }}
      >
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Connect Gitea</SheetTitle>
            <SheetDescription>
              Provide your Gitea instance URL and a Personal Access Token.
            </SheetDescription>
          </SheetHeader>
          <form
            onSubmit={handleGiteaConnect}
            className="flex flex-1 flex-col gap-4 overflow-y-auto"
          >
            <div className="space-y-1">
              <Label>Instance URL</Label>
              <Input
                value={giteaUrl}
                onChange={(e) => setGiteaUrl(e.target.value)}
                placeholder="https://gitea.example.com"
                required
              />
            </div>
            <div className="space-y-1">
              <Label>Token</Label>
              <Input
                type="password"
                value={giteaToken}
                onChange={(e) => setGiteaToken(e.target.value)}
                placeholder="Token"
                required
              />
            </div>
            {showGiteaName ? (
              <div className="space-y-1">
                <Label>Name</Label>
                <Input
                  value={giteaName}
                  onChange={(e) => setGiteaName(e.target.value)}
                  placeholder="Auto-generated"
                />
                <p className="text-xs text-muted-foreground">Leave empty to auto-generate</p>
              </div>
            ) : (
              <button
                type="button"
                className="text-xs text-primary hover:underline text-left"
                onClick={() => setShowGiteaName(true)}
              >
                Custom name
              </button>
            )}
            <div className="mt-auto pt-4">
              <Button type="submit" className="w-full" disabled={createMutation.isPending}>
                {createMutation.isPending ? "Connecting..." : "Connect"}
              </Button>
            </div>
          </form>
        </SheetContent>
      </Sheet>

      {/* Edit Sheet (reuses generic ResourceSheet) */}
      <ResourceSheet
        open={editSheetOpen}
        onOpenChange={setEditSheetOpen}
        type="git_provider"
        resource={editing}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete resource"
        description={`Are you sure you want to delete "${deleteTarget?.name}"? This cannot be undone.`}
        confirmLabel="Delete"
        loading={deleteMutation.isPending}
        onConfirm={() => {
          if (deleteTarget) {
            deleteMutation.mutate(deleteTarget.id, {
              onSuccess: () => setDeleteTarget(null),
            });
          }
        }}
      />
    </div>
  );
}
