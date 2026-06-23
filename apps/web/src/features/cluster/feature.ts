import { HardDrive, Network, Server } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const clusterFeature: FeatureModule = {
  id: "cluster",
  nav: [
    {
      href: "/admin/cluster",
      label: "Cluster",
      icon: Server,
      section: "infrastructure",
      isActive: (p) => p.startsWith("/admin/cluster"),
      badge: "alerts",
    },
    {
      href: "/admin/traefik",
      label: "Traefik",
      icon: Network,
      section: "infrastructure",
      isActive: (p) => p === "/admin/traefik",
    },
    {
      href: "/admin/volumes",
      label: "Volumes",
      icon: HardDrive,
      section: "infrastructure",
      isActive: (p) => p === "/admin/volumes",
    },
  ],
  invalidations: {
    server_nodes: [["nodes"], ["cluster"]],
  },
};
