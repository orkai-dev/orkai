import { useQuery } from "@tanstack/react-query";
import type { App } from "@/features/apps/types";
import type { ManagedDB } from "@/features/databases/types";
import { useProjects } from "@/features/projects";
import { api } from "@/lib/api";

export function useDashboardApps() {
  const { data: projects } = useProjects();
  const projectIds =
    projects
      ?.map((p) => p.id)
      .sort()
      .join(",") ?? "";
  return useQuery({
    queryKey: ["dashboard", "apps", projectIds],
    queryFn: async () => {
      if (!projects?.length) return [];
      const results = await Promise.all(
        projects.map((p) =>
          api.get<{ items: App[] }>(`/api/v1/projects/${p.id}/apps`).then((r) => r.items ?? []),
        ),
      );
      return results.flat();
    },
    enabled: !!projects?.length,
  });
}

export function useDashboardDatabases() {
  const { data: projects } = useProjects();
  const projectIds =
    projects
      ?.map((p) => p.id)
      .sort()
      .join(",") ?? "";
  return useQuery({
    queryKey: ["dashboard", "databases", projectIds],
    queryFn: async () => {
      if (!projects?.length) return [];
      const results = await Promise.all(
        projects.map((p) =>
          api
            .get<{ items: ManagedDB[] }>(`/api/v1/projects/${p.id}/databases`)
            .then((r) => r.items ?? []),
        ),
      );
      return results.flat();
    },
    enabled: !!projects?.length,
  });
}
