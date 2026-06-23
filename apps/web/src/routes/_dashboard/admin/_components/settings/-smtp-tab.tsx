import { Mail, Save } from "lucide-react";
import { useEffect, useState } from "react";
import { LoadingScreen } from "@/components/loading-screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ToggleSwitch } from "@/components/ui/toggle-switch";
import { useSaveSMTPConfig, useSMTPConfig, useTestSMTP } from "@/features/notifications";
import type { SMTPConfig } from "@/features/notifications/types";

// ── SMTP Tab ────────────────────────────────────────────────────────

export function SMTPTab() {
  const { data: smtp, isLoading } = useSMTPConfig();
  const saveSMTP = useSaveSMTPConfig();
  const testSMTP = useTestSMTP();

  const [form, setForm] = useState<SMTPConfig>({
    host: "",
    port: "587",
    user: "",
    password: "",
    from: "",
    enabled: false,
  });

  useEffect(() => {
    if (smtp) setForm(smtp);
  }, [smtp]);

  const update = (key: keyof SMTPConfig, value: string | boolean) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  if (isLoading) return <LoadingScreen />;

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-4">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Mail className="h-4 w-4" /> SMTP Configuration
          </CardTitle>
          <ToggleSwitch checked={form.enabled} onChange={(checked) => update("enabled", checked)} />
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label>Host</Label>
              <Input
                value={form.host}
                onChange={(e) => update("host", e.target.value)}
                placeholder="smtp.gmail.com"
              />
            </div>
            <div className="space-y-2">
              <Label>Port</Label>
              <Input
                value={form.port}
                onChange={(e) => update("port", e.target.value)}
                placeholder="587"
              />
            </div>
            <div className="space-y-2">
              <Label>Username</Label>
              <Input
                value={form.user}
                onChange={(e) => update("user", e.target.value)}
                placeholder="user@gmail.com"
              />
            </div>
            <div className="space-y-2">
              <Label>Password</Label>
              <Input
                type="password"
                value={form.password}
                onChange={(e) => update("password", e.target.value)}
                placeholder="••••••••"
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label>From Address</Label>
            <Input
              value={form.from}
              onChange={(e) => update("from", e.target.value)}
              placeholder="noreply@orkai.dev"
              className="max-w-md"
            />
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => testSMTP.mutate()}
              disabled={testSMTP.isPending || !form.enabled}
            >
              {testSMTP.isPending ? "Testing..." : "Test"}
            </Button>
            <Button size="sm" onClick={() => saveSMTP.mutate(form)} disabled={saveSMTP.isPending}>
              <Save className="mr-1 h-3.5 w-3.5" />
              {saveSMTP.isPending ? "Saving..." : "Save"}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
