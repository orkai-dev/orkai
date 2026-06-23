import { Copy, Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { copyToClipboard } from "./-db-helpers";

export function DbCredentialRow({
  label,
  value,
  secret,
  mono,
}: {
  label: string;
  value: string;
  secret?: boolean;
  mono?: boolean;
}) {
  const [revealed, setRevealed] = useState(false);

  const display = secret && !revealed ? "••••••••••••" : value;

  return (
    <div className="flex items-center justify-between gap-4 py-2">
      <span className="shrink-0 text-sm text-muted-foreground">{label}</span>
      <div className="flex min-w-0 items-center gap-2">
        <span className={`min-w-0 truncate text-sm ${mono ? "font-mono text-xs" : ""}`}>
          {display}
        </span>
        {secret && (
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 shrink-0"
            aria-label="Toggle visibility"
            onClick={() => setRevealed(!revealed)}
          >
            {revealed ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
          </Button>
        )}
        <Button
          size="icon"
          variant="ghost"
          className="h-7 w-7 shrink-0"
          aria-label="Copy to clipboard"
          onClick={() => copyToClipboard(value)}
        >
          <Copy className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  );
}
