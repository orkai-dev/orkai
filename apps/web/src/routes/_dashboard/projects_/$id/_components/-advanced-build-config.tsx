import { ExternalLink, Hammer, Plus, Settings2, Trash2, X } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { useClearBuildCache, useUpdateApp } from "@/features/apps";
import type { App } from "@/features/apps/types";
import { InfoBanner, SectionCard } from "./-advanced-shared";

export function SourceProviderCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const isGit = app.source_type === "git";

  const [dockerImage, setDockerImage] = useState(app.docker_image || "");
  const [gitRepo, setGitRepo] = useState(app.git_repo || "");
  const [gitBranch, setGitBranch] = useState(app.git_branch || "main");

  const imageDirty = dockerImage !== (app.docker_image || "");
  const repoDirty = gitRepo !== (app.git_repo || "") || gitBranch !== (app.git_branch || "main");
  const dirty = imageDirty || repoDirty;

  function handleSave() {
    if (isGit) {
      updateApp.mutate({ git_repo: gitRepo, git_branch: gitBranch });
    } else {
      updateApp.mutate({ docker_image: dockerImage });
    }
  }

  return (
    <SectionCard
      icon={Settings2}
      title="Source Provider"
      description="Where your application code comes from. Changes take effect on next deploy."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="mb-4 inline-flex rounded-lg border bg-muted p-0.5">
        <span
          className={`rounded-md px-3 py-1 text-xs font-medium ${!isGit ? "bg-background text-foreground shadow-sm" : "text-muted-foreground"}`}
        >
          Docker Image
        </span>
        <span
          className={`rounded-md px-3 py-1 text-xs font-medium ${isGit ? "bg-background text-foreground shadow-sm" : "text-muted-foreground"}`}
        >
          Git Repository
        </span>
      </div>

      {isGit ? (
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label className="text-xs">Repository</Label>
            <div className="flex items-center gap-2">
              <Input
                value={gitRepo}
                onChange={(e) => setGitRepo(e.target.value)}
                placeholder="https://github.com/org/repo"
                className="flex-1 font-mono text-xs"
              />
              {app.git_repo && (
                <a href={app.git_repo} target="_blank" rel="noopener noreferrer">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 shrink-0"
                    title="Open repository"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                  </Button>
                </a>
              )}
            </div>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">Branch</Label>
            <Input
              value={gitBranch}
              onChange={(e) => setGitBranch(e.target.value)}
              placeholder="main"
              list="git-branches"
              className="w-48"
            />
            <datalist id="git-branches">
              <option value="main" />
              <option value="master" />
              <option value="develop" />
              <option value="staging" />
              <option value="production" />
            </datalist>
          </div>
          <div className="flex items-center justify-between gap-4 py-2">
            <span className="text-sm text-muted-foreground">Auto Deploy</span>
            <Badge variant={app.auto_deploy ? "success" : "secondary"}>
              {app.auto_deploy ? "Enabled" : "Disabled"}
            </Badge>
          </div>
        </div>
      ) : (
        <div className="space-y-1.5">
          <Label className="text-xs">Image</Label>
          <Input
            value={dockerImage}
            onChange={(e) => setDockerImage(e.target.value)}
            placeholder="nginx:latest"
            className="font-mono text-xs"
          />
        </div>
      )}
    </SectionCard>
  );
}

