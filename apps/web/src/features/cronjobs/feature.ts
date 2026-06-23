import { Clock } from "lucide-react";
import type { FeatureModule } from "@/app/registry";

export const cronjobsFeature: FeatureModule = {
  id: "cronjobs",
  nav: [
    {
      href: "/cronjobs",
      label: "CronJobs",
      icon: Clock,
      section: "main",
      isActive: (p) => p.startsWith("/cronjobs"),
    },
  ],
  invalidations: {
    cron_jobs: [["cronjobs"], ["projects"]],
    cron_job_runs: [["cronjobs"]],
  },
};
