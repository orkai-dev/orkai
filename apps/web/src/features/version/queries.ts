import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { endpoints } from "@/lib/endpoints";
import type { VersionInfo } from "./types";

export function useVersion() {
  return useQuery({
    queryKey: ["version"],
    queryFn: () => api.get<VersionInfo>(endpoints.version.get()),
    refetchInterval: 60 * 60 * 1000,
    staleTime: 60 * 60 * 1000,
  });
}

export function useTriggerUpgrade() {
  return useMutation({
    mutationFn: () => api.post<{ message: string }>(endpoints.system.upgrade(), {}),
    meta: { skipErrorToast: true },
  });
}
