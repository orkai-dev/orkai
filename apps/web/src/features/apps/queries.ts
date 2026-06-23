import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { Deployment } from "@/features/deployments/types";
import { projectKeys } from "@/features/projects";
import { api } from "@/lib/api";
import { endpoints } from "@/lib/endpoints";
import type { AppStatus, PaginatedResponse, PodEvent, PodInfo } from "@/shared/types";
import type { App, Domain, TargetCapabilities, WebhookConfig } from "./types";

export const appKeys = {
  detail: (id: string) => ["apps", id] as const,
  status: (id: string) => ["apps", id, "status"] as const,
  pods: (id: string) => ["apps", id, "pods"] as const,
  podEvents: (id: string, podName: string) => ["apps", id, "pods", podName, "events"] as const,
  deployments: (id: string) => ["apps", id, "deployments"] as const,
  deployment: (id: string) => ["deployments", id] as const,
  domains: (id: string) => ["apps", id, "domains"] as const,
  capabilities: (id: string) => ["apps", id, "capabilities"] as const,
};

// ── All apps (global list) ────────────────────────────────────────

export function useAllApps(
  page = 1,
  perPage = 20,
  search?: string,
  status?: string,
  options?: { enabled?: boolean },
) {
  const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (search) params.set("search", search);
  if (status) params.set("status", status);

  return useQuery({
    queryKey: ["apps", "all", page, perPage, search, status],
    queryFn: () => api.get<PaginatedResponse<App>>(`${endpoints.apps.list()}?${params}`),
    enabled: options?.enabled ?? true,
  });
}

// ── Queries ───────────────────────────────────────────────────────

export function useProjectApps(projectId: string) {
  return useQuery({
    queryKey: projectKeys.apps(projectId),
    queryFn: () => api.get<PaginatedResponse<App>>(endpoints.projects.apps(projectId)),
    select: (data) => data.items ?? [],
  });
}

export function useApp(appId: string) {
  return useQuery({
    queryKey: appKeys.detail(appId),
    queryFn: () => api.get<App>(endpoints.apps.detail(appId)),
  });
}

export function useAppTargetCapabilities(appId: string) {
  return useQuery({
    queryKey: appKeys.capabilities(appId),
    queryFn: () => api.get<TargetCapabilities>(endpoints.apps.capabilities(appId)),
  });
}

export function useAppStatus(appId: string) {
  return useQuery({
    queryKey: appKeys.status(appId),
    queryFn: () => api.get<AppStatus>(endpoints.apps.status(appId)),
    refetchInterval: (query) => {
      const phase = query.state.data?.phase;
      if (!phase) return 3_000;
      const stable = ["running", "stopped", "error", "failed", "partial", "not deployed"];
      return stable.includes(phase) ? 30_000 : 3_000;
    },
  });
}

export function useAppPods(appId: string) {
  return useQuery({
    queryKey: appKeys.pods(appId),
    queryFn: () => api.get<PodInfo[]>(endpoints.apps.pods(appId)),
    refetchInterval: (query) => {
      const pods = query.state.data;
      if (!pods) return 3_000;
      if (pods.length === 0) return 30_000;
      const allRunning = pods.every((p) => p.phase === "Running");
      return allRunning ? 30_000 : 3_000; // Fast poll during transitions, slow poll when stable
    },
  });
}

export function usePodEvents(appId: string, podName: string) {
  return useQuery({
    queryKey: appKeys.podEvents(appId, podName),
    queryFn: () => api.get<PodEvent[]>(endpoints.apps.podEvents(appId, podName)),
    enabled: !!podName,
  });
}

export function useAppDeployments(appId: string) {
  return useQuery({
    queryKey: appKeys.deployments(appId),
    queryFn: () => api.get<PaginatedResponse<Deployment>>(endpoints.apps.deployments(appId)),
    select: (data) => data.items ?? [],
  });
}

export function useDeploymentDetail(deployId: string) {
  return useQuery({
    queryKey: appKeys.deployment(deployId),
    queryFn: () => api.get<Deployment>(endpoints.deployments.detail(deployId)),
    enabled: !!deployId,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      // Poll every 3s while build/deploy is in progress
      if (status === "queued" || status === "building" || status === "deploying") {
        return 3000;
      }
      return false;
    },
  });
}

export function useAppDomains(appId: string) {
  return useQuery({
    queryKey: appKeys.domains(appId),
    queryFn: () => api.get<Domain[]>(endpoints.apps.domains(appId)),
    refetchInterval: (query) => {
      const domains = query.state.data;
      if (!domains) return 5_000;
      // Poll faster while any domain has pending ingress
      return domains.some((d) => !d.ingress_ready) ? 10_000 : 60_000;
    },
  });
}

// ── Invalidation helpers ──────────────────────────────────────────

function useInvalidateApp(appId: string) {
  const qc = useQueryClient();
  return () => {
    const invalidate = () => {
      qc.invalidateQueries({ queryKey: ["apps", appId] });
      qc.invalidateQueries({ queryKey: ["projects"], type: "active" });
    };
    invalidate();
    setTimeout(invalidate, 1500);
  };
}

// ── Mutations ─────────────────────────────────────────────────────

export function useCreateApp(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      is_critical?: boolean;
      source_type: string;
      docker_image?: string;
      git_repo?: string;
      git_branch?: string;
      registry_id?: string;
    }) => api.post<App>(endpoints.apps.create(), { ...data, project_id: projectId }),
    onSuccess: (_, vars) => {
      toast.success(`Application "${vars.name}" created`);
      qc.invalidateQueries({ queryKey: projectKeys.apps(projectId) });
    },
    meta: { errorMessage: "Failed to create app" },
  });
}

