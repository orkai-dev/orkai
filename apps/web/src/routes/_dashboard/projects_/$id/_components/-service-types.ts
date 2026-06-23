import type { App } from "@/features/apps/types";
import type { CronJob } from "@/features/cronjobs/types";
import type { ManagedDB } from "@/features/databases/types";
import type { Page } from "@/features/pages/types";
import type { Worker } from "@/features/workers/types";

export type ServiceItem =
  | { type: "app"; data: App }
  | { type: "database"; data: ManagedDB }
  | { type: "page"; data: Page }
  | { type: "worker"; data: Worker }
  | { type: "cronjob"; data: CronJob };
