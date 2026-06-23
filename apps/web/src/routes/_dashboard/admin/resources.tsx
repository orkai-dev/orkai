import { createFileRoute, useNavigate } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import { Plug2 } from "lucide-react";
import { useEffect } from "react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { GitProviderTab } from "./_components/resources/-git-provider-tab";
import { ResourceTab } from "./_components/resources/-resource-tab";
import {
  type ResourceType,
  TABS,
  VALID_TAB_VALUES,
} from "./_components/resources/-resources.config";

type NavItem = {
  id: ResourceType;
  label: string;
  icon: LucideIcon;
  description: string;
};

const TAB_BY_VALUE = Object.fromEntries(TABS.map((t) => [t.value, t])) as Record<
  ResourceType,
  (typeof TABS)[number]
>;

const NAV_GROUPS: { label: string; items: NavItem[] }[] = [
  {
    label: "Deploy sources",
    items: [
      {
        id: "git_provider",
        label: TAB_BY_VALUE.git_provider.label,
        icon: TAB_BY_VALUE.git_provider.icon,
        description: "GitHub, GitLab, and Gitea connections",
      },
      {
        id: "registry",
        label: TAB_BY_VALUE.registry.label,
        icon: TAB_BY_VALUE.registry.icon,
        description: "Docker Hub, GHCR, ECR, and custom registries",
      },
    ],
  },
  {
    label: "Credentials",
    items: [
      {
        id: "ssh_key",
        label: TAB_BY_VALUE.ssh_key.label,
        icon: TAB_BY_VALUE.ssh_key.icon,
        description: "Shared keys for private repositories",
      },
    ],
  },
  {
    label: "Cloud",
    items: [
      {
        id: "object_storage",
        label: TAB_BY_VALUE.object_storage.label,
        icon: TAB_BY_VALUE.object_storage.icon,
        description: "S3-compatible buckets for backups",
      },
      {
        id: "cloud_account",
        label: TAB_BY_VALUE.cloud_account.label,
        icon: TAB_BY_VALUE.cloud_account.icon,
        description: "AWS accounts for Pages and DNS",
      },
    ],
  },
];

const TAB_META: Record<ResourceType, { title: string; description: string }> = {
  git_provider: {
    title: "Git Providers",
    description: "Authorize repository access for builds, deploys, and webhooks.",
  },
  registry: {
    title: "Registries",
    description: "Store credentials for pulling and pushing container images.",
  },
  ssh_key: {
    title: "SSH Keys",
    description: "Manage shared keys used across projects and services.",
  },
  object_storage: {
    title: "Object Storage",
    description: "Configure S3-compatible storage for backups and artifacts.",
  },
  cloud_account: {
    title: "Cloud Accounts",
    description: "Connect AWS accounts for static pages, DNS, and cloud workflows.",
  },
};

export const Route = createFileRoute("/_dashboard/admin/resources")({
  component: ResourcesPage,
  validateSearch: (search: Record<string, unknown>): { tab: ResourceType } => {
    const tab = search.tab as string | undefined;
    if (tab && VALID_TAB_VALUES.includes(tab)) {
      return { tab: tab as ResourceType };
    }
    return { tab: "git_provider" };
  },
});

function ResourcesPage() {
  const { tab } = Route.useSearch();
  const navigate = useNavigate();
  const meta = TAB_META[tab];
  const activeTab = TAB_BY_VALUE[tab];

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    let handledCallback = false;

    if (params.get("setup") === "complete") {
      toast.success("GitHub App created! Now click Connect GitHub to authorize.");
      handledCallback = true;
    } else if (params.get("connected") === "true") {
      toast.success("GitHub connected successfully!");
      handledCallback = true;
    } else if (params.get("error")) {
      toast.error(`Connection failed: ${params.get("error")}`);
      handledCallback = true;
    }

    if (handledCallback) {
      navigate({
        to: "/admin/resources",
        search: { tab },
        replace: true,
      });
    }
  }, [navigate, tab]);

  const setTab = (next: ResourceType) => {
    navigate({ to: "/admin/resources", search: { tab: next }, replace: true });
  };

  return (
    <div className="space-y-6">
      <div className="relative overflow-hidden rounded-lg border bg-muted/30">
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent" />
        <div className="flex flex-col gap-3 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-label-caps text-primary">Shared infrastructure</p>
            <h1 className="mt-2 text-display-lg">Resources</h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              Credentials and integrations reused across projects — git access, registries, storage,
              and cloud accounts.
            </p>
          </div>
          <div className="flex h-10 w-10 shrink-0 items-center justify-center border border-primary/30 bg-primary/10">
            <Plug2 className="h-5 w-5 text-primary" />
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
            {tab === "git_provider" ? (
              <GitProviderTab />
            ) : (
              <ResourceTab type={tab} icon={activeTab.icon} />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
