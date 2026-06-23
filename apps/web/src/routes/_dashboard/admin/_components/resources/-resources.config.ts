import { Cloud, GitBranch, KeyRound, Package, Server } from "lucide-react";

// ── Tab config ──────────────────────────────────────────────────

export const TABS = [
  { value: "git_provider", label: "Git Providers", icon: GitBranch },
  { value: "registry", label: "Registries", icon: Package },
  { value: "ssh_key", label: "SSH Keys", icon: KeyRound },
  { value: "object_storage", label: "Object Storage", icon: Cloud },
  { value: "cloud_account", label: "Cloud Accounts", icon: Server },
] as const;

export type ResourceType = (typeof TABS)[number]["value"];

export const VALID_TAB_VALUES = TABS.map((t) => t.value) as readonly string[];

// ── Provider options per type ───────────────────────────────────

export const PROVIDER_OPTIONS: Record<ResourceType, { value: string; label: string }[]> = {
  git_provider: [
    { value: "github", label: "GitHub" },
    { value: "gitlab", label: "GitLab" },
    { value: "gitea", label: "Gitea" },
  ],
  registry: [
    { value: "dockerhub", label: "Docker Hub" },
    { value: "ghcr", label: "GHCR" },
    { value: "ecr", label: "AWS ECR" },
    { value: "custom", label: "Custom" },
  ],
  ssh_key: [{ value: "ssh_key", label: "SSH Key" }],
  object_storage: [
    { value: "aws_s3", label: "AWS S3" },
    { value: "cloudflare_r2", label: "Cloudflare R2" },
    { value: "minio", label: "MinIO" },
    { value: "backblaze_b2", label: "Backblaze B2" },
    { value: "do_spaces", label: "DigitalOcean Spaces" },
    { value: "custom", label: "Custom (S3 Compatible)" },
  ],
  cloud_account: [
    { value: "aws", label: "AWS" },
    { value: "cloudflare", label: "Cloudflare" },
  ],
};

// ── Form field definitions per type ─────────────────────────────

export interface FieldDef {
  key: string;
  label: string;
  type: "text" | "password" | "textarea" | "select" | "tags";
  placeholder?: string;
  required?: boolean;
  options?: { value: string; label: string }[];
  help?: string;
  // showIf hides the field unless the predicate matches the current config
  // (e.g. only show static keys in access-key auth mode).
  showIf?: (config: Record<string, string>) => boolean;
}

export const FIELDS: Record<ResourceType, FieldDef[]> = {
  git_provider: [
    { key: "token", label: "Token", type: "password", placeholder: "ghp_...", required: true },
    {
      key: "api_url",
      label: "API URL",
      type: "text",
      placeholder: "https://api.github.com (optional)",
    },
    { key: "username", label: "Username", type: "text", placeholder: "Username" },
  ],
  registry: [
    {
      key: "url",
      label: "URL",
      type: "text",
      placeholder: "https://registry.example.com",
      required: true,
    },
    { key: "username", label: "Username", type: "text", placeholder: "Username", required: true },
    {
      key: "password",
      label: "Password",
      type: "password",
      placeholder: "Password",
      required: true,
    },
  ],
  ssh_key: [
    {
      key: "private_key",
      label: "Private Key",
      type: "textarea",
      placeholder: "-----BEGIN OPENSSH PRIVATE KEY-----",
      required: true,
    },
    {
      key: "passphrase",
      label: "Passphrase",
      type: "password",
      placeholder: "Optional passphrase",
    },
  ],
  object_storage: [
    {
      key: "endpoint",
      label: "Endpoint",
      type: "text",
      placeholder: "https://s3.amazonaws.com",
      required: true,
    },
    { key: "bucket", label: "Bucket", type: "text", placeholder: "my-bucket", required: true },
    {
      key: "access_key",
      label: "Access Key",
      type: "text",
      placeholder: "AKIA...",
      required: true,
    },
    {
      key: "secret_key",
      label: "Secret Key",
      type: "password",
      placeholder: "Secret key",
      required: true,
    },
    { key: "region", label: "Region", type: "text", placeholder: "us-east-1" },
  ],
  cloud_account: [
    {
      key: "auth_mode",
      label: "Authentication",
      type: "select",
      options: [
        { value: "access_key", label: "Access Key" },
        { value: "instance_role", label: "Instance Role / Env (EC2)" },
        { value: "assume_role", label: "Assume Role (STS)" },
      ],
      help: "Access Key: static IAM keys (required below). Instance Role: EC2 instance profile or AWS environment variables — no keys needed. Assume Role: STS assume-role using the instance profile or environment as base credentials (configure Role ARN below).",
    },
    {
      key: "access_key_id",
      label: "Access Key ID",
      type: "text",
      placeholder: "AKIA…",
      showIf: (c) => (c.auth_mode ?? "access_key") === "access_key",
    },
    {
      key: "secret_access_key",
      label: "Secret Access Key",
      type: "password",
      showIf: (c) => (c.auth_mode ?? "access_key") === "access_key",
    },
    {
      key: "role_arn",
      label: "Role ARN",
      type: "text",
      placeholder: "arn:aws:iam::123456789012:role/orkai",
      required: true,
      showIf: (c) => c.auth_mode === "assume_role",
      help: "The IAM role orka'i assumes. The instance profile (or AWS environment credentials) must be allowed to sts:AssumeRole this role.",
    },
    {
      key: "external_id",
      label: "External ID",
      type: "text",
      placeholder: "Optional",
      showIf: (c) => c.auth_mode === "assume_role",
      help: "Optional STS ExternalId, commonly required for cross-account roles.",
    },
    {
      key: "default_region",
      label: "Default Region",
      type: "text",
      placeholder: "us-east-1",
      required: true,
    },
    {
      key: "tags",
      label: "Resource Tags",
      type: "tags",
      help: "Applied to AWS resources created by orka'i (S3, CloudFront, ACM). Use {{env}}, {{team}}, {{project}}, {{page}} for dynamic values. Requires s3:PutBucketTagging, cloudfront:TagResource, acm:AddTagsToCertificate.",
    },
  ],
};

