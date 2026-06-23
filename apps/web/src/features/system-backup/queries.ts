import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { endpoints } from "@/lib/endpoints";
import type { PaginatedResponse } from "@/shared/types";
import type { S3BackupFile, SystemBackup, SystemBackupConfig } from "./types";

export function useSystemBackupConfig() {
  return useQuery({
    queryKey: ["system", "backup", "config"],
    queryFn: () => api.get<SystemBackupConfig>(endpoints.system.backupConfig()),
  });
}

export function useSaveSystemBackupConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SystemBackupConfig) => api.put(endpoints.system.backupConfig(), data),
    onSuccess: () => {
      toast.success("Backup configuration saved");
      qc.invalidateQueries({ queryKey: ["system", "backup"] });
    },
    meta: { errorMessage: "Failed to save backup configuration" },
  });
}

export function useSystemBackups() {
  return useQuery({
    queryKey: ["system", "backup", "list"],
    queryFn: () => api.get<PaginatedResponse<SystemBackup>>(endpoints.system.backupList()),
    select: (data) => data.items ?? [],
    refetchInterval: (query) => {
      const items = Array.isArray(query.state.data) ? query.state.data : [];
      if (items.some((b: SystemBackup) => b.status === "pending" || b.status === "running"))
        return 5000;
      return false;
    },
  });
}

export function useTriggerSystemBackup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<SystemBackup>(endpoints.system.backupTrigger()),
    onSuccess: () => {
      toast.success("Backup started");
      qc.invalidateQueries({ queryKey: ["system", "backup"] });
    },
    meta: { errorMessage: "Backup failed" },
  });
}

export function useScanS3Backups() {
  return useMutation({
    mutationFn: (data: {
      endpoint: string;
      bucket: string;
      access_key: string;
      secret_key: string;
      path: string;
      setup_secret: string;
    }) => {
      const { setup_secret, ...body } = data;
      return api.post<S3BackupFile[]>(endpoints.system.restoreScan(), body, {
        headers: { "X-Setup-Secret": setup_secret },
      });
    },
  });
}

export function useRestoreFromS3() {
  return useMutation({
    mutationFn: (data: {
      endpoint: string;
      bucket: string;
      access_key: string;
      secret_key: string;
      s3_key: string;
      setup_secret: string;
    }) => {
      const { setup_secret, ...body } = data;
      return api.post(endpoints.system.restoreExecute(), body, {
        headers: { "X-Setup-Secret": setup_secret },
      });
    },
  });
}