export function BuildConfigCard({ app, appId }: { app: App; appId: string }) {
  const updateApp = useUpdateApp(appId);
  const clearCache = useClearBuildCache(appId);
  const [buildType, setBuildType] = useState(app.build_type || "dockerfile");
  const [dockerfile, setDockerfile] = useState(app.dockerfile || "Dockerfile");
  const [buildContext, setBuildContext] = useState(app.build_context || ".");
  const [watchPaths, setWatchPaths] = useState<string[]>(app.watch_paths || []);
  const [noCache, setNoCache] = useState(app.no_cache || false);
  const [newPath, setNewPath] = useState("");

  const dirty =
    buildType !== (app.build_type || "dockerfile") ||
    dockerfile !== (app.dockerfile || "Dockerfile") ||
    buildContext !== (app.build_context || ".") ||
    noCache !== (app.no_cache || false) ||
    JSON.stringify(watchPaths) !== JSON.stringify(app.watch_paths || []);

  const addWatchPath = () => {
    const trimmed = newPath.trim();
    if (trimmed && !watchPaths.includes(trimmed)) {
      setWatchPaths([...watchPaths, trimmed]);
      setNewPath("");
    }
  };

  const removeWatchPath = (path: string) => {
    setWatchPaths(watchPaths.filter((p) => p !== path));
  };

  function handleSave() {
    updateApp.mutate({
      build_type: buildType,
      dockerfile: buildType === "dockerfile" ? dockerfile : "",
      build_context: buildContext,
      watch_paths: watchPaths,
      no_cache: noCache,
    });
  }

  return (
    <SectionCard
      icon={Hammer}
      title="Build Configuration"
      description="How your code is built into a container image."
      dirty={dirty}
      saving={updateApp.isPending}
      onSave={handleSave}
    >
      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label className="text-xs">Build Type</Label>
          <div className="grid grid-cols-2 gap-4">
            {[
              { value: "dockerfile", label: "Dockerfile", desc: "Use your own Dockerfile" },
              { value: "nixpacks", label: "Nixpacks", desc: "Auto-detect language & build" },
            ].map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => setBuildType(opt.value)}
                className={`rounded-lg border p-3 text-left transition-colors ${
                  buildType === opt.value ? "border-primary bg-primary/5" : "hover:bg-accent"
                }`}
              >
                <p
                  className={`text-sm font-medium ${buildType === opt.value ? "text-primary" : ""}`}
                >
                  {opt.label}
                </p>
                <p className="text-xs text-muted-foreground">{opt.desc}</p>
              </button>
            ))}
          </div>
        </div>

        {buildType === "dockerfile" && (
          <div className="space-y-1.5">
            <Label className="text-xs">Dockerfile Path</Label>
            <Input
              value={dockerfile}
              onChange={(e) => setDockerfile(e.target.value)}
              placeholder="Dockerfile"
              className="w-64 font-mono text-xs"
            />
            <p className="text-xs text-muted-foreground">
              Relative to build context, e.g. Dockerfile, Dockerfile.prod, docker/Dockerfile
            </p>
          </div>
        )}

        {buildType === "nixpacks" && (
          <InfoBanner>
            Nixpacks automatically detects your language and builds the optimal image. No
            configuration needed.
          </InfoBanner>
        )}

        <div className="space-y-1.5">
          <Label className="text-xs">Build Context</Label>
          <Input
            value={buildContext}
            onChange={(e) => setBuildContext(e.target.value)}
            placeholder="/ (repository root)"
            className="w-64 font-mono text-xs"
          />
          <p className="text-xs text-muted-foreground">
            Leave empty for repository root. For monorepos, e.g. apps/api, packages/web
          </p>
        </div>

        <div className="space-y-1.5">
          <Label className="text-xs">Watch Paths</Label>
          <p className="text-xs text-muted-foreground">
            Only trigger auto-deploy when files in these paths change. Leave empty to deploy on any
            change.
          </p>
          {watchPaths.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {watchPaths.map((p) => (
                <span
                  key={p}
                  className="inline-flex items-center gap-1 rounded-md border bg-muted px-2 py-0.5 font-mono text-xs"
                >
                  {p}
                  <button
                    type="button"
                    onClick={() => removeWatchPath(p)}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </span>
              ))}
            </div>
          )}
          <div className="flex items-center gap-2">
            <Input
              value={newPath}
              onChange={(e) => setNewPath(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  addWatchPath();
                }
              }}
              placeholder="apps/api/**, *.go, src/"
              className="w-64 font-mono text-xs"
            />
            <Button
              type="button"
              variant="outline"
              size="icon"
              className="h-8 w-8 shrink-0"
              onClick={addWatchPath}
              disabled={!newPath.trim()}
            >
              <Plus className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>

        <Separator />
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium">Build Cache</p>
            <p className="text-xs text-muted-foreground">
              Disable to force a clean build every time. Slower but avoids stale layers.
            </p>
          </div>
          <div className="flex items-center gap-3">
            <Button
              size="sm"
              variant="outline"
              onClick={() => clearCache.mutate(undefined)}
              disabled={clearCache.isPending}
            >
              <Trash2 className="h-3.5 w-3.5" />{" "}
              {clearCache.isPending ? "Clearing..." : "Clear Cache"}
            </Button>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={!noCache}
                onChange={(e) => setNoCache(!e.target.checked)}
                className="rounded"
              />
              Enabled
            </label>
          </div>
        </div>
      </div>
    </SectionCard>
  );
}
