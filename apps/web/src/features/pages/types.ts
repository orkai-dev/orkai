// Static-site "Pages" domain types.

export type PageProvider = "aws_cloudfront" | "cloudflare_pages";

export type PageStatus = "idle" | "deploying" | "live" | "error" | "draining";

export interface PageValidationRecord {
  name?: string;
  type?: string;
  value?: string;
}

export interface PageRuntime {
  bucket_name?: string;
  distribution_id?: string;
  oac_id?: string;
  default_url?: string;
  certificate_arn?: string;
  cert_status?: string;
  validation_record?: PageValidationRecord;
  alias_target?: string;
  cf_project_name?: string;
  cf_project_id?: string;
}

export type PagePackageManager = "auto" | "npm" | "pnpm";

export interface Page {
  id: string;
  project_id: string;
  name: string;
  description: string;
  git_repo: string;
  git_branch: string;
  git_provider_id?: string;
  publish_path: string;
  build_enabled: boolean;
  package_manager: PagePackageManager;
  install_command: string;
  build_command: string;
  output_dir: string;
  root_directory: string;
  node_version: string;
  build_env_vars: Record<string, string>;
  provider: PageProvider;
  cloud_account_id?: string;
  region: string;
  custom_domain: string;
  manage_dns: boolean;
  dns_account_id?: string;
  dns_zone_id: string;
  runtime: PageRuntime;
  status: PageStatus;
  auto_deploy: boolean;
}

export type PageDeploymentStatus = "queued" | "deploying" | "success" | "failed" | "cancelled";

export interface PageDeployment {
  id: string;
  page_id: string;
  status: PageDeploymentStatus;
  commit_sha: string;
  deploy_log: string;
  provider_ref: string;
  trigger_type: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  updated_at: string;
}
