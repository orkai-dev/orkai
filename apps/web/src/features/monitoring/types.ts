export interface MetricSnapshot {
  collected_at: string;
  source_type: "node" | "app";
  source_name: string;
  cpu_used: number;
  cpu_total: number;
  mem_used: number;
  mem_total: number;
  disk_used?: number;
  disk_total?: number;
  pod_count?: number;
}

export interface MetricEvent {
  id: string;
  recorded_at: string;
  event_type: "Normal" | "Warning";
  reason: string;
  message: string;
  namespace: string;
  involved_object: string;
  source_component: string;
  first_seen: string;
  last_seen: string;
  count: number;
}

export interface MetricAlert {
  id: string;
  rule_name: string;
  severity: "critical" | "warning" | "info";
  source_type: string;
  source_name: string;
  message: string;
  fired_at: string;
  resolved_at?: string;
  notified: boolean;
}
