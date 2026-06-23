// Managed database domain types.

export interface ManagedDB {
  id: string;
  name: string;
  database_name: string;
  engine: string;
  version: string;
  status: string;
  storage_size: string;
  cpu_limit: string;
  mem_limit: string;
  created_at: string;
  project_id: string;
  external_port: number;
  external_enabled: boolean;
  backup_enabled: boolean;
  backup_schedule: string;
  backup_s3_id?: string;
}

export interface DBVersionInfo {
  tag: string;
  label: string;
  is_recommended: boolean;
}

export interface DatabaseCredentials {
  host: string;
  port: number;
  username: string;
  password: string;
  database_name: string;
  connection_string: string;
  internal_url: string;
}

export interface DatabaseBackup {
  id: string;
  database_id: string;
  status: string;
  restore_status?: string;
  size_bytes: number;
  file_path: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}
