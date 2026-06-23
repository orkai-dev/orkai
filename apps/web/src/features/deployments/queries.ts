import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { PaginatedResponse } from "@/shared/types";
import type { Deployment } from "./types";

export const deploymentKeys = {
  all: (page: number, perPage: number, status?: string) =>
    ["deployments", "all", page, perPage, status] as const,
  queue: ["deployments", "queue"] as const,
};

export function useAllDeployments(page = 1, perPage = 20, status?: string) {
  const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (status) params.set("status", status);

  return useQuery({
    queryKey: deploymentKeys.all(page, perPage, status),
    queryFn: () => api.get<PaginatedResponse<Deployment>>(`/api/v1/deployments?${params}`),
  });
}

export function useDeploymentQueue() {
  return useQuery({
    queryKey: deploymentKeys.queue,
    queryFn: () => api.get<PaginatedResponse<Deployment>>("/api/v1/deployments/queue?per_page=100"),
    refetchInterval: 5000,
  });
}
