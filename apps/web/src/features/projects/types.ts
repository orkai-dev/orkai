import type { Team } from "@/features/teams/types";

export interface ResourceQuotaConfig {
  cpu_limit?: string;
  mem_limit?: string;
  pod_limit?: number;
  pvc_limit?: number;
  storage_limit?: string;
}

export type Environment = "prod" | "testing" | "sandbox" | "qa" | "poc" | "development";

export interface Project {
  id: string;
  team_id: string;
  team?: Team;
  name: string;
  namespace: string;
  description: string;
  environment: Environment;
  env_vars: Record<string, string>;
  resource_quota: ResourceQuotaConfig;
  network_policy_enabled: boolean;
  service_account: string;
  created_at: string;
}
