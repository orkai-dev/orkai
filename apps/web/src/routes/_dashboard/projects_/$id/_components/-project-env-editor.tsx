import { Plus, Save, X } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useUpdateProjectEnv } from "@/features/projects";

type EnvPair = { key: string; value: string };

export function ProjectEnvEditor({
  projectId,
  envVars,
}: {
  projectId: string;
  envVars: Record<string, string>;
}) {
  const updateEnv = useUpdateProjectEnv(projectId);
  const initial = Object.entries(envVars || {}).map(([key, value]) => ({ key, value }));
  const [pairs, setPairs] = useState<EnvPair[]>(initial);

  function pairsToRecord(p: EnvPair[]): Record<string, string> {
    const result: Record<string, string> = {};
    for (const pair of p) {
      if (pair.key.trim()) result[pair.key.trim()] = pair.value;
    }
    return result;
  }

  const dirty = JSON.stringify(pairsToRecord(pairs)) !== JSON.stringify(envVars || {});

  function handleSave() {
    updateEnv.mutate(pairsToRecord(pairs));
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-sm">Environment Variables</CardTitle>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => setPairs([...pairs, { key: "", value: "" }])}
          >
            <Plus className="h-3.5 w-3.5" /> Add Variable
          </Button>
          {dirty && (
            <Button size="sm" onClick={handleSave} disabled={updateEnv.isPending}>
              <Save className="h-3.5 w-3.5" /> {updateEnv.isPending ? "Saving..." : "Save"}
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {pairs.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No environment variables. Click <strong>Add Variable</strong> to add one.
          </p>
        ) : (
          <div className="space-y-2">
            {pairs.map((pair, i) => (
              <div key={i} className="flex items-center gap-2">
                <Input
                  className="w-48 border-border bg-background font-mono text-sm"
                  placeholder="KEY"
                  value={pair.key}
                  onChange={(e) => {
                    const next = [...pairs];
                    next[i] = { ...next[i], key: e.target.value };
                    setPairs(next);
                  }}
                />
                <span className="text-muted-foreground">=</span>
                <Input
                  className="flex-1 border-border bg-background font-mono text-sm"
                  placeholder="value"
                  value={pair.value}
                  onChange={(e) => {
                    const next = [...pairs];
                    next[i] = { ...next[i], value: e.target.value };
                    setPairs(next);
                  }}
                />
                <Button
                  size="icon"
                  variant="ghost"
                  className="h-8 w-8 shrink-0 text-destructive"
                  onClick={() => setPairs(pairs.filter((_, j) => j !== i))}
                >
                  <X className="h-3.5 w-3.5" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
