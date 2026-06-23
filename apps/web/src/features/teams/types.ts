export interface OrgMember {
  id: string;
  email: string;
  display_name: string;
  first_name: string;
  last_name: string;
  avatar_url: string;
  role: string;
  created_at: string;
}

export interface Team {
  id: string;
  org_id: string;
  name: string;
  description: string;
  created_at: string;
}

export interface Invitation {
  id: string;
  email: string;
  role: string;
  invited_by?: string;
  expires_at: string;
  accepted_at?: string;
  created_at: string;
}
