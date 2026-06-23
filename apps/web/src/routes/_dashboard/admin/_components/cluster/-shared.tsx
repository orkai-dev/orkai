import { Card, CardContent } from "@/components/ui/card";

export function ErrorBanner({ message }: { message: string }) {
  return (
    <Card className="mt-3">
      <CardContent className="py-8 text-center text-sm text-destructive">{message}</CardContent>
    </Card>
  );
}

export function ProgressBar({ value, className }: { value: number; className?: string }) {
  const color = value > 90 ? "bg-red-500" : value > 70 ? "bg-amber-500" : "bg-green-500";
  return (
    <div className={`h-2 w-full rounded-full bg-muted ${className ?? ""}`}>
      <div className={`h-2 rounded-full transition-all ${color}`} style={{ width: `${value}%` }} />
    </div>
  );
}

export function eventVariant(type: string): "secondary" | "destructive" {
  return type === "Warning" ? "destructive" : "secondary";
}

export function pvcStatusVariant(status: string) {
  if (status === "Bound") return "success" as const;
  if (status === "Pending") return "warning" as const;
  if (status === "Lost") return "destructive" as const;
  return "secondary" as const;
}

export function helmStatusVariant(status: string) {
  if (status === "deployed") return "success" as const;
  if (status === "failed") return "destructive" as const;
  if (status === "pending-install" || status === "pending-upgrade") return "warning" as const;
  return "secondary" as const;
}
