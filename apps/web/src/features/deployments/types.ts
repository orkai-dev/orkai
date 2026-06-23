export interface Deployment {
  id: string;
  app_id: string;
  project_id?: string;
  status: string;
  commit_sha: string;
  trigger_type: string;
  image: string;
  build_log?: string;
  app_name?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}
