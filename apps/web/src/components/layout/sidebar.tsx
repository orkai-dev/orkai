import { Link, useLocation, useNavigate } from "@tanstack/react-router";
import { ChevronsUpDown, LogOut, PanelLeftClose, PanelLeftOpen, User } from "lucide-react";
import { createContext, useContext, useEffect, useState } from "react";
import { type NavDescriptor, navItemsForSection } from "@/app/registry";
import { Logo } from "@/components/logo";
import { useCurrentUser } from "@/features/auth";
import { useActiveAlerts } from "@/features/monitoring";
import { useTriggerUpgrade, useVersion } from "@/features/version";
import { clearTokens, formatApiError, resetVerification } from "@/lib/auth";
import { BRAND_NAME } from "@/lib/brand";
import { tokenStore } from "@/lib/token-store";
import { cn } from "@/lib/utils";

// ── Collapse context ────────────────────────────────────────────

const STORAGE_KEY = "orkai_sidebar_collapsed";

function getInitialCollapsed() {
  if (typeof window === "undefined") return false;
  return localStorage.getItem(STORAGE_KEY) === "true";
}

export const SidebarContext = createContext<{
  collapsed: boolean;
  toggle: () => void;
}>({ collapsed: false, toggle: () => {} });

export function useSidebarState() {
  const [collapsed, setCollapsed] = useState(getInitialCollapsed);
  const toggle = () => {
    const next = !collapsed;
    setCollapsed(next);
    localStorage.setItem(STORAGE_KEY, String(next));
  };
  return { collapsed, toggle };
}

// ── NavItem ─────────────────────────────────────────────────────

function NavItem({
  href,
  icon: Icon,
  label,
  isActive,
  collapsed,
  badge,
}: {
  href: string;
  icon: React.ElementType;
  label: string;
  isActive: boolean;
  collapsed: boolean;
  badge?: number;
}) {
  return (
    <Link
      to={href}
      title={collapsed ? label : undefined}
      className={cn(
        "group relative flex items-center transition-colors",
        collapsed ? "justify-center p-2" : "gap-3 px-3 py-2",
        "text-sm",
        isActive
          ? "bg-accent font-semibold text-foreground border-l-2 border-l-primary"
          : "font-medium text-muted-foreground hover:bg-accent/50 hover:text-foreground",
      )}
    >
      {/* Active indicator — left border handled in className */}
      <Icon
        className={cn(
          "h-[18px] w-[18px] shrink-0 transition-colors",
          isActive ? "text-primary" : "text-muted-foreground/60 group-hover:text-foreground",
        )}
        strokeWidth={isActive ? 2.25 : 1.5}
      />
      {!collapsed && <span className="flex-1 truncate">{label}</span>}
      {!collapsed && badge != null && badge > 0 && (
        <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-destructive px-1.5 text-xs font-semibold leading-none text-destructive-foreground">
          {badge}
        </span>
      )}
    </Link>
  );
}

// ── SectionLabel ────────────────────────────────────────────────

function SectionLabel({ label, collapsed }: { label: string; collapsed: boolean }) {
  if (collapsed) return <div className="mx-auto my-2 h-px w-4 bg-border/50" />;
  return <p className="mb-1 px-3 pt-3 text-label-caps text-muted-foreground/50">{label}</p>;
}

// ── Avatar helpers ───────────────────────────────────────────────

const AVATAR_EMOJI: Record<string, string> = {
  bear: "\u{1F43B}",
  cat: "\u{1F431}",
  dog: "\u{1F436}",
  fox: "\u{1F98A}",
  koala: "\u{1F428}",
  lion: "\u{1F981}",
  monkey: "\u{1F435}",
  owl: "\u{1F989}",
  panda: "\u{1F43C}",
  penguin: "\u{1F427}",
  rabbit: "\u{1F430}",
  tiger: "\u{1F42F}",
  whale: "\u{1F433}",
  wolf: "\u{1F43A}",
};

// ── VersionIndicator ────────────────────────────────────────────

