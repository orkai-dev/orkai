export interface UserInfo {
  id: string;
  display_name: string;
  first_name: string;
  last_name: string;
  email: string;
  role: string;
  avatar_url?: string;
  two_fa_enabled: boolean;
}

export interface AuthResponse {
  user: UserInfo;
  access_token: string;
  refresh_token: string;
}
