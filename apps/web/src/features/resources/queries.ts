import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type {
  DeleteDnsRecordInput,
  DnsRecord,
  DnsZone,
  GitRepo,
  SharedResource,
  UpsertDnsRecordInput,
} from "./types";

export const resourceKeys = {
  all: ["resources"] as const,
  byType: (type: string) => ["resources", type] as const,
};

export function useResources(type?: string) {
  return useQuery({
    queryKey: type ? resourceKeys.byType(type) : resourceKeys.all,
    queryFn: () => api.get<SharedResource[]>(`/api/v1/resources${type ? `?type=${type}` : ""}`),
  });
}

export function useCreateResource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      type: string;
      provider: string;
      config: Record<string, unknown>;
    }) => api.post<SharedResource>("/api/v1/resources", data),
    onSuccess: (data, vars) => {
      toast.success(`"${data.name || vars.name}" created`);
      qc.invalidateQueries({ queryKey: ["resources"] });
    },
    meta: { errorMessage: "Failed to create resource" },
  });
}

export function useUpdateResource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      ...data
    }: {
      id: string;
      name?: string;
      provider?: string;
      config?: Record<string, unknown>;
    }) => api.patch<SharedResource>(`/api/v1/resources/${id}`, data),
    onSuccess: () => {
      toast.success("Updated");
      qc.invalidateQueries({ queryKey: ["resources"] });
    },
    meta: { errorMessage: "Failed to update resource" },
  });
}

export function useDeleteResource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/resources/${id}`),
    onSuccess: () => {
      toast.success("Deleted");
      qc.invalidateQueries({ queryKey: ["resources"] });
    },
    meta: { errorMessage: "Failed to delete resource" },
  });
}

export function useGenerateSSHKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { algorithm: string; name?: string }) =>
      api.post<SharedResource>("/api/v1/resources/generate-ssh-key", data),
    onSuccess: (data) => {
      toast.success(`SSH key "${data.name}" generated`);
      qc.invalidateQueries({ queryKey: ["resources"] });
    },
    meta: { errorMessage: "Generation failed" },
  });
}

export function useSearchGitRepos(resourceId: string, query: string) {
  return useQuery({
    queryKey: ["resources", resourceId, "repos", "search", query] as const,
    queryFn: () =>
      api.get<GitRepo[]>(
        `/api/v1/resources/${resourceId}/repos/search?q=${encodeURIComponent(query)}`,
      ),
    enabled: !!resourceId,
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  });
}

export function useResourceBuckets(cloudAccountId: string) {
  return useQuery({
    queryKey: ["resources", cloudAccountId, "buckets"] as const,
    queryFn: () => api.get<string[]>(`/api/v1/resources/${cloudAccountId}/buckets`),
    enabled: !!cloudAccountId,
    staleTime: 60_000,
    retry: false,
  });
}

export function useGitHubStatus() {
  return useQuery({
    queryKey: ["github", "status"] as const,
    queryFn: () =>
      api.get<{ configured: boolean; app_name: string; install_url: string }>(
        "/api/v1/auth/github/status",
      ),
  });
}

export function useTestResource() {
  return useMutation({
    mutationFn: (id: string) =>
      api.post<{ success: boolean; message: string }>(`/api/v1/resources/${id}/test`),
    onSuccess: (data) => {
      if (data.success) {
        toast.success(data.message);
      } else {
        toast.error(data.message);
      }
    },
    meta: { errorMessage: "Test failed" },
  });
}

export function useDnsZones(resourceId: string) {
  return useQuery({
    queryKey: ["resources", resourceId, "dns", "zones"] as const,
    queryFn: () => api.get<DnsZone[]>(`/api/v1/resources/${resourceId}/dns/zones`),
    enabled: !!resourceId,
  });
}

export function useDnsRecords(resourceId: string, zoneId: string) {
  return useQuery({
    queryKey: ["resources", resourceId, "dns", "records", zoneId] as const,
    queryFn: () =>
      api.get<DnsRecord[]>(
        `/api/v1/resources/${resourceId}/dns/records?zone_id=${encodeURIComponent(zoneId)}`,
      ),
    enabled: !!resourceId && !!zoneId,
  });
}

export function useUpsertDnsRecord(resourceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: UpsertDnsRecordInput) =>
      api.post<{ success: boolean }>(`/api/v1/resources/${resourceId}/dns/records`, data),
    onSuccess: (_data, vars) => {
      toast.success("DNS record saved");
      qc.invalidateQueries({
        queryKey: ["resources", resourceId, "dns", "records", vars.zone_id],
      });
    },
    meta: { errorMessage: "Failed to save DNS record" },
  });
}

export function useDeleteDnsRecord(resourceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: DeleteDnsRecordInput) =>
      api.post(`/api/v1/resources/${resourceId}/dns/records/delete`, data),
    onSuccess: (_data, vars) => {
      toast.success("DNS record deleted");
      qc.invalidateQueries({
        queryKey: ["resources", resourceId, "dns", "records", vars.zone_id],
      });
    },
    meta: { errorMessage: "Failed to delete DNS record" },
  });
}
