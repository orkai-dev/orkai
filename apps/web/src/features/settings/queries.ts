import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { DomainVerification, Settings } from "./types";

export const settingsKeys = {
  all: ["settings"] as const,
};

export function useSettings() {
  return useQuery({
    queryKey: settingsKeys.all,
    queryFn: () => api.get<Settings>("/api/v1/settings"),
  });
}

export function useVerifyDomain() {
  return useMutation({
    mutationFn: (domain: string) =>
      api.get<DomainVerification>(
        `/api/v1/settings/verify-domain?domain=${encodeURIComponent(domain)}`,
      ),
  });
}

export function useUpdateSetting() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { key: string; value: string }) => api.put("/api/v1/settings", data),
    onSuccess: () => {
      toast.success("Setting updated");
      qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
    meta: { errorMessage: "Failed to save" },
  });
}
