export type Settings = Record<string, string>;

export interface DomainVerification {
  domain: string;
  dns: "ok" | "failed" | "wrong_ip";
  dns_ip?: string;
  dns_message?: string;
  dns_warning?: string;
  reachable?: boolean;
  reachable_message?: string;
  cert?: "valid" | "self_signed" | "cloudflare" | "none" | "unknown";
  cert_message?: string;
  cert_issuer?: string;
  cert_expiry?: string;
  cert_days?: number;
}
