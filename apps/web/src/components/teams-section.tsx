import { FolderKanban, Plus, Trash2, UserMinus, Users as UsersIcon } from "lucide-react";
import { useState } from "react";
import { LoadingScreen } from "@/components/loading-screen";
import type { BadgeProps } from "@/components/ui/badge";
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
import {
  useAddTeamMember,
  useCreateTeam,
  useDeleteTeam,
  useRemoveTeamMember,
  useTeamMembersList,
  useTeams,
} from "@/features/teams";
import type { OrgMember, Team } from "@/features/teams/types";

function roleVariant(role: string): NonNullable<BadgeProps["variant"]> {
  switch (role.toLowerCase()) {
    case "admin":
      return "outline";
    default:
      return "secondary";
  }
}

export function TeamsSection({ members }: { members: OrgMember[] }) {
  const { data: teams, isLoading } = useTeams();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <FolderKanban className="h-4 w-4" /> Teams
        </CardTitle>
        <CreateTeamDialog />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <LoadingScreen />
        ) : !teams || teams.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-8 text-center">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
              <FolderKanban className="h-5 w-5 text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground">
              No teams yet. Create a team to scope project access.
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {teams.map((team) => (
              <TeamRow key={team.id} team={team} orgMembers={members} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function TeamRow({ team, orgMembers }: { team: Team; orgMembers: OrgMember[] }) {
  const [open, setOpen] = useState(false);
  const deleteTeam = useDeleteTeam();
  const { data: teamMembers } = useTeamMembersList(team.id, open);
  const addMember = useAddTeamMember(team.id);
  const removeMember = useRemoveTeamMember(team.id);

  const memberIds = new Set((teamMembers ?? []).map((m) => m.id));
  // Owners/admins already have org-wide access; only members need to be added.
  const candidates = orgMembers.filter(
    (m) => m.role.toLowerCase() === "member" && !memberIds.has(m.id),
  );

  return (
    <div className="rounded-lg border">
      <div className="flex items-center gap-3 px-3 py-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
          <FolderKanban className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium leading-tight">{team.name}</p>
          {team.description && (
            <p className="truncate text-xs text-muted-foreground">{team.description}</p>
          )}
        </div>
        <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setOpen(!open)}>
          <UsersIcon className="mr-1 h-3.5 w-3.5" />
          Members
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
          onClick={() => deleteTeam.mutate(team.id)}
          disabled={deleteTeam.isPending}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>

      {open && (
        <div className="space-y-3 border-t bg-muted/30 px-3 py-3">
          {/* Current members */}
          <div>
            <p className="mb-1 text-xs font-medium text-muted-foreground">Team members</p>
            {!teamMembers || teamMembers.length === 0 ? (
              <p className="text-xs text-muted-foreground">No members in this team yet.</p>
            ) : (
              <div className="space-y-1">
                {teamMembers.map((m) => (
                  <div
                    key={m.id}
                    className="flex items-center gap-2 rounded px-2 py-1.5 hover:bg-accent/50"
                  >
                    <span className="min-w-0 flex-1 truncate text-sm">
                      {m.display_name || m.email}
                    </span>
                    <Badge variant={roleVariant(m.role)} className="shrink-0 capitalize">
                      {m.role}
                    </Badge>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                      onClick={() => removeMember.mutate(m.id)}
                      disabled={removeMember.isPending}
                    >
                      <UserMinus className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Add member */}
          <div>
            <p className="mb-1 text-xs font-medium text-muted-foreground">Add member</p>
            {candidates.length === 0 ? (
              <p className="text-xs text-muted-foreground">
                No members available to add. Invite members with the "member" role first.
              </p>
            ) : (
              <Select onValueChange={(userId) => addMember.mutate(userId)}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="Select a member to add" />
                </SelectTrigger>
                <SelectContent>
                  {candidates.map((m) => (
                    <SelectItem key={m.id} value={m.id}>
                      {m.display_name || m.email}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function CreateTeamDialog() {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const createTeam = useCreateTeam();

  const handleClose = (isOpen: boolean) => {
    setOpen(isOpen);
    if (!isOpen) {
      setName("");
      setDescription("");
    }
  };

  const handleCreate = () => {
    createTeam.mutate(
      { name: name.trim(), description: description.trim() },
      { onSuccess: () => handleClose(false) },
    );
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-1 h-3.5 w-3.5" />
          New Team
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New Team</DialogTitle>
          <DialogDescription>
            Teams group members and own projects. Members can only access their teams' projects.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="platform-team"
            />
          </div>
          <div className="space-y-2">
            <Label>Description</Label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional context"
            />
          </div>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">Cancel</Button>
          </DialogClose>
          <Button onClick={handleCreate} disabled={!name.trim() || createTeam.isPending}>
            {createTeam.isPending ? "Creating..." : "Create Team"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
