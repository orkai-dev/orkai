import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { endpoints } from "@/lib/endpoints";
import type { UserInfo } from "./types";

export const userKeys = {
  me: ["auth", "me"] as const,
};

export function useCurrentUser() {
  return useQuery({
    queryKey: userKeys.me,
    queryFn: () => api.get<UserInfo>(endpoints.auth.me()),
    staleTime: 5 * 60_000,
    retry: false,
  });
}

export function useUpdateProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      first_name?: string;
      last_name?: string;
      display_name?: string;
      avatar_url?: string;
    }) => api.patch<UserInfo>(endpoints.auth.profile(), data),
    onSuccess: () => {
      toast.success("Profile updated");
      qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
    meta: { errorMessage: "Failed to update profile" },
  });
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (data: { current_password: string; new_password: string }) =>
      api.post(endpoints.auth.changePassword(), data),
    onSuccess: () => toast.success("Password changed"),
    meta: { errorMessage: "Failed to change password" },
  });
}

export function useAvatars() {
  return useQuery({
    queryKey: ["auth", "avatars"],
    queryFn: () => api.get<string[]>(endpoints.auth.avatars()),
  });
}

export function useSetup2FA() {
  return useMutation({
    mutationFn: () => api.post<{ secret: string; qr_code: string }>(endpoints.auth.twoFASetup()),
  });
}

export function useVerify2FA() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (code: string) => api.post(endpoints.auth.twoFAVerify(), { code }),
    onSuccess: () => {
      toast.success("2FA enabled");
      qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
    meta: { errorMessage: "Invalid code" },
  });
}

export function useSetupStatus() {
  return useQuery({
    queryKey: ["auth", "setup-status"],
    queryFn: () => api.get<{ initialized: boolean }>(endpoints.auth.setupStatus()),
    staleTime: 0,
    gcTime: 0,
  });
}

export type OAuthProviders = {
  google?: { enabled: boolean };
  password_enabled?: boolean;
};

export function useOAuthProviders() {
  return useQuery({
    queryKey: ["auth", "providers"],
    queryFn: () => api.get<OAuthProviders>(endpoints.auth.providers()),
    staleTime: 60_000,
  });
}

export function useDisable2FA() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (code: string) => api.post(endpoints.auth.twoFADisable(), { code }),
    onSuccess: () => {
      toast.success("2FA disabled");
      qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
    meta: { errorMessage: "Invalid code" },
  });
}