function VersionIndicator({ collapsed }: { collapsed: boolean }) {
  const { data: v } = useVersion();

  if (!v) return null;

  if (collapsed) {
    if (v.update_available) {
      return (
        <div className="flex justify-center py-2">
          <span
            className="h-2 w-2 rounded-full bg-primary animate-pulse"
            title={`Update: ${v.latest}`}
          />
        </div>
      );
    }
    return null;
  }

  if (v.update_available) {
    return <UpgradeButton v={v} />;
  }

  return (
    <div className="px-4 py-2">
      <p className="text-[10px] text-muted-foreground/40">{v.current}</p>
    </div>
  );
}

// ── Upgrade Button + Modal ───────────────────────────────────────

function UpgradeButton({
  v,
}: {
  v: {
    current: string;
    latest: string;
    release_url: string;
    changelog: string;
    published_at: string;
  };
}) {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [upgradeState, setUpgradeState] = useState<"idle" | "upgrading" | "done" | "error">("idle");
  const [upgradeMsg, setUpgradeMsg] = useState("");
  const triggerUpgrade = useTriggerUpgrade();

  const upgradeCmd = "curl -sSL https://get.orkai.dev/upgrade | sudo sh";

  function copyCommand() {
    navigator.clipboard.writeText(upgradeCmd);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  // Poll upgrade status — the upgrader container writes status to a shared file,
  // and the API reads it. The upgrader handles health checks and rollback.
  useEffect(() => {
    if (upgradeState !== "upgrading") return;
    const interval = setInterval(async () => {
      try {
        const res = await fetch("/api/v1/system/upgrade/status", {
          headers: { Authorization: `Bearer ${tokenStore.getAccess() ?? ""}` },
        });
        if (!res.ok) return; // API temporarily unavailable during restart
        const data = await res.json();
        setUpgradeMsg(data.message || "");
        if (data.status === "done") {
          setUpgradeState("done");
          // Clear status file, then reload to pick up new version
          fetch("/api/v1/system/upgrade/status", {
            method: "DELETE",
            headers: { Authorization: `Bearer ${tokenStore.getAccess() ?? ""}` },
          }).catch(() => {});
          setTimeout(() => window.location.reload(), 2000);
        } else if (data.status === "error") {
          setUpgradeState("error");
          setUpgradeMsg(data.message);
        }
      } catch {
        // Connection lost during container restart — keep polling, upgrader is still running
      }
    }, 2000);
    return () => clearInterval(interval);
  }, [upgradeState]);

  function handleUpgrade() {
    setUpgradeState("upgrading");
    setUpgradeMsg("Starting upgrade...");
    triggerUpgrade.mutate(undefined, {
      onError: (err) => {
        setUpgradeState("error");
        setUpgradeMsg(formatApiError(err, "Failed to start upgrade"));
      },
    });
  }

  const changelogLines = (v.changelog || "")
    .split("\n")
    .filter((l) => l.trim().startsWith("- ") || l.trim().startsWith("* "))
    .slice(0, 5);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="group relative mx-2 mb-2 flex items-center gap-3 px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent/50 hover:text-foreground"
      >
        <span className="relative h-[18px] w-[18px] shrink-0">
          <svg
            role="img"
            aria-label="Upgrade"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="h-[18px] w-[18px] text-muted-foreground/60 group-hover:text-foreground"
          >
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
            <polyline points="7 10 12 15 17 10" />
            <line x1="12" y1="15" x2="12" y2="3" />
          </svg>
          <span className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full bg-destructive" />
        </span>
        <span className="flex-1 truncate text-left">Upgrade</span>
      </button>

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          {/* biome-ignore lint/a11y/useKeyWithClickEvents: backdrop */}
          {/* biome-ignore lint/a11y/noStaticElementInteractions: backdrop */}
          <div className="absolute inset-0 bg-black/60" onClick={() => setOpen(false)} />
          <div className="relative w-full max-w-lg border bg-popover">
            {/* Header */}
            <div className="border-b px-6 py-5">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center border border-primary/30 bg-primary/10">
                  <svg
                    role="img"
                    aria-label="Upgrade"
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="h-5 w-5 text-primary"
                  >
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="7 10 12 15 17 10" />
                    <line x1="12" y1="15" x2="12" y2="3" />
                  </svg>
                </div>
                <div>
                  <h2 className="text-lg font-bold">New Version Available</h2>
                  <p className="text-sm text-muted-foreground">
                    {v.current} → <span className="font-semibold text-primary">{v.latest}</span>
                  </p>
                </div>
              </div>
            </div>

            {/* Content */}
            <div className="space-y-5 px-6 py-5">
              {/* Changelog */}
              {changelogLines.length > 0 && (
                <div>
                  <h3 className="mb-2 text-sm font-semibold">What's new</h3>
                  <ul className="space-y-1.5">
                    {changelogLines.map((line, i) => (
                      <li key={i} className="flex items-start gap-2 text-sm text-muted-foreground">
                        <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                        {line.replace(/^[-*]\s*/, "")}
                      </li>
                    ))}
                  </ul>
                </div>
              )}

              {/* Upgrade steps */}
              <div>
                <h3 className="mb-2 text-sm font-semibold">Manual Upgrade</h3>
                <div className="space-y-2">
                  <div className="flex items-center gap-3">
                    <code className="flex-1 border bg-muted px-2.5 py-1.5 font-mono text-xs">
                      {upgradeCmd}
                    </code>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center border border-success/30 bg-success/10 text-xs font-bold text-success">
                      ✓
                    </span>
                    <span className="text-xs text-muted-foreground">
                      Handles backup, image pull, version update, and health check automatically.
                    </span>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={copyCommand}
                  className="mt-3 text-xs text-primary hover:underline"
                >
                  {copied ? "✓ Copied!" : "Copy upgrade command"}
                </button>
              </div>
            </div>

            {/* Upgrade progress */}
            {(upgradeState === "upgrading" || upgradeState === "done") && (
              <div className="border-t px-6 py-3">
                <div className="flex items-center gap-2 text-sm">
                  <svg
                    role="img"
                    aria-label="Loading"
                    className="h-4 w-4 animate-spin text-primary"
                    viewBox="0 0 24 24"
                    fill="none"
                  >
                    <circle
                      className="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      strokeWidth="4"
                    />
                    <path
                      className="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                    />
                  </svg>
                  <span className="text-muted-foreground">
                    {upgradeState === "done" ? "Upgrade complete! Reloading..." : upgradeMsg}
                  </span>
                </div>
              </div>
            )}

            {upgradeState === "error" && (
              <div className="border-t px-6 py-3 space-y-2">
                <div className="border bg-destructive/10 p-2 text-xs text-destructive">
                  {upgradeMsg}
                </div>
                <p className="text-xs text-muted-foreground">
                  You can try upgrading manually via SSH:
                </p>
                <code className="block border bg-muted px-2 py-1 font-mono text-xs">
                  {upgradeCmd}
                </code>
              </div>
            )}

            {/* Footer */}
            <div className="flex items-center justify-between border-t px-6 py-4">
              <a
                href={v.release_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-muted-foreground hover:text-primary hover:underline"
              >
                Full changelog on GitHub →
              </a>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setOpen(false)}
                  className="px-4 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent"
                  disabled={upgradeState === "upgrading"}
                >
                  Later
                </button>
                <button
                  type="button"
                  onClick={handleUpgrade}
                  disabled={upgradeState === "upgrading" || upgradeState === "done"}
                  className="bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {upgradeState === "upgrading" ? "Upgrading..." : "Upgrade Now"}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

// ── Sidebar ─────────────────────────────────────────────────────

export function Sidebar() {
  const { collapsed, toggle } = useContext(SidebarContext);
  const location = useLocation();
  const navigate = useNavigate();
  const { data: user } = useCurrentUser();
  const [menuOpen, setMenuOpen] = useState(false);
  const { data: alertsData } = useActiveAlerts();
  const alertCount = alertsData?.count ?? 0;
  const isAdmin = user?.role?.toLowerCase() === "admin";

  const renderNav = (items: NavDescriptor[]) =>
    items.map((item) => (
      <NavItem
        key={item.href}
        href={item.href}
        icon={item.icon}
        label={item.label}
        isActive={
          item.isActive ? item.isActive(location.pathname) : location.pathname === item.href
        }
        collapsed={collapsed}
        badge={item.badge === "alerts" ? alertCount : undefined}
      />
    ));

  useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [menuOpen]);

  return (
    <aside
      className={cn(
        "flex h-screen flex-col border-r bg-card transition-[width] duration-200 ease-in-out",
        collapsed ? "w-[56px]" : "w-[240px]",
      )}
    >
      {/* Header */}
      <div
        className={cn(
          "flex shrink-0 items-center border-b",
          collapsed ? "justify-center px-1.5 py-4" : "justify-between px-4 py-4",
        )}
      >
        <Link to="/dashboard" className="flex items-center gap-2">
          <Logo className="h-7 w-7 shrink-0 text-primary" />
          {!collapsed && (
            <span className="text-[17px] font-bold leading-none tracking-tight">{BRAND_NAME}</span>
          )}
        </Link>
        {!collapsed && (
          <button
            type="button"
            onClick={toggle}
            className="rounded p-1 text-muted-foreground/40 transition-colors hover:bg-accent hover:text-foreground"
            aria-label="Collapse sidebar"
          >
            <PanelLeftClose className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {collapsed && (
        <div className="flex justify-center py-2">
          <button
            type="button"
            onClick={toggle}
            className="rounded p-1 text-muted-foreground/40 transition-colors hover:bg-accent hover:text-foreground"
            aria-label="Expand sidebar"
          >
            <PanelLeftOpen className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Navigation */}
      <nav className={cn("flex-1 overflow-y-auto py-3", collapsed ? "px-1.5" : "px-2.5")}>
        {/* Main */}
        <div className="space-y-1">{renderNav(navItemsForSection("main"))}</div>

        {/* Infrastructure (admin only) */}
        {isAdmin && (
          <>
            <SectionLabel label="Infrastructure" collapsed={collapsed} />
            <div className="space-y-1">{renderNav(navItemsForSection("infrastructure"))}</div>
          </>
        )}

        {/* System (admin only) */}
        {isAdmin && (
          <>
            <SectionLabel label="System" collapsed={collapsed} />
            <div className="space-y-1">{renderNav(navItemsForSection("system"))}</div>
          </>
        )}
      </nav>

      {/* Version */}
      <VersionIndicator collapsed={collapsed} />

      {/* Footer: User */}
      <div className="relative shrink-0 border-t px-2 py-2">
        <button
          type="button"
          onClick={() => setMenuOpen(!menuOpen)}
          title={collapsed ? user?.display_name || "" : undefined}
          aria-label="User menu"
          className={cn(
            "flex w-full items-center transition-colors hover:bg-accent",
            collapsed ? "justify-center p-2" : "gap-3 px-3 py-2",
          )}
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
            {user?.avatar_url && AVATAR_EMOJI[user.avatar_url] ? (
              <span className="text-lg leading-none">{AVATAR_EMOJI[user.avatar_url]}</span>
            ) : (
              user?.display_name?.[0]?.toUpperCase() || <User className="h-4 w-4" />
            )}
          </div>
          {!collapsed && (
            <>
              <div className="min-w-0 flex-1 text-left">
                <p className="truncate text-sm font-medium leading-tight">
                  {user?.display_name || "..."}
                </p>
              </div>
              <ChevronsUpDown className="h-3 w-3 shrink-0 text-muted-foreground/40" />
            </>
          )}
        </button>

        {menuOpen && (
          <>
            {/* biome-ignore lint/a11y/useKeyWithClickEvents: backdrop */}
            {/* biome-ignore lint/a11y/noStaticElementInteractions: backdrop */}
            <div className="fixed inset-0 z-40" onClick={() => setMenuOpen(false)} />
            <div
              className={cn(
                "absolute bottom-full z-50 mb-1 overflow-hidden border bg-popover",
                collapsed ? "left-0 w-48" : "left-2 right-2",
              )}
            >
              <div className="px-3 py-3">
                <p className="text-sm font-medium">{user?.display_name}</p>
                <p className="text-xs text-muted-foreground">{user?.email}</p>
              </div>
              <div className="border-t p-1.5">
                <button
                  type="button"
                  className="flex w-full items-center gap-3 px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                  onClick={() => {
                    setMenuOpen(false);
                    navigate({ to: "/profile" });
                  }}
                >
                  <User className="h-[18px] w-[18px]" />
                  Profile
                </button>
                <button
                  type="button"
                  className="flex w-full items-center gap-3 px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                  onClick={() => {
                    clearTokens();
                    resetVerification();
                    navigate({ to: "/auth/login" });
                  }}
                >
                  <LogOut className="h-[18px] w-[18px]" />
                  Sign out
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </aside>
  );
}
