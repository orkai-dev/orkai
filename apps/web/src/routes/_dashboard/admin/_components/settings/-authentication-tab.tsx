import { Copy, Globe, Loader2, Save, Shield } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { LoadingScreen } from "@/components/loading-screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ToggleSwitch } from "@/components/ui/toggle-switch";
import { useSettings, useUpdateSetting } from "@/features/settings";

// ── Authentication Tab ──────────────────────────────────────────────

export function AuthenticationTab() {
  const { data: settings, isLoading } = useSettings();
  const saveMutation = useUpdateSetting();

  const [googleEnabled, setGoogleEnabled] = useState(false);
  const [googleOnly, setGoogleOnly] = useState(false);
  const [clientID, setClientID] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [allowedDomains, setAllowedDomains] = useState("");

  useEffect(() => {
    if (!settings) return;
    setGoogleEnabled(settings.google_oauth_enabled === "true");
    setGoogleOnly(settings.auth_google_only === "true");
    setClientID(settings.google_oauth_client_id ?? "");
    setClientSecret(settings.google_oauth_client_secret ?? "");
    setAllowedDomains(settings.oauth_allowed_domains ?? "");
  }, [settings]);

  const redirectURI =
    typeof window !== "undefined"
      ? `${window.location.origin}/api/v1/auth/oauth/google/callback`
      : "/api/v1/auth/oauth/google/callback";

  async function saveSetting(key: string, value: string) {
    await saveMutation.mutateAsync({ key, value });
  }

  async function handleToggleEnabled(checked: boolean) {
    const previous = googleEnabled;
    setGoogleEnabled(checked);
    try {
      await saveSetting("google_oauth_enabled", checked ? "true" : "false");
    } catch {
      setGoogleEnabled(previous);
    }
  }

  async function handleToggleGoogleOnly(checked: boolean) {
    const previous = googleOnly;
    setGoogleOnly(checked);
    try {
      await saveSetting("auth_google_only", checked ? "true" : "false");
    } catch {
      setGoogleOnly(previous);
    }
  }

  async function handleSaveGoogle() {
    await saveSetting("google_oauth_client_id", clientID.trim());
    await saveSetting("google_oauth_client_secret", clientSecret.trim());
  }

  async function handleSaveDomains() {
    await saveSetting("oauth_allowed_domains", allowedDomains.trim());
  }

  if (isLoading) return <LoadingScreen />;

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-4">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Shield className="h-4 w-4" /> Google Sign-In
          </CardTitle>
          <ToggleSwitch checked={googleEnabled} onChange={handleToggleEnabled} />
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Allow existing users to sign in with Google. When Google-only mode is enabled below,
            invited users are created immediately and sign in with Google only.
          </p>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label>Client ID</Label>
              <Input
                value={clientID}
                onChange={(e) => setClientID(e.target.value)}
                placeholder="xxxx.apps.googleusercontent.com"
              />
            </div>
            <div className="space-y-2">
              <Label>Client Secret</Label>
              <Input
                type="password"
                value={clientSecret}
                onChange={(e) => setClientSecret(e.target.value)}
                placeholder="••••••••"
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label>Redirect URI</Label>
            <div className="flex items-center gap-2">
              <Input value={redirectURI} readOnly className="font-mono text-xs" />
              <Button
                type="button"
                size="icon"
                variant="outline"
                onClick={() => {
                  navigator.clipboard.writeText(redirectURI);
                  toast.success("Redirect URI copied");
                }}
              >
                <Copy className="h-4 w-4" />
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              Add this redirect URI in your Google Cloud Console OAuth client.
            </p>
          </div>
          <Button
            size="sm"
            onClick={handleSaveGoogle}
            disabled={saveMutation.isPending || !clientID.trim() || !clientSecret.trim()}
          >
            {saveMutation.isPending ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Save className="mr-2 h-4 w-4" />
            )}
            Save Google credentials
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-4">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Shield className="h-4 w-4" /> Google-Only Sign-In
          </CardTitle>
          <ToggleSwitch checked={googleOnly} onChange={handleToggleGoogleOnly} />
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Disable password sign-in for all users except the original bootstrap administrator.
            Invited users are created immediately and can only sign in with Google. The bootstrap
            admin can still use password sign-in at{" "}
            <code className="text-foreground">/auth/login/not-admin</code>.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Globe className="h-4 w-4" /> Allowed Email Domains
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Restrict Google sign-in and Google-only invitations to specific email domains. Leave
            empty to allow any domain.
          </p>
          <div className="space-y-2">
            <Label>Domains</Label>
            <Input
              value={allowedDomains}
              onChange={(e) => setAllowedDomains(e.target.value)}
              placeholder="mycompany.com, subsidiary.com"
              className="max-w-md"
            />
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={handleSaveDomains}
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Save className="mr-2 h-4 w-4" />
            )}
            Save domain restriction
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
