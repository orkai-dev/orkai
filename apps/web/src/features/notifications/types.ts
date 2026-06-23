export interface NotificationChannel {
  id: string;
  type: "email" | "telegram" | "discord" | "slack" | "google_chat";
  enabled: boolean;
  config: Record<string, unknown>;
}

export interface NotifyEventInfo {
  key: string;
  label: string;
  category: string;
}

export interface SMTPConfig {
  host: string;
  port: string;
  user: string;
  password: string;
  from: string;
  enabled: boolean;
}
