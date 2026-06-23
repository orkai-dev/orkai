import { Box, Cpu, MemoryStick, Plus, Server, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { EmptyState } from "@/components/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  type useClusterNodes,
  useCreateNode,
  useDeleteNode,
  useInitializeNode,
  useNodePools,
  useNodes,
  useSetNodePool,
} from "@/features/cluster";
import type { NodeMetrics as NodeMetricsType } from "@/features/cluster/types";
import { useResources } from "@/features/resources";
import { getToken } from "@/lib/auth";
import { statusVariant } from "@/lib/constants";
import { pctNumber } from "@/lib/resources";
import { ProgressBar } from "./-shared";

function AddNodeSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const createNode = useCreateNode();
  const initializeNode = useInitializeNode();
  const deleteNodeMut = useDeleteNode();
  const { data: sshKeys } = useResources("ssh_key");

  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [port, setPort] = useState("22");
  const [customPort, setCustomPort] = useState("");
  const [sshUser, setSshUser] = useState("root");
  const [customUser, setCustomUser] = useState("");
  const [authType, setAuthType] = useState("password");
  const [password, setPassword] = useState("");
  const [sshKeyId, setSshKeyId] = useState("");
  const [role, setRole] = useState("worker");

  const [phase, setPhase] = useState<"form" | "initializing" | "done" | "error">("form");
  const [logs, setLogs] = useState<string[]>([]);
  const [createdNodeId, setCreatedNodeId] = useState<string | null>(null);
  const logRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const resetForm = useCallback(() => {
    setName("");
    setHost("");
    setPort("22");
    setCustomPort("");
    setSshUser("root");
    setCustomUser("");
    setAuthType("password");
    setPassword("");
    setSshKeyId("");
    setRole("worker");
    setPhase("form");
    setLogs([]);
    setCreatedNodeId(null);
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  const handleOpenChange = useCallback(
    (next: boolean) => {
      if (!next) {
        resetForm();
      }
      onOpenChange(next);
    },
    [onOpenChange, resetForm],
  );

  useEffect(() => {
    return () => {
      wsRef.current?.close();
    };
  }, []);

  const logsLength = logs.length;
  // biome-ignore lint/correctness/useExhaustiveDependencies: scroll on log count change
  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logsLength]);

  const connectWs = useCallback((nodeId: string) => {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const token = getToken();
    const wsUrl = `${protocol}//${window.location.host}/ws/nodes/${nodeId}/logs?token=${token}`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      const msg = event.data as string;
      setLogs((prev) => [...prev, msg]);
      if (msg.includes("joined the cluster")) {
        setPhase("done");
      } else if (msg.startsWith("ERROR:") || msg.includes("Timeout:")) {
        setPhase("error");
      }
    };
    ws.onclose = () => {
      setPhase((prev) => {
        if (prev === "initializing") return "error";
        return prev;
      });
    };
    ws.onerror = () => {
      setLogs((prev) => [...prev, "--- WebSocket error ---"]);
    };
  }, []);

  const handleInitialize = useCallback(async () => {
    const resolvedPort = port === "custom" ? Number(customPort) : Number(port);
    const resolvedUser = sshUser === "custom" ? customUser : sshUser;

    setPhase("initializing");

    try {
      if (createdNodeId) {
        setLogs(["Cleaning up previous attempt..."]);
        try {
          await deleteNodeMut.mutateAsync(createdNodeId);
        } catch {
          // Ignore — record may already be gone
        }
        setCreatedNodeId(null);
      }

      setLogs((prev) => [...prev, "Creating node record..."]);
      const node = await createNode.mutateAsync({
        name,
        host,
        port: resolvedPort,
        ssh_user: resolvedUser,
        auth_type: authType,
        ...(authType === "password" ? { password } : sshKeyId ? { ssh_key_id: sshKeyId } : {}),
        role,
      });
      setCreatedNodeId(node.id);
      setLogs((prev) => [...prev, `Node "${node.name}" created. Starting initialization...`]);

      connectWs(node.id);
      await initializeNode.mutateAsync(node.id);
    } catch {
      setPhase("error");
      setLogs((prev) => [...prev, "--- Failed ---"]);
    }
  }, [
    name,
    host,
    port,
    customPort,
    sshUser,
    customUser,
    authType,
    password,
    sshKeyId,
    role,
    createdNodeId,
    createNode,
    deleteNodeMut,
    initializeNode,
    connectWs,
  ]);

  const formDisabled = phase === "initializing";

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className="flex flex-col sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>Add Node</SheetTitle>
          <SheetDescription>Add a server node to the cluster via SSH.</SheetDescription>
        </SheetHeader>

        <div className="flex flex-1 flex-col gap-4 overflow-hidden">
          <div className="space-y-3 overflow-y-auto">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label htmlFor="node-name">Name</Label>
                <Input
                  id="node-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="worker-01"
                  disabled={formDisabled}
                  required
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="node-host">Host</Label>
                <Input
                  id="node-host"
                  value={host}
                  onChange={(e) => setHost(e.target.value)}
                  placeholder="192.168.1.100"
                  disabled={formDisabled}
                  required
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label>Port</Label>
                <Select value={port} onValueChange={setPort} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="22">22</SelectItem>
                    <SelectItem value="2222">2222</SelectItem>
                    <SelectItem value="custom">Custom</SelectItem>
                  </SelectContent>
                </Select>
                {port === "custom" && (
                  <Input
                    type="number"
                    value={customPort}
                    onChange={(e) => setCustomPort(e.target.value)}
                    placeholder="Port"
                    disabled={formDisabled}
                    className="mt-1"
                  />
                )}
              </div>
              <div className="space-y-1">
                <Label>User</Label>
                <Select value={sshUser} onValueChange={setSshUser} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="root">root</SelectItem>
                    <SelectItem value="ubuntu">ubuntu</SelectItem>
                    <SelectItem value="admin">admin</SelectItem>
                    <SelectItem value="custom">Custom</SelectItem>
                  </SelectContent>
                </Select>
                {sshUser === "custom" && (
                  <Input
                    value={customUser}
                    onChange={(e) => setCustomUser(e.target.value)}
                    placeholder="Username"
                    disabled={formDisabled}
                    className="mt-1"
                  />
                )}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label>Auth Type</Label>
                <Select value={authType} onValueChange={setAuthType} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="password">Password</SelectItem>
                    <SelectItem value="ssh_key">SSH Key</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <Label>Role</Label>
                <Select value={role} onValueChange={setRole} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="worker">Worker</SelectItem>
                    <SelectItem value="server">Server</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {authType === "password" && (
              <div className="space-y-1">
                <Label htmlFor="node-password">Password</Label>
                <Input
                  id="node-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="SSH password"
                  disabled={formDisabled}
                />
              </div>
            )}

            {authType === "ssh_key" && (
              <div className="space-y-1">
                <Label>SSH Key</Label>
                <Select value={sshKeyId} onValueChange={setSshKeyId} disabled={formDisabled}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select SSH key" />
                  </SelectTrigger>
                  <SelectContent>
                    {(sshKeys ?? []).map((k) => (
                      <SelectItem key={k.id} value={k.id}>
                        {k.name}
                      </SelectItem>
                    ))}
                    {(!sshKeys || sshKeys.length === 0) && (
                      <SelectItem value="_none" disabled>
                        No SSH keys — add one in Resources
                      </SelectItem>
                    )}
                  </SelectContent>
                </Select>
              </div>
            )}
          </div>

          <div
            ref={logRef}
            className="min-h-[160px] flex-1 overflow-y-auto rounded-md bg-muted p-3 font-mono text-xs text-foreground"
          >
            {logs.length === 0 ? (
              <span className="text-muted-foreground">Initialization logs will appear here...</span>
            ) : (
              logs.map((line, i) => <div key={`log-${i}`}>{line}</div>)
            )}
          </div>

          {phase === "done" ? (
            <Button onClick={() => onOpenChange(false)} className="w-full">
              Close
            </Button>
          ) : (
            <Button
              onClick={handleInitialize}
              disabled={phase === "initializing" || !name || !host}
              className="w-full"
            >
              {phase === "initializing"
                ? "Initializing..."
                : phase === "error"
                  ? "Retry"
                  : "Initialize"}
            </Button>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function NodesTab({
  nodes,
  nodeMetrics,
}: {
  nodes: ReturnType<typeof useClusterNodes>["data"] extends infer T ? NonNullable<T> : never;
  nodeMetrics: NodeMetricsType[];
}) {
  const [sheetOpen, setSheetOpen] = useState(false);
  const { data: pools } = useNodePools();
  const setNodePool = useSetNodePool();
  const { data: managedNodes } = useNodes();
  const deleteNode = useDeleteNode();

  const k8sNodeNames = new Set(nodes.map((n) => n.name));
  const pendingNodes = (managedNodes ?? []).filter(
    (mn) =>
      !k8sNodeNames.has(mn.name) && !k8sNodeNames.has(mn.k8s_node_name) && mn.status !== "ready",
  );
  const offlineNodes = (managedNodes ?? []).filter(
    (mn) =>
      mn.status === "ready" && !k8sNodeNames.has(mn.name) && !k8sNodeNames.has(mn.k8s_node_name),
  );

  const metricsMap = useMemo(() => {
    const map = new Map<string, NodeMetricsType>();
    for (const m of nodeMetrics) map.set(m.name, m);
    return map;
  }, [nodeMetrics]);

  return (
    <div className="mt-3 space-y-3">
      <div className="flex justify-end">
        <Button size="sm" onClick={() => setSheetOpen(true)}>
          <Plus className="h-4 w-4" /> Add Node
        </Button>
      </div>

      {nodes.length === 0 ? (
        <EmptyState
          icon={Server}
          message="No nodes found"
          actionLabel="Add Node"
          onAction={() => setSheetOpen(true)}
        />
      ) : (
        nodes.map((node) => {
          const nm = metricsMap.get(node.name);
          const cpuPct = nm ? pctNumber(nm.cpu_used, nm.cpu_total) : 0;
          const memPct = nm ? pctNumber(nm.mem_used, nm.mem_total) : 0;

          return (
            <Card key={node.name}>
              <CardHeader className="flex flex-row items-start gap-3 pb-3">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                  <Server className="h-4 w-4 text-primary" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <CardTitle className="text-sm">{node.name}</CardTitle>
                    <Badge variant={statusVariant(node.status)}>{node.status}</Badge>
                    {node.roles.map((r) => (
                      <Badge key={r} variant="outline" className="text-xs">
                        {r}
                      </Badge>
                    ))}
                  </div>
                  <CardDescription className="text-xs">
                    {node.ip} · {node.version} · {node.os}/{node.arch}
                  </CardDescription>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="grid gap-4 sm:grid-cols-3">
                  <div>
                    <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
                      <span className="flex items-center gap-1">
                        <Cpu className="h-3 w-3" /> CPU
                      </span>
                      <span>
                        {nm?.cpu_used || "N/A"} / {nm?.cpu_total || node.resources.cpu_total} (
                        {cpuPct}%)
                      </span>
                    </div>
                    <ProgressBar value={cpuPct} />
                  </div>
                  <div>
                    <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
                      <span className="flex items-center gap-1">
                        <MemoryStick className="h-3 w-3" /> Memory
                      </span>
                      <span>
                        {nm?.mem_used || "N/A"} / {nm?.mem_total || node.resources.mem_total} (
                        {memPct}%)
                      </span>
                    </div>
                    <ProgressBar value={memPct} />
                  </div>
                  <div className="flex items-center gap-1 text-xs text-muted-foreground">
                    <Box className="h-3 w-3" /> {nm?.pod_count ?? 0} pods
                  </div>
                </div>
                <div className="flex items-center justify-between border-t pt-3">
                  <span className="text-xs text-muted-foreground">Node Pool</span>
                  <Select
                    value={node.pool || "none"}
                    onValueChange={(v) =>
                      setNodePool.mutate({ nodeName: node.name, pool: v === "none" ? "" : v })
                    }
                  >
                    <SelectTrigger className="h-7 w-40 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">No pool</SelectItem>
                      {[
                        ...new Set([
                          ...(pools ?? []),
                          "default",
                          "production",
                          "development",
                          "testing",
                          "build",
                        ]),
                      ].map((p) => (
                        <SelectItem key={p} value={p}>
                          {p}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </CardContent>
            </Card>
          );
        })
      )}

      {offlineNodes.length > 0 && (
        <div className="space-y-2">
          <h3 className="text-xs font-medium text-muted-foreground">Offline Nodes</h3>
          {offlineNodes.map((mn) => (
            <Card key={mn.id}>
              <CardContent className="flex items-center gap-3 p-4">
                <Server className="h-4 w-4 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{mn.k8s_node_name || mn.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {mn.host} · Previously ready, no longer in cluster
                  </p>
                </div>
                <Badge variant="secondary" className="text-xs">
                  offline
                </Badge>
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-destructive"
                  onClick={() => deleteNode.mutate(mn.id)}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {pendingNodes.length > 0 && (
        <div className="space-y-2">
          <h3 className="text-xs font-medium text-muted-foreground">Pending / Failed Nodes</h3>
          {pendingNodes.map((mn) => (
            <Card key={mn.id}>
              <CardContent className="flex items-center gap-3 p-4">
                <Server className="h-4 w-4 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{mn.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {mn.host}:{mn.port} · {mn.status}
                  </p>
                </div>
                <Badge
                  variant={mn.status === "error" ? "destructive" : "warning"}
                  className="text-xs"
                >
                  {mn.status}
                </Badge>
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-destructive"
                  onClick={() => deleteNode.mutate(mn.id)}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <AddNodeSheet open={sheetOpen} onOpenChange={setSheetOpen} />
    </div>
  );
}