export function useUpdateApp(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (data: Partial<App>) => api.patch<App>(endpoints.apps.detail(appId), data),
    onSuccess: () => {
      toast.success("Settings saved");
      invalidate();
    },
    meta: { errorMessage: "Save failed" },
  });
}

export function useDeploy(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (opts?: { force_build?: boolean }) =>
      api.post(endpoints.apps.deploy(appId), opts ?? {}),
    onSuccess: () => {
      toast.success("Deployment triggered");
      invalidate();
    },
    meta: { errorMessage: "Deploy failed" },
  });
}

export function useCancelDeploy(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (deployId: string) => api.post(endpoints.deployments.cancel(deployId)),
    onSuccess: () => {
      toast.success("Deployment cancelled");
      invalidate();
    },
    meta: { errorMessage: "Cancel failed" },
  });
}

export function useClearBuildCache(appId: string) {
  return useMutation({
    mutationFn: () => api.post(endpoints.apps.clearCache(appId)),
    onSuccess: () => toast.success("Build cache cleared"),
    meta: { errorMessage: "Failed to clear cache" },
  });
}

export function useRestartApp(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: () => api.post(endpoints.apps.restart(appId)),
    onSuccess: () => {
      toast.success("Restart triggered");
      invalidate();
    },
    meta: { errorMessage: "Restart failed" },
  });
}

export function useStopApp(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: () => api.post(endpoints.apps.stop(appId)),
    onSuccess: () => {
      toast.success("Stopped");
      invalidate();
    },
    meta: { errorMessage: "Stop failed" },
  });
}

export function useScaleApp(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (replicas: number) => api.post(endpoints.apps.scale(appId), { replicas }),
    onSuccess: (_, replicas) => {
      toast.success(`Scaled to ${replicas}`);
      invalidate();
    },
    meta: { errorMessage: "Scale failed" },
  });
}

export function useUpdateEnv(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (envVars: Record<string, string>) =>
      api.put(endpoints.apps.env(appId), { env_vars: envVars }),
    onSuccess: () => {
      toast.success("Environment saved");
      invalidate();
    },
    meta: { errorMessage: "Save failed" },
  });
}

export function useAddDomain(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: (data: { host: string; tls: boolean; auto_cert: boolean }) =>
      api.post(endpoints.apps.domains(appId), data),
    onSuccess: () => {
      toast.success("Domain added");
      invalidate();
    },
    meta: { errorMessage: "Failed to add domain" },
  });
}

export function useGenerateDomain(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: () => api.post<Domain>(endpoints.apps.generateDomain(appId)),
    onSuccess: (domain) => {
      toast.success(`Generated: ${domain.host}`);
      invalidate();
    },
    meta: { errorMessage: "Failed to generate domain" },
  });
}

export function useDeleteDomain(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: ({ id, host }: { id: string; host: string }) =>
      api.delete(endpoints.domains.detail(id)).then(() => host),
    onSuccess: (host) => {
      toast.success(`${host} removed`);
      invalidate();
    },
    meta: { errorMessage: "Failed to remove domain" },
  });
}

export function useUpdateDomain(appId: string) {
  const invalidate = useInvalidateApp(appId);
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string; host?: string; force_https?: boolean }) =>
      api.patch(endpoints.domains.detail(id), data),
    onSuccess: () => {
      toast.success("Domain updated");
      invalidate();
    },
    meta: { errorMessage: "Failed to update domain" },
  });
}

// ── Webhook ──────────────────────────────────────────────────────

export function useWebhookConfig(appId: string) {
  return useQuery({
    queryKey: ["apps", appId, "webhook"] as const,
    queryFn: () => api.get<WebhookConfig>(endpoints.apps.webhook(appId)),
  });
}

export function useEnableWebhook(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<WebhookConfig>(endpoints.apps.webhookEnable(appId)),
    onSuccess: () => {
      toast.success("Webhook enabled");
      qc.invalidateQueries({ queryKey: ["apps", appId] });
    },
    meta: { errorMessage: "Failed to enable webhook" },
  });
}

export function useDisableWebhook(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post(endpoints.apps.webhookDisable(appId)),
    onSuccess: () => {
      toast.success("Webhook disabled");
      qc.invalidateQueries({ queryKey: ["apps", appId] });
    },
    meta: { errorMessage: "Failed to disable webhook" },
  });
}

export function useRegenerateWebhook(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<WebhookConfig>(endpoints.apps.webhookRegenerate(appId)),
    onSuccess: () => {
      toast.success("Webhook secret regenerated");
      qc.invalidateQueries({ queryKey: ["apps", appId] });
    },
    meta: { errorMessage: "Failed to regenerate webhook secret" },
  });
}

// ── Secrets ──────────────────────────────────────────────────────

export function useAppSecrets(appId: string) {
  return useQuery({
    queryKey: ["apps", appId, "secrets"] as const,
    queryFn: () => api.get<string[]>(endpoints.apps.secrets(appId)),
  });
}

export function useUpdateSecrets(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (secrets: Record<string, string>) =>
      api.put<string[]>(endpoints.apps.secrets(appId), secrets),
    onSuccess: () => {
      toast.success("Secrets saved");
      qc.invalidateQueries({ queryKey: ["apps", appId, "secrets"] });
    },
    meta: { errorMessage: "Failed to save secrets" },
  });
}

export function useDeleteApp(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete(endpoints.apps.detail(appId)),
    onSuccess: () => {
      toast.success("Application deleted");
      qc.invalidateQueries({ queryKey: ["projects"], type: "active" });
      qc.removeQueries({ queryKey: ["apps", appId] });
    },
    meta: { errorMessage: "Failed to delete" },
  });
}
