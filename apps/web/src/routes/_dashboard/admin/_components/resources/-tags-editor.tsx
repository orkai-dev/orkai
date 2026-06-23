import { Plus, X } from "lucide-react";
import { useRef } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ResourceTag } from "@/features/resources/types";

function nextRowId() {
  return crypto.randomUUID();
}

export function TagsEditor({
  tags,
  onChange,
}: {
  tags: ResourceTag[];
  onChange: (tags: ResourceTag[]) => void;
}) {
  const rowIdsRef = useRef<string[]>(tags.map(() => nextRowId()));

  const addRow = () => {
    rowIdsRef.current.push(nextRowId());
    onChange([...tags, { key: "", value: "" }]);
  };

  const removeRow = (index: number) => {
    rowIdsRef.current = rowIdsRef.current.filter((_, j) => j !== index);
    onChange(tags.filter((_, j) => j !== index));
  };

  return (
    <div className="space-y-2">
      {tags.map((tag, i) => (
        <div key={rowIdsRef.current[i]} className="flex items-center gap-2">
          <Input
            className="w-40 font-mono text-sm"
            placeholder="Key"
            value={tag.key}
            onChange={(e) => {
              const next = [...tags];
              next[i] = { ...next[i], key: e.target.value };
              onChange(next);
            }}
          />
          <span className="text-muted-foreground">=</span>
          <Input
            className="flex-1 font-mono text-sm"
            placeholder="value or {{env}}"
            value={tag.value}
            onChange={(e) => {
              const next = [...tags];
              next[i] = { ...next[i], value: e.target.value };
              onChange(next);
            }}
          />
          <Button
            type="button"
            size="icon"
            variant="ghost"
            className="h-8 w-8 shrink-0 text-destructive"
            onClick={() => removeRow(i)}
          >
            <X className="h-3.5 w-3.5" />
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" className="w-full" onClick={addRow}>
        <Plus className="mr-1.5 h-3.5 w-3.5" />
        Add tag
      </Button>
      <p className="text-xs text-muted-foreground">
        Variables: <code className="font-mono">{"{{env}}"}</code>,{" "}
        <code className="font-mono">{"{{team}}"}</code>,{" "}
        <code className="font-mono">{"{{project}}"}</code>,{" "}
        <code className="font-mono">{"{{page}}"}</code>
      </p>
    </div>
  );
}

function parseResourceTags(raw: unknown): ResourceTag[] {
  if (!Array.isArray(raw)) return [];
  return raw
    .filter((item): item is ResourceTag => {
      return (
        typeof item === "object" &&
        item !== null &&
        "key" in item &&
        "value" in item &&
        typeof (item as ResourceTag).key === "string" &&
        typeof (item as ResourceTag).value === "string"
      );
    })
    .map((item) => ({ key: item.key, value: item.value }));
}

export function extractTagsFromConfig(config: Record<string, unknown>): ResourceTag[] {
  return parseResourceTags(config.tags);
}

export function configWithoutTags(config: Record<string, unknown>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(config)) {
    if (k === "tags") continue;
    if (typeof v === "string") out[k] = v;
  }
  return out;
}

export function buildResourceConfig(
  config: Record<string, string>,
  tags: ResourceTag[],
): Record<string, unknown> {
  const cleaned = tags.filter((t) => t.key.trim() !== "");
  return cleaned.length > 0 ? { ...config, tags: cleaned } : { ...config };
}
