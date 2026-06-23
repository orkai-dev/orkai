import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { endpoints } from "@/lib/endpoints";
import type { APIKey, CreateAPIKeyInput, CreateAPIKeyResult } from "./types";

export function useApiKeys() {
  return useQuery({
    queryKey: ["api-keys"],
    queryFn: () => api.get<APIKey[]>(endpoints.apiKeys.list()),
  });
}

export function useCreateApiKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateAPIKeyInput) =>
      api.post<CreateAPIKeyResult>(endpoints.apiKeys.create(), data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-keys"] });
    },
    meta: { errorMessage: "Failed to create API key" },
  });
}

export function useRevokeApiKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(endpoints.apiKeys.revoke(id)),
    onSuccess: () => {
      toast.success("API key revoked");
      qc.invalidateQueries({ queryKey: ["api-keys"] });
    },
    meta: { errorMessage: "Failed to revoke API key" },
  });
}
