export interface CronJob {
  id: string;
  project_id: string;
  name: string;
  description: string;
  cron_expression: string;
  timezone: string;
  command: string;
  image: string;
  source_type: "image" | "git";
  git_repo: string;
  git_branch: string;
  env_vars: Record<string, string>;
  cpu_limit: string;
  mem_limit: string;
  enabled: boolean;
  concurrency_policy: string;
  restart_policy: string;
  backoff_limit: number;
  active_deadline_seconds: number;
  namespace: string;
  k8s_name: string;
  last_run_at?: string;
  status: string;
  created_at: string;
}

export interface CronJobRun {
  id: string;
  cron_job_id: string;
  status: string;
  started_at: string;
  finished_at?: string;
  exit_code?: number;
  logs: string;
  trigger_type: string;
  created_at: string;
}
