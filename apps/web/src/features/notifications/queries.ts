import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { endpoints } from "@/lib/endpoints";
import type { NotificationChannel, NotifyEventInfo, SMTPConfig } from "./types";

export function useNotificationChannels() {
  return useQuery({
    queryKey: ["notifications", "channels"],
    queryFn: () => api.get<NotificationChannel[]>(endpoints.notifications.channels()),
  });
}

export function useNotifyEvents() {
  return useQuery({
    queryKey: ["notifications", "events"],
    queryFn: () => api.get<NotifyEventInfo[]>(endpoints.notifications.events()),
  });
}

export function useSaveChannel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { type: string; enabled: boolean; config: Record<string, unknown> }) =>
      api.put(endpoints.notifications.channels(), data),
    onSuccess: () => {
      toast.success("Channel saved");
      qc.invalidateQueries({ queryKey: ["notifications"] });
    },
    meta: { errorMessage: "Failed to save channel" },
  });
}

export function useTestChannel() {
  return useMutation({
    mutationFn: (type: string) => api.post(endpoints.notifications.test(), { type }),
    onSuccess: () => toast.success("Test notification sent"),
    meta: { errorMessage: "Test failed" },
  });
}

export function useSMTPConfig() {
  return useQuery({
    queryKey: ["settings", "smtp"],
    queryFn: () => api.get<SMTPConfig>(endpoints.settings.smtp()),
  });
}

export function useSaveSMTPConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: SMTPConfig) => api.put(endpoints.settings.smtp(), data),
    onSuccess: () => {
      toast.success("SMTP settings saved");
      qc.invalidateQueries({ queryKey: ["settings", "smtp"] });
    },
    meta: { errorMessage: "Failed to save SMTP settings" },
  });
}

export function useTestSMTP() {
  return useMutation({
    mutationFn: () => api.post(endpoints.settings.smtpTest()),
    onSuccess: () => toast.success("Test email sent"),
    meta: { errorMessage: "Test failed" },
  });
}
