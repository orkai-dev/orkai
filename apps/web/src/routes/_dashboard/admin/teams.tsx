import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Mail, UserPlus, Users } from "lucide-react";
import { LoadingScreen } from "@/components/loading-screen";
import { TeamsSection } from "@/components/teams-section";
import { useCurrentUser } from "@/features/auth";
import { useTeamMembers } from "@/features/teams";
import { cn } from "@/lib/utils";
import { InvitationsTab } from "./_components/teams/-invitations-tab";
import { MembersTab } from "./_components/teams/-members-tab";

const TEAMS_TABS = ["members", "teams", "invitations"] as const;

type TeamsTab = (typeof TEAMS_TABS)[number];

const TAB_CONFIG: {
  id: TeamsTab;
  label: string;
  icon: typeof Users;
  description: string;
}[] = [
  {
    id: "members",
    label: "Members",
    icon: Users,
    description: "Organization members, roles, and access",
  },
  {
    id: "teams",
    label: "Project Teams",
    icon: UserPlus,
    description: "Groups that scope project access",
  },
  {
    id: "invitations",
    label: "Invitations",
    icon: Mail,
    description: "Pending invites awaiting acceptance",
  },
];

const TAB_META: Record<TeamsTab, { title: string; description: string }> = {
  members: {
    title: "Members",
    description: "Manage organization members and their roles.",
  },
  teams: {
    title: "Project Teams",
    description: "Create teams and assign members to scope project access.",
  },
  invitations: {
    title: "Invitations",
    description: "Review and cancel pending member invitations.",
  },
};

export const Route = createFileRoute("/_dashboard/admin/teams")({
  component: TeamsPage,
  validateSearch: (search: Record<string, unknown>): { tab: TeamsTab } => {
    const tab = search.tab as string | undefined;
    if (tab && (TEAMS_TABS as readonly string[]).includes(tab)) {
      return { tab: tab as TeamsTab };
    }
    return { tab: "members" };
  },
});

function TeamsPage() {
  const { tab } = Route.useSearch();
  const navigate = useNavigate();
  const { data: user, isLoading: userLoading } = useCurrentUser();
  const { data: members, isLoading: membersLoading } = useTeamMembers();
  const meta = TAB_META[tab];

  const setTab = (next: TeamsTab) => {
    navigate({ to: "/admin/teams", search: { tab: next }, replace: true });
  };

  if (userLoading || membersLoading) return <LoadingScreen />;

  const isAdmin = user?.role?.toLowerCase() === "admin";
  const visibleTabs = TAB_CONFIG.filter((t) => t.id !== "teams" || isAdmin);

  return (
    <div className="space-y-6">
      <div className="relative overflow-hidden rounded-lg border bg-muted/30">
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent" />
        <div className="flex flex-col gap-3 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-label-caps text-primary">Organization</p>
            <h1 className="mt-2 text-display-lg">Teams</h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              Manage members, project teams, and invitations for your organization.
            </p>
          </div>
          <div className="flex h-10 w-10 shrink-0 items-center justify-center border border-primary/30 bg-primary/10">
            <Users className="h-5 w-5 text-primary" />
          </div>
        </div>
      </div>

      <div className="flex flex-col gap-6">
        <nav className="flex gap-2 overflow-x-auto border-b pb-px">
          {visibleTabs.map(({ id, label, icon: Icon }) => {
            const active = tab === id;
            return (
              <button
                key={id}
                type="button"
                onClick={() => setTab(id)}
                className={cn(
                  "flex shrink-0 items-center gap-2 border-b-2 px-3 py-2 text-sm font-medium transition-colors",
                  active
                    ? "border-primary text-primary"
                    : "border-transparent text-muted-foreground hover:text-foreground",
                )}
              >
                <Icon className="h-4 w-4" />
                {label}
              </button>
            );
          })}
        </nav>

        <div className="rounded-lg border bg-card">
          <div className="border-b px-5 py-4">
            <h2 className="text-sm font-semibold">{meta.title}</h2>
            <p className="mt-0.5 text-xs text-muted-foreground">{meta.description}</p>
          </div>
          <div className="p-5">
            {tab === "members" && <MembersTab />}
            {tab === "teams" && isAdmin && <TeamsSection members={members ?? []} />}
            {tab === "invitations" && <InvitationsTab />}
          </div>
        </div>
      </div>
    </div>
  );
}
