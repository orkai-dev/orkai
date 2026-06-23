import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { projectKeys } from "@/features/projects";
import { api } from "@/lib/api";
import { endpoints } from "@/lib/endpoints";
import type { PaginatedResponse } from "@/shared/types";
import type { Worker, WorkerDeployment } from "./types";

export const workerKeys = {
  detail: (id: string) => ["workers", id] as const,
  deployments: (id: string) => ["workers", id, "deployments"] as const,
};

// ── All workers (global list) ───────────────────────────────────────

export function useAllWorkers(page = 1, perPage = 20, search?: string, status?: string) {
  const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (search) params.set("search", search);
  if (status) params.set("status", status);

  return useQuery({
    queryKey: ["workers", "all", page, perPage, search, status],
    queryFn: () => api.get<PaginatedResponse<Worker>>(`${endpoints.workers.list()}?${params}`),
  });
}

export function useProjectWorkers(projectId: string) {
  return useQuery({
    queryKey: projectKeys.workers(projectId),
    queryFn: () => api.get<PaginatedResponse<Worker>>(`/api/v1/projects/${projectId}/workers`),
    select: (data) => data.items ?? [],
  });
}

export function useWorker(workerId: string) {
  return useQuery({
    queryKey: workerKeys.detail(workerId),
    queryFn: () => api.get<Worker>(`/api/v1/workers/${workerId}`),
  });
}

export function useCreateWorker(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      git_repo: string;
      git_branch?: string;
      git_provider_id?: string;
      root_directory?: string;
      wrangler_config?: string;
      package_manager?: string;
      install_command?: string;
      build_command?: string;
      deploy_command?: string;
      build_env_vars?: Record<string, string>;
      cloud_account_id?: string;
    }) =>
      api.post<Worker>("/api/v1/workers", {
        ...data,
        project_id: projectId,
      }),
    onSuccess: (_, vars) => {
      toast.success(`Cloudflare Worker "${vars.name}" created`);
      qc.invalidateQueries({ queryKey: projectKeys.workers(projectId) });
    },
    meta: { errorMessage: "Failed to create Cloudflare Worker" },
  });
}

export function useUpdateWorker(workerId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      description?: string;
      git_repo?: string;
      git_branch?: string;
      git_provider_id?: string;
      root_directory?: string;
      wrangler_config?: string;
      package_manager?: string;
      install_command?: string;
      build_command?: string;
      deploy_command?: string;
      build_env_vars?: Record<string, string>;
      cloud_account_id?: string;
    }) => api.patch<Worker>(`/api/v1/workers/${workerId}`, data),
    onSuccess: () => {
      toast.success("Cloudflare Worker updated");
      qc.invalidateQueries({ queryKey: workerKeys.detail(workerId) });
    },
    meta: { errorMessage: "Failed to update" },
  });
}

export function useDeleteWorker(workerId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete(`/api/v1/workers/${workerId}`),
    onSuccess: () => {
      toast.success("Cloudflare Worker deleted");
      qc.invalidateQueries({ queryKey: ["projects"], type: "active" });
      qc.removeQueries({ queryKey: workerKeys.detail(workerId) });
    },
    meta: { errorMessage: "Failed to delete" },
  });
}

export function useDeployWorker(workerId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<WorkerDeployment>(`/api/v1/workers/${workerId}/deploy`),
    onSuccess: () => {
      toast.success("Deployment triggered");
      qc.invalidateQueries({ queryKey: workerKeys.detail(workerId) });
      qc.invalidateQueries({ queryKey: workerKeys.deployments(workerId) });
    },
    meta: { errorMessage: "Deploy failed" },
  });
}

export function useConfirmWorkerR2(workerId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<WorkerDeployment>(`/api/v1/workers/${workerId}/confirm-r2`),
    onSuccess: () => {
      toast.success("R2 bucket confirmed — re-deploying");
      qc.invalidateQueries({ queryKey: workerKeys.detail(workerId) });
      qc.invalidateQueries({ queryKey: workerKeys.deployments(workerId) });
    },
    meta: { errorMessage: "Failed to confirm R2 bucket" },
  });
}

export function useWorkerDeployments(workerId: string) {
  return useQuery({
    queryKey: workerKeys.deployments(workerId),
    queryFn: () =>
      api.get<PaginatedResponse<WorkerDeployment>>(`/api/v1/workers/${workerId}/deployments`),
    enabled: !!workerId,
    select: (data) => data.items ?? [],
  });
}

export function useWorkerDeployment(workerId: string, deployId: string) {
  return useQuery({
    queryKey: ["workers", workerId, "deployments", deployId] as const,
    queryFn: () => api.get<WorkerDeployment>(`/api/v1/workers/${workerId}/deployments/${deployId}`),
    enabled: !!workerId && !!deployId,
  });
}
