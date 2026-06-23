export interface AppStatus {
  phase: string;
  ready_replicas: number;
  desired_replicas: number;
}

export interface ContainerStatus {
  name: string;
  ready: boolean;
  restart_count: number;
  state: string; // running | waiting | terminated
  reason: string;
}

export interface PodInfo {
  name: string;
  namespace: string;
  phase: string;
  node: string;
  ip: string;
  started_at: string;
  restart_count: number;
  ready: boolean;
  containers: ContainerStatus[];
  resources: {
    cpu_used: string;
    cpu_total: string;
    mem_used: string;
    mem_total: string;
  };
}

export interface PodEvent {
  type: string; // Normal | Warning
  reason: string;
  message: string;
  count: number;
  first_seen: string;
  last_seen: string;
}
