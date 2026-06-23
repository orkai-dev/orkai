import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { Invitation, OrgMember, Team } from "./types";

export const teamKeys = {
  members: ["team", "members"] as const,
  invitations: ["team", "invitations"] as const,
};

export const teamsKeys = {
  all: ["teams"] as const,
  members: (id: string) => ["teams", id, "members"] as const,
};

export function useTeamMembers() {
  return useQuery({
    queryKey: teamKeys.members,
    queryFn: () => api.get<OrgMember[]>("/api/v1/team/members"),
  });
}

export function useTeamInvitations() {
  return useQuery({
    queryKey: teamKeys.invitations,
    queryFn: () => api.get<Invitation[]>("/api/v1/team/invitations"),
  });
}

export function useInviteMember() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { email: string; role: string }) =>
      api.post<{
        created?: boolean;
        user?: OrgMember;
        invitation?: Invitation;
        invite_url?: string;
        email_sent?: boolean;
      }>("/api/v1/team/invitations", data),
    onSuccess: (data) => {
      if (data.created) {
        toast.success("User created — they can sign in with Google");
        qc.invalidateQueries({ queryKey: teamKeys.members });
      } else {
        toast.success(data.email_sent ? "Invitation sent via email" : "Invitation created");
        qc.invalidateQueries({ queryKey: teamKeys.invitations });
      }
    },
    meta: { errorMessage: "Failed to invite" },
  });
}

export function useCancelInvitation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/team/invitations/${id}`),
    onSuccess: () => {
      toast.success("Invitation cancelled");
      qc.invalidateQueries({ queryKey: teamKeys.invitations });
    },
    meta: { errorMessage: "Failed to cancel invitation" },
  });
}

export function useUpdateMemberRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, role }: { id: string; role: string }) =>
      api.patch(`/api/v1/team/members/${id}/role`, { role }),
    onSuccess: () => {
      toast.success("Role updated");
      qc.invalidateQueries({ queryKey: teamKeys.members });
    },
    meta: { errorMessage: "Failed to update role" },
  });
}

export function useRemoveMember() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/team/members/${id}`),
    onSuccess: () => {
      toast.success("Member removed");
      qc.invalidateQueries({ queryKey: teamKeys.members });
    },
    meta: { errorMessage: "Failed to remove member" },
  });
}

export function useTeams(opts?: { enabled?: boolean }) {
  return useQuery({
    queryKey: teamsKeys.all,
    queryFn: () => api.get<Team[]>("/api/v1/teams"),
    enabled: opts?.enabled ?? true,
  });
}

export function useTeamMembersList(teamId: string, enabled = true) {
  return useQuery({
    queryKey: teamsKeys.members(teamId),
    queryFn: () => api.get<OrgMember[]>(`/api/v1/teams/${teamId}/members`),
    enabled: enabled && !!teamId,
  });
}

export function useCreateTeam() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; description: string }) =>
      api.post<Team>("/api/v1/teams", data),
    onSuccess: (_, vars) => {
      toast.success(`Team "${vars.name}" created`);
      qc.invalidateQueries({ queryKey: teamsKeys.all });
    },
    meta: { errorMessage: "Failed to create team" },
  });
}

export function useDeleteTeam() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/teams/${id}`),
    onSuccess: () => {
      toast.success("Team deleted");
      qc.invalidateQueries({ queryKey: teamsKeys.all });
    },
    meta: { errorMessage: "Failed to delete team" },
  });
}

export function useAddTeamMember(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) =>
      api.post(`/api/v1/teams/${teamId}/members`, { user_id: userId }),
    onSuccess: () => {
      toast.success("Member added to team");
      qc.invalidateQueries({ queryKey: teamsKeys.members(teamId) });
    },
    meta: { errorMessage: "Failed to add member" },
  });
}

export function useRemoveTeamMember(teamId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => api.delete(`/api/v1/teams/${teamId}/members/${userId}`),
    onSuccess: () => {
      toast.success("Member removed from team");
      qc.invalidateQueries({ queryKey: teamsKeys.members(teamId) });
    },
    meta: { errorMessage: "Failed to remove member" },
  });
}
