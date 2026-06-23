import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useMonitoringSnapshots } from "@/features/monitoring";

export function NodeTrendCharts() {
  const { data: snapshots } = useMonitoringSnapshots("node", undefined, 60);

  if (!snapshots || snapshots.length < 2) return null;

  // Group by node, then build time series
  const nodeMap = new Map<string, typeof snapshots>();
  for (const s of snapshots) {
    const arr = nodeMap.get(s.source_name) || [];
    arr.push(s);
    nodeMap.set(s.source_name, arr);
  }

  // Aggregate all nodes into total
  const timeMap = new Map<
    string,
    { time: string; cpu: number; cpuTotal: number; mem: number; memTotal: number }
  >();
  for (const s of snapshots) {
    const t = new Date(s.collected_at).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    });
    const key = s.collected_at;
    const existing = timeMap.get(key) || { time: t, cpu: 0, cpuTotal: 0, mem: 0, memTotal: 0 };
    existing.cpu += s.cpu_used;
    existing.cpuTotal += s.cpu_total;
    existing.mem += Math.round(s.mem_used / (1024 * 1024));
    existing.memTotal += Math.round(s.mem_total / (1024 * 1024));
    timeMap.set(key, existing);
  }
  const chartData = [...timeMap.values()];

  return (
    <div className="mt-6 grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader className="pb-0">
          <div className="flex items-center justify-between">
            <CardTitle className="text-xs font-medium">Cluster CPU</CardTitle>
            <span className="text-xs text-muted-foreground">last hour · millicores</span>
          </div>
        </CardHeader>
        <CardContent className="pt-2">
          <div className="select-none">
            <ResponsiveContainer width="100%" height={140}>
              <AreaChart data={chartData} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="grad-cluster-cpu" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#6d5cdb" stopOpacity={0.15} />
                    <stop offset="100%" stopColor="#6d5cdb" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid
                  strokeDasharray="3 3"
                  vertical={false}
                  stroke="#9ca3af"
                  strokeOpacity={0.12}
                />
                <XAxis
                  dataKey="time"
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 9, fill: "#9ca3af" }}
                  interval="preserveStartEnd"
                  minTickGap={50}
                />
                <YAxis
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 9, fill: "#9ca3af" }}
                  tickFormatter={(v) => (v >= 1000 ? `${(v / 1000).toFixed(1)}` : `${v}m`)}
                  width={36}
                />
                <Tooltip
                  cursor={false}
                  contentStyle={{
                    fontSize: 11,
                    borderRadius: 8,
                    border: "1px solid var(--color-border)",
                    background: "var(--color-popover)",
                  }}
                  formatter={(v: number, name: string) => [
                    `${v}m`,
                    name === "cpu" ? "Used" : "Total",
                  ]}
                />
                <Area
                  type="monotone"
                  dataKey="cpuTotal"
                  stroke="#9ca3af"
                  strokeOpacity={0.25}
                  fill="none"
                  strokeDasharray="4 2"
                  strokeWidth={1}
                  dot={false}
                  activeDot={false}
                  isAnimationActive={false}
                />
                <Area
                  type="monotone"
                  dataKey="cpu"
                  stroke="#6d5cdb"
                  fill="url(#grad-cluster-cpu)"
                  strokeWidth={1.5}
                  dot={false}
                  activeDot={false}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-0">
          <div className="flex items-center justify-between">
            <CardTitle className="text-xs font-medium">Cluster Memory</CardTitle>
            <span className="text-xs text-muted-foreground">last hour · MiB</span>
          </div>
        </CardHeader>
        <CardContent className="pt-2">
          <div className="select-none">
            <ResponsiveContainer width="100%" height={140}>
              <AreaChart data={chartData} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="grad-cluster-mem" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#8b5cf6" stopOpacity={0.15} />
                    <stop offset="100%" stopColor="#8b5cf6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid
                  strokeDasharray="3 3"
                  vertical={false}
                  stroke="#9ca3af"
                  strokeOpacity={0.12}
                />
                <XAxis
                  dataKey="time"
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 9, fill: "#9ca3af" }}
                  interval="preserveStartEnd"
                  minTickGap={50}
                />
                <YAxis
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 9, fill: "#9ca3af" }}
                  tickFormatter={(v) => (v >= 1024 ? `${(v / 1024).toFixed(1)}G` : `${v}`)}
                  width={36}
                />
                <Tooltip
                  cursor={false}
                  contentStyle={{
                    fontSize: 11,
                    borderRadius: 8,
                    border: "1px solid var(--color-border)",
                    background: "var(--color-popover)",
                  }}
                  formatter={(v: number, name: string) => [
                    `${v} Mi`,
                    name === "mem" ? "Used" : "Total",
                  ]}
                />
                <Area
                  type="monotone"
                  dataKey="memTotal"
                  stroke="#9ca3af"
                  strokeOpacity={0.25}
                  fill="none"
                  strokeDasharray="4 2"
                  strokeWidth={1}
                  dot={false}
                  activeDot={false}
                  isAnimationActive={false}
                />
                <Area
                  type="monotone"
                  dataKey="mem"
                  stroke="#8b5cf6"
                  fill="url(#grad-cluster-mem)"
                  strokeWidth={1.5}
                  dot={false}
                  activeDot={false}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
