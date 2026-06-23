// Application domain types. K8s runtime primitives (`AppStatus`, `ContainerStatus`,
// `PodInfo`, `PodEvent`) live in `@/shared/types/k8s` because they are shared
// with other domains (e.g. databases).

export interface HealthCheck {
  path: string;
  port: number;
  initial_delay_seconds: number;
  period_seconds: number;
  timeout_seconds: number;
  failure_threshold: number;
  type: string; // http | tcp | exec
  command?: string;
}

export interface AutoscalingConfig {
  enabled: boolean;
  min_replicas: number;
  max_replicas: number;
  cpu_target: number; // percentage
  mem_target: number; // percentage
}

export interface VolumeMount {
  name: string;
  mount_path: string;
  size: string;
  storage_class?: string;
  pvc_name?: string;
}

export interface TargetCapabilities {
  target_id: string;
  kind: string;
  capabilities: string[];
  default_storage_class?: string;
  allowed_storage_classes?: string[];
}

export interface DeployStrategyConfig {
  max_surge: string;
  max_unavailable: string;
}

export interface PortMapping {
  container_port: number;
  service_port: number;
  protocol: "tcp" | "udp";
}

export interface App {
  id: string;
  name: string;
  description: string;
  is_critical: boolean;
  source_type: "image" | "git";
  docker_image: string;
  registry_id?: string;
  git_repo: string;
  git_branch: string;
  build_type: string;
  dockerfile: string;
  status: string;
  replicas: number;
  cpu_limit: string;
  mem_limit: string;
  env_vars: Record<string, string>;
  ports: PortMapping[];
  // Advanced config
  health_check: HealthCheck;
  autoscaling: AutoscalingConfig;
  cpu_request: string;
  mem_request: string;
  volumes: VolumeMount[];
  deploy_strategy: string;
  deploy_strategy_config: DeployStrategyConfig;
  termination_grace_period: number;
  build_env_vars: Record<string, string>;
  build_context: string;
  watch_paths: string[];
  no_cache: boolean;
  node_pool: string;
  auto_deploy: boolean;
  // K8s mapping
  namespace: string;
  k8s_name: string;
  project_id: string;
  created_at: string;
}

export interface WebhookConfig {
  webhook_url: string;
  secret: string;
  auto_deploy: boolean;
}

export interface Domain {
  id: string;
  host: string;
  tls: boolean;
  auto_cert: boolean;
  force_https: boolean;
  cert_expiry?: string;
  ingress_ready: boolean;
}
