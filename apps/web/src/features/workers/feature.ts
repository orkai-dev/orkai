import { Zap } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const workersFeature: FeatureModule = {
  id: "workers",
  nav: [
    {
      href: "/workers",
      label: "Cloudflare Workers",
      icon: Zap,
      section: "main",
      isActive: (p) =>
        p === "/workers" ||
        (p.startsWith("/projects") && (p.includes("/workers/") || p.endsWith("/workers"))),
    },
  ],
  invalidations: {
    workers: [["workers"], ["projects"]],
    worker_deployments: [["workers"]],
  },
};
