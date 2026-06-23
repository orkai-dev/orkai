// Cloudflare Workers domain types.

export type WorkerStatus = "idle" | "deploying" | "live" | "error" | "draining";

export type WorkerPackageManager = "auto" | "npm" | "pnpm";

export interface WorkerRuntime {
  script_name?: string;
  deployed_url?: string;
  routes?: string[];
  last_deploy_id?: string;
}

export interface Worker {
  id: string;
  project_id: string;
  name: string;
  description: string;
  git_repo: string;
  git_branch: string;
  git_provider_id?: string;
  root_directory: string;
  wrangler_config: string;
  package_manager: WorkerPackageManager;
  install_command: string;
  build_command: string;
  deploy_command: string;
  build_env_vars: Record<string, string>;
  cloud_account_id?: string;
  r2_confirmed_buckets: string[];
  runtime: WorkerRuntime;
  status: WorkerStatus;
  auto_deploy: boolean;
}

export type WorkerDeploymentStatus =
  | "queued"
  | "needs_confirmation"
  | "deploying"
  | "success"
  | "failed"
  | "cancelled";

export interface WorkerR2Bucket {
  name: string;
  empty: boolean;
}

export interface WorkerDeployment {
  id: string;
  worker_id: string;
  status: WorkerDeploymentStatus;
  commit_sha: string;
  deploy_log: string;
  provider_ref: string;
  trigger_type: string;
  r2_pending?: WorkerR2Bucket[];
  started_at?: string;
  finished_at?: string;
  created_at: string;
  updated_at: string;
}
