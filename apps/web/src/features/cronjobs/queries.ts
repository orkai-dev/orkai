import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { projectKeys } from "@/features/projects";
import { api } from "@/lib/api";
import type { PaginatedResponse } from "@/shared/types";
import type { CronJob, CronJobRun } from "./types";

export const cronJobKeys = {
  all: ["cronjobs"] as const,
  byProject: (projectId: string) => ["projects", projectId, "cronjobs"] as const,
  detail: (id: string) => ["cronjobs", id] as const,
  runs: (id: string) => ["cronjobs", id, "runs"] as const,
};

export function useProjectCronJobs(projectId: string) {
  return useQuery({
    queryKey: cronJobKeys.byProject(projectId),
    queryFn: () => api.get<PaginatedResponse<CronJob>>(`/api/v1/projects/${projectId}/cronjobs`),
    select: (data) => data.items ?? [],
  });
}

export function useCronJob(id: string) {
  return useQuery({
    queryKey: cronJobKeys.detail(id),
    queryFn: () => api.get<CronJob>(`/api/v1/cronjobs/${id}`),
    enabled: !!id,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === "running" ? 5_000 : false;
    },
  });
}

export function useCronJobRuns(id: string) {
  return useQuery({
    queryKey: cronJobKeys.runs(id),
    queryFn: () => api.get<PaginatedResponse<CronJobRun>>(`/api/v1/cronjobs/${id}/runs`),
    select: (data) => data.items ?? [],
    enabled: !!id,
    refetchInterval: (query) => {
      const items = Array.isArray(query.state.data) ? query.state.data : [];
      return items.some((r: CronJobRun) => r.status === "running") ? 5_000 : false;
    },
  });
}

export function useCreateCronJob(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      cron_expression: string;
      command: string;
      image?: string;
      timezone?: string;
    }) => api.post<CronJob>("/api/v1/cronjobs", { ...data, project_id: projectId }),
    onSuccess: (_, vars) => {
      toast.success(`CronJob "${vars.name}" created`);
      qc.invalidateQueries({ queryKey: cronJobKeys.byProject(projectId) });
      qc.invalidateQueries({ queryKey: projectKeys.all });
    },
    meta: { errorMessage: "Failed to create cronjob" },
  });
}

export function useUpdateCronJob(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Partial<CronJob>) => api.patch<CronJob>(`/api/v1/cronjobs/${id}`, data),
    onSuccess: () => {
      toast.success("CronJob updated");
      qc.invalidateQueries({ queryKey: cronJobKeys.detail(id) });
      qc.invalidateQueries({ queryKey: cronJobKeys.all });
      qc.invalidateQueries({ queryKey: ["projects"] });
    },
    meta: { errorMessage: "Update failed" },
  });
}

export function useDeleteCronJob(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete(`/api/v1/cronjobs/${id}`),
    onSuccess: () => {
      toast.success("CronJob deleted");
      qc.invalidateQueries({ queryKey: ["cronjobs"] });
      qc.invalidateQueries({ queryKey: ["projects"] });
    },
    meta: { errorMessage: "Delete failed" },
  });
}

export function useTriggerCronJob(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<CronJobRun>(`/api/v1/cronjobs/${id}/trigger`),
    onSuccess: () => {
      toast.success("CronJob triggered");
      qc.invalidateQueries({ queryKey: cronJobKeys.runs(id) });
      qc.invalidateQueries({ queryKey: cronJobKeys.detail(id) });
      qc.invalidateQueries({ queryKey: cronJobKeys.all });
      qc.invalidateQueries({ queryKey: ["projects"] });
    },
    meta: { errorMessage: "Trigger failed" },
  });
}
