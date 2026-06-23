import { Check, Copy, Trash2, UserPlus, Users as UsersIcon } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useCurrentUser } from "@/features/auth";
import {
  useInviteMember,
  useRemoveMember,
  useTeamMembers,
  useUpdateMemberRole,
} from "@/features/teams";
import type { OrgMember } from "@/features/teams/types";
import { AVATAR_EMOJI, roleVariant } from "./-shared";

export function MembersTab() {
  const { data: user, isLoading: userLoading } = useCurrentUser();
  const { data: members, isLoading: membersLoading } = useTeamMembers();

  if (userLoading || membersLoading) return <LoadingScreen />;
  if (!user) return null;

  const isAdmin = user.role?.toLowerCase() === "admin";

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <UsersIcon className="h-4 w-4" /> Members
        </CardTitle>
        {isAdmin && <InviteDialog />}
      </CardHeader>
      <CardContent>
        {!members || members.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-8 text-center">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
              <UsersIcon className="h-5 w-5 text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground">No team members found</p>
          </div>
        ) : (
          <div className="space-y-1">
            {members.map((member) => (
              <MemberRow
                key={member.id}
                member={member}
                isAdmin={isAdmin}
                isCurrentUser={member.id === user.id}
              />
            ))}
          </div>
        )}
        {!isAdmin && (
          <p className="mt-4 text-xs text-muted-foreground">
            Contact an admin to manage team members.
          </p>
        )}
      </CardContent>
    </Card>
  );
}

function MemberRow({
  member,
  isAdmin,
  isCurrentUser,
}: {
  member: OrgMember;
  isAdmin: boolean;
  isCurrentUser: boolean;
}) {
  const updateRole = useUpdateMemberRole();
  const removeMember = useRemoveMember();
  const [removeOpen, setRemoveOpen] = useState(false);
  const [removeConfirm, setRemoveConfirm] = useState("");

  const showActions = isAdmin && !isCurrentUser;

  const handleRoleChange = (newRole: string) => {
    updateRole.mutate({ id: member.id, role: newRole });
  };

  const handleRemove = () => {
    if (removeConfirm !== "REMOVE") return;
    removeMember.mutate(member.id, {
      onSuccess: () => {
        setRemoveOpen(false);
        setRemoveConfirm("");
      },
    });
  };

  return (
    <div className="flex items-center gap-3 rounded-lg px-3 py-3 hover:bg-accent/50">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
        {member.avatar_url && AVATAR_EMOJI[member.avatar_url] ? (
          <span className="text-lg leading-none">{AVATAR_EMOJI[member.avatar_url]}</span>
        ) : (
          <span>{member.display_name?.[0]?.toUpperCase() || member.email[0]?.toUpperCase()}</span>
        )}
      </div>

      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium leading-tight">
          {member.display_name || `${member.first_name} ${member.last_name}`.trim() || member.email}
        </p>
        <p className="truncate text-xs text-muted-foreground">{member.email}</p>
      </div>

      <Badge variant={roleVariant(member.role)} className="shrink-0 capitalize">
        {member.role}
      </Badge>

      {showActions && (
        <div className="flex shrink-0 items-center gap-1">
          <Select value={member.role.toLowerCase()} onValueChange={handleRoleChange}>
            <SelectTrigger className="h-7 w-[100px] text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="admin">Admin</SelectItem>
              <SelectItem value="member">Member</SelectItem>
            </SelectContent>
          </Select>

          <Dialog
            open={removeOpen}
            onOpenChange={(open) => {
              setRemoveOpen(open);
              if (!open) setRemoveConfirm("");
            }}
          >
            <DialogTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Remove Member</DialogTitle>
                <DialogDescription>
                  Are you sure you want to remove{" "}
                  <strong>{member.display_name || member.email}</strong> from the team? Type{" "}
                  <strong>REMOVE</strong> to confirm.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-2">
                <Label>Confirmation</Label>
                <Input
                  value={removeConfirm}
                  onChange={(e) => setRemoveConfirm(e.target.value)}
                  placeholder="Type REMOVE"
                  className="font-mono"
                />
              </div>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline">Cancel</Button>
                </DialogClose>
                <Button
                  variant="destructive"
                  onClick={handleRemove}
                  disabled={removeConfirm !== "REMOVE" || removeMember.isPending}
                >
                  {removeMember.isPending ? "Removing..." : "Remove"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      )}
    </div>
  );
}

function InviteDialog() {
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("member");
  const [inviteUrl, setInviteUrl] = useState<string | null>(null);
  const [createdUser, setCreatedUser] = useState<{ email: string; role: string } | null>(null);
  const [copied, setCopied] = useState(false);
  const inviteMember = useInviteMember();

  const handleInvite = () => {
    inviteMember.mutate(
      { email, role },
      {
        onSuccess: (data) => {
          if (data.created && data.user) {
            setCreatedUser({ email: data.user.email, role: data.user.role });
            return;
          }
          setInviteUrl(data.invite_url ?? null);
        },
      },
    );
  };

  const handleCopy = () => {
    if (!inviteUrl) return;
    navigator.clipboard.writeText(inviteUrl);
    setCopied(true);
    toast.success("Invite URL copied");
    setTimeout(() => setCopied(false), 2000);
  };

  const handleClose = (isOpen: boolean) => {
    setOpen(isOpen);
    if (!isOpen) {
      setEmail("");
      setRole("member");
      setInviteUrl(null);
      setCreatedUser(null);
      setCopied(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogTrigger asChild>
        <Button size="sm">
          <UserPlus className="mr-1 h-3.5 w-3.5" />
          Invite Member
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Invite Member</DialogTitle>
          <DialogDescription>Send an invitation to join your team.</DialogDescription>
        </DialogHeader>

        {!inviteUrl && !createdUser ? (
          <>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>Email</Label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="teammate@example.com"
                />
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Select value={role} onValueChange={setRole}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="admin">Admin</SelectItem>
                    <SelectItem value="member">Member</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline">Cancel</Button>
              </DialogClose>
              <Button onClick={handleInvite} disabled={!email || inviteMember.isPending}>
                {inviteMember.isPending ? "Sending..." : "Send Invitation"}
              </Button>
            </DialogFooter>
          </>
        ) : createdUser ? (
          <>
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">
                <strong>{createdUser.email}</strong> was added as{" "}
                <span className="capitalize">{createdUser.role}</span>. They can sign in with Google
                using that email address.
              </p>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Done
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <div className="space-y-2">
              <Label>Invite URL</Label>
              <p className="text-xs text-muted-foreground">
                Share this link with the invitee. It will expire in 7 days.
              </p>
              <div className="flex items-center gap-2">
                <Input value={inviteUrl} readOnly className="font-mono text-xs" />
                <Button variant="outline" size="sm" onClick={handleCopy}>
                  {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                </Button>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Done
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