// Cloud account fields differ per provider. AWS uses IAM credentials; Cloudflare
// uses API tokens or global API key + email.
export const CLOUD_ACCOUNT_FIELDS_BY_PROVIDER: Record<string, FieldDef[]> = {
  aws: FIELDS.cloud_account,
  cloudflare: [
    {
      key: "auth_mode",
      label: "Authentication",
      type: "select",
      options: [
        { value: "api_token", label: "API Token" },
        { value: "api_key", label: "Global API Key + Email" },
      ],
      help: "API Token: scoped token (recommended). Global API Key: legacy key plus account email.",
    },
    {
      key: "api_token",
      label: "API Token",
      type: "password",
      placeholder: "Scoped API token",
      showIf: (c) => (c.auth_mode ?? "api_token") === "api_token",
      help: "For DNS/Pages: Zone:DNS:Edit + Zone:Zone:Read. For Workers: Account:Workers Scripts:Edit + Account:Read (the global API key is not supported for Worker deploys).",
    },
    {
      key: "api_key",
      label: "Global API Key",
      type: "password",
      showIf: (c) => c.auth_mode === "api_key",
    },
    {
      key: "email",
      label: "Account Email",
      type: "text",
      placeholder: "you@example.com",
      showIf: (c) => c.auth_mode === "api_key",
    },
    {
      key: "account_id",
      label: "Account ID",
      type: "text",
      placeholder: "Optional for DNS — required for Workers",
      help: "Required for Worker deploys; optional for DNS (limits zone listing to one account). Find it in the Cloudflare dashboard URL or account overview.",
    },
  ],
};

// Registry fields differ per provider. ECR authenticates with IAM credentials
// and fetches a short-lived token, so it has no static username/password. The
// credentials come either from static keys entered here or from a connected AWS
// cloud account (rendered as a dedicated picker in the resource sheet, not via
// these field defs).
export const REGISTRY_FIELDS_BY_PROVIDER: Record<string, FieldDef[]> = {
  ecr: [
    {
      key: "auth_mode",
      label: "Authentication",
      type: "select",
      options: [
        { value: "access_key", label: "Access Keys" },
        { value: "cloud_account", label: "AWS Cloud Account" },
      ],
      help: "Access Keys: static IAM keys entered below. AWS Cloud Account: reuse a connected AWS account's credentials (static keys, instance role, or assume role) — nothing to re-enter, and rotation propagates automatically.",
    },
    { key: "region", label: "Region", type: "text", placeholder: "us-east-1", required: true },
    {
      key: "access_key",
      label: "Access Key",
      type: "text",
      placeholder: "AKIA...",
      required: true,
      showIf: (c) => (c.auth_mode ?? "access_key") === "access_key",
    },
    {
      key: "secret_key",
      label: "Secret Key",
      type: "password",
      placeholder: "Secret key",
      required: true,
      showIf: (c) => (c.auth_mode ?? "access_key") === "access_key",
    },
  ],
};
