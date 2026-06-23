import { Clock, Mail, X } from "lucide-react";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useCurrentUser } from "@/features/auth";
import { useCancelInvitation, useTeamInvitations } from "@/features/teams";
import type { Invitation } from "@/features/teams/types";
import { relativeExpiry, roleVariant } from "./-shared";

export function InvitationsTab() {
  const { data: user, isLoading: userLoading } = useCurrentUser();
  const { data: invitations, isLoading: invitationsLoading } = useTeamInvitations();

  if (userLoading) return <LoadingScreen />;
  if (!user) return null;

  const isAdmin = user.role?.toLowerCase() === "admin";
  const pendingInvitations = (invitations ?? []).filter((inv) => !inv.accepted_at);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Mail className="h-4 w-4" /> Pending Invitations
        </CardTitle>
      </CardHeader>
      <CardContent>
        {invitationsLoading ? (
          <LoadingScreen />
        ) : pendingInvitations.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-8 text-center">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
              <Mail className="h-5 w-5 text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground">No pending invitations</p>
          </div>
        ) : (
          <div className="space-y-1">
            {pendingInvitations.map((inv) => (
              <InvitationRow key={inv.id} invitation={inv} isAdmin={isAdmin} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function InvitationRow({ invitation, isAdmin }: { invitation: Invitation; isAdmin: boolean }) {
  const cancelInvitation = useCancelInvitation();

  return (
    <div className="flex items-center gap-3 rounded-lg px-3 py-3 hover:bg-accent/50">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted">
        <Mail className="h-4 w-4 text-muted-foreground" />
      </div>

      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium leading-tight">{invitation.email}</p>
      </div>

      <Badge variant={roleVariant(invitation.role)} className="shrink-0 capitalize">
        {invitation.role}
      </Badge>

      <span className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
        <Clock className="h-3 w-3" />
        {relativeExpiry(invitation.expires_at)}
      </span>

      {isAdmin && (
        <Button
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 text-xs text-muted-foreground hover:text-destructive"
          onClick={() => cancelInvitation.mutate(invitation.id)}
          disabled={cancelInvitation.isPending}
        >
          <X className="mr-1 h-3 w-3" />
          Cancel
        </Button>
      )}
    </div>
  );
}
