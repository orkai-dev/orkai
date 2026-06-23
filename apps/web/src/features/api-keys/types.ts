export type APIKeyRole = "admin" | "member";

export interface APIKey {
  id: string;
  org_id: string;
  user_id: string;
  name: string;
  key_prefix: string;
  role: APIKeyRole;
  last_used_at?: string;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateAPIKeyInput {
  name: string;
  role?: APIKeyRole;
  expires_at?: string;
}

export interface CreateAPIKeyResult {
  key: string;
  api_key: APIKey;
}
