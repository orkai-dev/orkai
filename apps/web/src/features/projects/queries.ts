import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { PaginatedResponse } from "@/shared/types";
import type { Project } from "./types";

export const projectKeys = {
  all: ["projects"] as const,
  detail: (id: string) => ["projects", id] as const,
  apps: (id: string) => ["projects", id, "apps"] as const,
  databases: (id: string) => ["projects", id, "databases"] as const,
  pages: (id: string) => ["projects", id, "pages"] as const,
  workers: (id: string) => ["projects", id, "workers"] as const,
};

export function useProjects() {
  return useQuery({
    queryKey: projectKeys.all,
    queryFn: () => api.get<PaginatedResponse<Project>>("/api/v1/projects"),
    select: (data) => data.items ?? [],
  });
}

/** Fetches every accessible project (paginated) for cross-project name lookups. */
export function useProjectNameMap() {
  return useQuery({
    queryKey: [...projectKeys.all, "nameMap"],
    queryFn: async () => {
      const all: Project[] = [];
      let page = 1;
      const perPage = 100;
      for (;;) {
        const data = await api.get<PaginatedResponse<Project>>(
          `/api/v1/projects?page=${page}&per_page=${perPage}`,
        );
        const items = data.items ?? [];
        all.push(...items);
        const total = data.pagination?.total ?? items.length;
        if (all.length >= total || items.length < perPage) break;
        page++;
      }
      return new Map(all.map((p) => [p.id, p.name]));
    },
  });
}

export function useProject(id: string) {
  return useQuery({
    queryKey: projectKeys.detail(id),
    queryFn: () => api.get<Project>(`/api/v1/projects/${id}`),
  });
}

export function useCreateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      description: string;
      environment: string;
      team_id: string;
    }) => api.post<Project>("/api/v1/projects", data),
    onSuccess: (_, vars) => {
      toast.success(`Project "${vars.name}" created`);
      qc.invalidateQueries({ queryKey: projectKeys.all });
    },
    meta: { errorMessage: "Failed to create project" },
  });
}

export function useUpdateProject(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name?: string; description?: string; environment?: string }) =>
      api.patch<Project>(`/api/v1/projects/${id}`, data),
    onSuccess: () => {
      toast.success("Project updated");
      qc.invalidateQueries({ queryKey: projectKeys.detail(id) });
    },
    meta: { errorMessage: "Failed to update" },
  });
}

export function useDeleteProject(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete(`/api/v1/projects/${id}`),
    onSuccess: () => {
      toast.success("Project deleted");
      qc.invalidateQueries({ queryKey: projectKeys.all });
    },
    meta: { errorMessage: "Failed to delete" },
  });
}

export function useUpdateProjectEnv(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (envVars: Record<string, string>) =>
      api.put(`/api/v1/projects/${id}/env`, { env_vars: envVars }),
    onSuccess: () => {
      toast.success("Environment saved");
      qc.invalidateQueries({ queryKey: ["projects", id] });
    },
    meta: { errorMessage: "Save failed" },
  });
}
