import type { ElementType } from "react";
import { appsFeature } from "@/features/apps/feature";
import { clusterFeature } from "@/features/cluster/feature";
import { cronjobsFeature } from "@/features/cronjobs/feature";
import { dashboardFeature } from "@/features/dashboard/feature";
import { databasesFeature } from "@/features/databases/feature";
import { deploymentsFeature } from "@/features/deployments/feature";
import { monitoringFeature } from "@/features/monitoring/feature";
import { pagesFeature } from "@/features/pages/feature";
import { projectsFeature } from "@/features/projects/feature";
import { resourcesFeature } from "@/features/resources/feature";
import { settingsFeature } from "@/features/settings/feature";
import { teamsFeature } from "@/features/teams/feature";
import { workersFeature } from "@/features/workers/feature";

/**
 * Feature registry — the seam that lets a domain "plug in" instead of forcing
 * edits to central files. Each domain contributes a {@link FeatureModule} here;
 * cross-cutting concerns (sidebar nav, SSE cache invalidation) are derived from
 * the registry rather than hardcoded.
 *
 * Domain hooks and types live in `features/<domain>/`. Import from
 * `@/features/<domain>` — not from `@/hooks/use-*`. Each domain's manifest
 * lives in `features/<domain>/feature.ts` and is imported here. Only non-domain
 * infra (e.g. SSE) remains under `hooks/`.
 */

export type NavSection = "main" | "infrastructure" | "system";

/** Named badge sources the sidebar knows how to resolve (e.g. active alerts). */
export type BadgeSource = "alerts";

export interface NavDescriptor {
  href: string;
  label: string;
  icon: ElementType;
  section: NavSection;
  /** Custom active matcher; defaults to an exact pathname match. */
  isActive?: (pathname: string) => boolean;
  /** Named badge source resolved by the sidebar. */
  badge?: BadgeSource;
}

export interface FeatureModule {
  id: string;
  /** Sidebar entries this feature owns. */
  nav?: NavDescriptor[];
  /** PG NOTIFY table name → TanStack Query keys to invalidate on change. */
  invalidations?: Record<string, string[][]>;
}

export const features: FeatureModule[] = [
  dashboardFeature,
  projectsFeature,
  appsFeature,
  pagesFeature,
  workersFeature,
  databasesFeature,
  deploymentsFeature,
  cronjobsFeature,
  clusterFeature,
  monitoringFeature,
  resourcesFeature,
  teamsFeature,
  settingsFeature,
];

// ── Derived seams ──────────────────────────────────────────────────

/** All nav entries across features, in registration order. */
export const navItems: NavDescriptor[] = features.flatMap((f) => f.nav ?? []);

/** Nav entries for a given sidebar section, in registration order. */
export function navItemsForSection(section: NavSection): NavDescriptor[] {
  return navItems.filter((n) => n.section === section);
}

/**
 * Query keys to invalidate when a DB table changes (via PG NOTIFY → SSE).
 * Replaces the hardcoded switch that previously had to be edited per domain.
 */
export function invalidationsForTable(table: string): string[][] {
  return features.flatMap((f) => f.invalidations?.[table] ?? []);
}
