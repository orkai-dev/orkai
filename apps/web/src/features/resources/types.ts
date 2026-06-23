export interface ResourceTag {
  key: string;
  value: string;
}

export interface SharedResource {
  id: string;
  org_id: string;
  name: string;
  type: string; // git_provider | registry | ssh_key | object_storage | cloud_account
  provider: string; // github | gitlab | dockerhub | ghcr | custom | aws | cloudflare
  config: Record<string, unknown>;
  status: string;
  created_at: string;
}

export interface GitRepo {
  name: string;
  full_name: string;
  clone_url: string;
  default_branch: string;
  private: boolean;
}

export interface DnsZone {
  id: string;
  name: string;
  private: boolean;
}

export interface DnsRecord {
  name: string;
  type: string;
  ttl: number;
  values: string[];
}

export interface UpsertDnsRecordInput {
  zone_id: string;
  name: string;
  type: string;
  ttl?: number;
  values: string[];
}

export interface DeleteDnsRecordInput {
  zone_id: string;
  name: string;
  type: string;
  ttl?: number;
  values: string[];
}
