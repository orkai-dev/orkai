import { Info, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function SectionCard({
  icon: Icon,
  title,
  description,
  dirty,
  saving,
  onSave,
  children,
}: {
  icon: React.ElementType;
  title: string;
  description: string;
  dirty?: boolean;
  saving?: boolean;
  onSave?: () => void;
  children: React.ReactNode;
}) {
  return (
    <Card className={dirty ? "border-primary/50" : ""}>
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <Icon className="h-4 w-4 text-primary" />
          </div>
          <div>
            <CardTitle className="text-sm">{title}</CardTitle>
            <CardDescription className="text-xs">{description}</CardDescription>
          </div>
        </div>
        {onSave && (
          <Button
            size="sm"
            onClick={onSave}
            disabled={saving || !dirty}
            variant={dirty ? "default" : "outline"}
          >
            <Save className="h-3.5 w-3.5" /> {saving ? "Saving..." : dirty ? "Save" : "Saved"}
          </Button>
        )}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

export function InfoBanner({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2 rounded-lg border border-blue-500/20 bg-blue-500/5 px-3 py-2 text-xs text-blue-600 dark:text-blue-400">
      <Info className="h-3.5 w-3.5 shrink-0" />
      {children}
    </div>
  );
}
