export interface SystemBackupConfig {
  enabled: boolean;
  s3_id: string;
  schedule: string;
  path: string;
  retention: number;
}

export interface S3BackupFile {
  key: string;
  file_name: string;
  size_bytes: number;
  last_modified: string;
}

export interface SystemBackup {
  id: string;
  status: string;
  size_bytes: number;
  file_name: string;
  s3_bucket: string;
  s3_path: string;
  error?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}
