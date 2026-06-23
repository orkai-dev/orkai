import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import { Archive, Bell, Globe, Mail, Settings2, Shield } from "lucide-react";
import { cn } from "@/lib/utils";
import { AuthenticationTab } from "./_components/settings/-authentication-tab";
import { BackupTab } from "./_components/settings/-backup-tab";
import { GeneralTab } from "./_components/settings/-general-tab";
import { NotificationsTab } from "./_components/settings/-notifications-tab";
import { SMTPTab } from "./_components/settings/-smtp-tab";

const SETTINGS_TABS = ["general", "authentication", "backup", "smtp", "notifications"] as const;

type SettingsTab = (typeof SETTINGS_TABS)[number];

type NavItem = {
  id: SettingsTab;
  label: string;
  icon: LucideIcon;
  description: string;
};

const NAV_GROUPS: { label: string; items: NavItem[] }[] = [
  {
    label: "Platform",
    items: [
      {
        id: "general",
        label: "General",
        icon: Globe,
        description: "Panel domain, wildcard DNS, and TLS",
      },
    ],
  },
  {
    label: "Security & data",
    items: [
      {
        id: "authentication",
        label: "Authentication",
        icon: Shield,
        description: "OAuth and sign-in providers",
      },
      {
        id: "backup",
        label: "Backup",
        icon: Archive,
        description: "System backup schedule and storage",
      },
    ],
  },
  {
    label: "Integrations",
    items: [
      {
        id: "smtp",
        label: "SMTP",
        icon: Mail,
        description: "Outbound email delivery",
      },
      {
        id: "notifications",
        label: "Notifications",
        icon: Bell,
        description: "Alert channels and webhooks",
      },
    ],
  },
];

const TAB_META: Record<SettingsTab, { title: string; description: string }> = {
  general: {
    title: "General",
    description: "Core panel configuration — domains, TLS, and server identity.",
  },
  authentication: {
    title: "Authentication",
    description: "Configure how users sign in to the control plane.",
  },
  backup: {
    title: "Backup",
    description: "Automated system backups to object storage.",
  },
  smtp: {
    title: "SMTP",
    description: "Email relay settings for notifications and alerts.",
  },
  notifications: {
    title: "Notifications",
    description: "Delivery channels for operational alerts.",
  },
};

export const Route = createFileRoute("/_dashboard/admin/settings")({
  component: SettingsPage,
  beforeLoad: ({ search }) => {
    if (search.tab === "team") {
      throw redirect({ to: "/admin/teams" });
    }
  },
  validateSearch: (search: Record<string, unknown>): { tab: SettingsTab } => {
    const tab = search.tab as string | undefined;
    if (tab && (SETTINGS_TABS as readonly string[]).includes(tab)) {
      return { tab: tab as SettingsTab };
    }
    return { tab: "general" };
  },
});

function SettingsPage() {
  const { tab } = Route.useSearch();
  const navigate = useNavigate();
  const meta = TAB_META[tab];

  const setTab = (next: SettingsTab) => {
    navigate({ to: "/admin/settings", search: { tab: next }, replace: true });
  };

  return (
    <div className="space-y-6">
      <div className="relative overflow-hidden rounded-lg border bg-muted/30">
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent" />
        <div className="flex flex-col gap-3 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-label-caps text-primary">System configuration</p>
            <h1 className="mt-2 text-display-lg">Settings</h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              Platform-wide options for domains, authentication, backups, and messaging.
            </p>
          </div>
          <div className="flex h-10 w-10 shrink-0 items-center justify-center border border-primary/30 bg-primary/10">
            <Settings2 className="h-5 w-5 text-primary" />
          </div>
        </div>
      </div>

      <div className="flex flex-col gap-6 lg:flex-row lg:items-start">
        <nav className="flex shrink-0 gap-4 overflow-x-auto lg:w-56 lg:flex-col lg:overflow-visible">
          {NAV_GROUPS.map((group) => (
            <div key={group.label} className="min-w-[160px] shrink-0 lg:min-w-0">
              <p className="mb-1.5 px-1 text-label-caps text-muted-foreground/70">{group.label}</p>
              <div className="space-y-1">
                {group.items.map(({ id, label, icon: Icon, description }) => {
                  const active = tab === id;
                  return (
                    <button
                      key={id}
                      type="button"
                      onClick={() => setTab(id)}
                      className={cn(
                        "flex w-full flex-col rounded-md border px-3 py-2.5 text-left transition-colors",
                        active
                          ? "border-primary/30 bg-primary/5"
                          : "border-transparent hover:border-border hover:bg-muted/50",
                      )}
                    >
                      <span className="flex items-center gap-2 text-sm font-medium">
                        <Icon
                          className={cn(
                            "h-4 w-4",
                            active ? "text-primary" : "text-muted-foreground",
                          )}
                        />
                        {label}
                      </span>
                      <span className="mt-0.5 hidden text-xs text-muted-foreground lg:block">
                        {description}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>

        <div className="min-w-0 flex-1 rounded-lg border bg-card">
          <div className="border-b px-5 py-4">
            <h2 className="text-sm font-semibold">{meta.title}</h2>
            <p className="mt-0.5 text-xs text-muted-foreground">{meta.description}</p>
          </div>
          <div className="p-5">
            {tab === "general" && <GeneralTab />}
            {tab === "authentication" && <AuthenticationTab />}
            {tab === "backup" && <BackupTab />}
            {tab === "smtp" && <SMTPTab />}
            {tab === "notifications" && <NotificationsTab />}
          </div>
        </div>
      </div>
    </div>
  );
}
