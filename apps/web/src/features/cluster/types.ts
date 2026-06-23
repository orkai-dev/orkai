export interface NodeInfo {
  name: string;
  ip: string;
  status: string;
  roles: string[];
  pool?: string;
  version: string;
  os: string;
  arch: string;
  resources: {
    cpu_used: string;
    cpu_total: string;
    mem_used: string;
    mem_total: string;
    storage_used: string;
    storage_total: string;
  };
}

export interface ClusterMetrics {
  nodes: number;
  total_pods: number;
  running_pods: number;
  resources: {
    cpu_used: string;
    cpu_total: string;
    mem_used: string;
    mem_total: string;
  };
}

export interface ClusterEvent {
  type: string;
  reason: string;
  message: string;
  namespace: string;
  involved_object: string;
  count: number;
  first_seen: string;
  last_seen: string;
}

export interface PVCInfo {
  name: string;
  namespace: string;
  status: string;
  capacity: string;
  storage_class: string;
  volume_name: string;
  used_by?: string[];
}

export interface StorageClassInfo {
  name: string;
  provisioner: string;
  is_default: boolean;
  allow_expansion: boolean;
}

export interface NamespaceInfo {
  name: string;
  status: string;
  pod_count: number;
  svc_count: number;
}

export interface NodeMetrics {
  name: string;
  cpu_used: string;
  cpu_total: string;
  mem_used: string;
  mem_total: string;
  pod_count: number;
}

export interface ClusterTopology {
  nodes: TopologyNode[];
  deployments: TopologyDeployment[];
  pods: TopologyPod[];
  services: TopologyService[];
  ingresses: TopologyIngress[];
}

export interface TopologyNode {
  name: string;
  status: string;
  ip: string;
  roles: string;
}

export interface TopologyDeployment {
  name: string;
  namespace: string;
  ready: number;
  desired: number;
  app_id?: string;
}

export interface TopologyPod {
  name: string;
  namespace: string;
  phase: string;
  node: string;
  ip: string;
  app_id?: string;
  deployment?: string;
}

export interface TopologyService {
  name: string;
  namespace: string;
  type: string;
  cluster_ip: string;
  ports: string;
  app_id?: string;
}

export interface TopologyIngress {
  name: string;
  namespace: string;
  host: string;
  service: string;
  app_id?: string;
}

export interface HelmRelease {
  name: string;
  namespace: string;
  chart: string;
  revision: string;
  status: string;
  updated: string;
}

export interface CleanupStats {
  evicted_pods: number;
  evicted_pod_names: string[];
  failed_pods: number;
  failed_pod_names: string[];
  completed_pods: number;
  completed_pod_names: string[];
  stale_replicasets: number;
  stale_rs_names: string[];
  completed_jobs: number;
  completed_job_names: string[];
  unbound_pvcs: number;
  unbound_pvc_names: string[];
  orphan_ingresses: number;
  orphan_ingress_names: string[];
}

export interface CleanupResult {
  deleted: number;
  message: string;
}

export interface DaemonSetInfo {
  name: string;
  namespace: string;
  desired_scheduled: number;
  current_scheduled: number;
  ready: number;
  node_selector: string;
  images: string;
  created_at: string;
}

export interface ServerNode {
  id: string;
  name: string;
  host: string;
  port: number;
  ssh_user: string;
  auth_type: string; // password | ssh_key
  ssh_key_id?: string;
  role: string; // worker | server
  status: string; // pending | initializing | ready | error | offline
  status_msg: string;
  k8s_node_name: string;
  created_at: string;
}

export interface TraefikConfig {
  yaml: string;
}
