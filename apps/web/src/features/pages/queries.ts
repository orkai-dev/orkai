import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { projectKeys } from "@/features/projects";
import { api } from "@/lib/api";
import { endpoints } from "@/lib/endpoints";
import type { PaginatedResponse } from "@/shared/types";
import type { Page, PageDeployment, PageProvider } from "./types";

export const pageKeys = {
  detail: (id: string) => ["pages", id] as const,
  deployments: (id: string) => ["pages", id, "deployments"] as const,
};

// ── All pages (global list) ───────────────────────────────────────

export function useAllPages(
  page = 1,
  perPage = 20,
  search?: string,
  status?: string,
  provider?: PageProvider,
  options?: { enabled?: boolean },
) {
  const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (search) params.set("search", search);
  if (status) params.set("status", status);
  if (provider) params.set("provider", provider);

  return useQuery({
    queryKey: ["pages", "all", page, perPage, search, status, provider],
    queryFn: () => api.get<PaginatedResponse<Page>>(`${endpoints.pages.list()}?${params}`),
    enabled: options?.enabled ?? true,
  });
}

export function useProjectPages(projectId: string) {
  return useQuery({
    queryKey: projectKeys.pages(projectId),
    queryFn: () => api.get<PaginatedResponse<Page>>(`/api/v1/projects/${projectId}/pages`),
    select: (data) => data.items ?? [],
  });
}

export function usePage(pageId: string) {
  return useQuery({
    queryKey: pageKeys.detail(pageId),
    queryFn: () => api.get<Page>(`/api/v1/pages/${pageId}`),
  });
}

export function useCreatePage(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      git_repo: string;
      git_branch?: string;
      git_provider_id?: string;
      publish_path?: string;
      build_enabled?: boolean;
      package_manager?: string;
      install_command?: string;
      build_command?: string;
      output_dir?: string;
      root_directory?: string;
      node_version?: string;
      build_env_vars?: Record<string, string>;
      cloud_account_id?: string;
      region?: string;
      custom_domain?: string;
      manage_dns?: boolean;
      dns_account_id?: string;
      dns_zone_id?: string;
    }) =>
      api.post<Page>("/api/v1/pages", {
        ...data,
        project_id: projectId,
      }),
    onSuccess: (_, vars) => {
      toast.success(`Page "${vars.name}" created`);
      qc.invalidateQueries({ queryKey: projectKeys.pages(projectId) });
    },
    meta: { errorMessage: "Failed to create page" },
  });
}

export function useUpdatePage(pageId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      description?: string;
      git_repo?: string;
      git_branch?: string;
      git_provider_id?: string;
      publish_path?: string;
      build_enabled?: boolean;
      package_manager?: string;
      install_command?: string;
      build_command?: string;
      output_dir?: string;
      root_directory?: string;
      node_version?: string;
      build_env_vars?: Record<string, string>;
      cloud_account_id?: string;
      region?: string;
      custom_domain?: string;
      manage_dns?: boolean;
      dns_account_id?: string;
      dns_zone_id?: string;
    }) => api.patch<Page>(`/api/v1/pages/${pageId}`, data),
    onSuccess: () => {
      toast.success("Page updated");
      qc.invalidateQueries({ queryKey: pageKeys.detail(pageId) });
    },
    meta: { errorMessage: "Failed to update" },
  });
}

export function useDeletePage(pageId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete(`/api/v1/pages/${pageId}`),
    onSuccess: () => {
      toast.success("Page deleted");
      qc.invalidateQueries({ queryKey: ["projects"], type: "active" });
      qc.removeQueries({ queryKey: pageKeys.detail(pageId) });
    },
    meta: { errorMessage: "Failed to delete" },
  });
}

export function useDeployPage(pageId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<PageDeployment>(`/api/v1/pages/${pageId}/deploy`),
    onSuccess: () => {
      toast.success("Deployment triggered");
      qc.invalidateQueries({ queryKey: pageKeys.detail(pageId) });
      qc.invalidateQueries({ queryKey: pageKeys.deployments(pageId) });
    },
    meta: { errorMessage: "Deploy failed" },
  });
}

export function usePageDeployments(pageId: string) {
  return useQuery({
    queryKey: pageKeys.deployments(pageId),
    queryFn: () =>
      api.get<PaginatedResponse<PageDeployment>>(`/api/v1/pages/${pageId}/deployments`),
    enabled: !!pageId,
    select: (data) => data.items ?? [],
  });
}

export function usePageDeployment(pageId: string, deployId: string) {
  return useQuery({
    queryKey: ["pages", pageId, "deployments", deployId] as const,
    queryFn: () => api.get<PageDeployment>(`/api/v1/pages/${pageId}/deployments/${deployId}`),
    enabled: !!pageId && !!deployId,
  });
}
