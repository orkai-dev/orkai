import { Check, Globe, Loader2, Save, Server, Shield, X } from "lucide-react";
import { useEffect, useState } from "react";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { useSettings, useUpdateSetting, useVerifyDomain } from "@/features/settings";

// ── General Tab ─────────────────────────────────────────────────────

// ── Panel Domain Card + Setup Dialog ──────────────────────────────

function PanelDomainCard({
  currentDomain,
  serverIP,
  saveMutation,
}: {
  currentDomain: string;
  serverIP: string;
  saveMutation: ReturnType<typeof useUpdateSetting>;
}) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Globe className="h-4 w-4" /> Panel Domain
          </CardTitle>
          <Button size="sm" variant="outline" onClick={() => setOpen(true)}>
            {currentDomain ? "Manage" : "Configure"}
          </Button>
        </CardHeader>
        <CardContent>
          {currentDomain ? (
            <a
              href={`https://${currentDomain}`}
              target="_blank"
              rel="noopener noreferrer"
              className="font-mono text-sm font-medium hover:underline"
            >
              {currentDomain}
            </a>
          ) : (
            <p className="text-sm text-muted-foreground">
              Access the panel via{" "}
              <code className="rounded bg-muted px-1.5 py-0.5">http://{serverIP}:3000</code>
            </p>
          )}
        </CardContent>
      </Card>

      <PanelDomainDialog
        open={open}
        onOpenChange={setOpen}
        currentDomain={currentDomain}
        serverIP={serverIP}
        saveMutation={saveMutation}
      />
    </>
  );
}

function VerifyStep({
  label,
  status,
  detail,
}: {
  label: string;
  status: "pass" | "fail" | "loading" | "warn" | "skip";
  detail: string;
}) {
  const icons = {
    pass: <Check className="h-4 w-4 text-green-500" />,
    fail: <X className="h-4 w-4 text-destructive" />,
    loading: <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />,
    warn: <Shield className="h-4 w-4 text-amber-500" />,
    skip: <div className="h-4 w-4 rounded-full bg-muted" />,
  };
  return (
    <div className="flex items-start gap-3 rounded-md border p-3">
      <div className="mt-0.5 shrink-0">{icons[status]}</div>
      <div>
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground">{detail}</p>
      </div>
    </div>
  );
}

function PanelDomainDialog({
  open,
  onOpenChange,
  currentDomain,
  serverIP,
  saveMutation,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  currentDomain: string;
  serverIP: string;
  saveMutation: ReturnType<typeof useUpdateSetting>;
}) {
  const [domain, setDomain] = useState("");
  const verify = useVerifyDomain();
  const v = verify.data;
  const allGood = v?.dns === "ok" && v?.reachable === true && v?.cert === "valid";

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset on open
  useEffect(() => {
    if (open) {
      setDomain(currentDomain);
      verify.reset();
    }
  }, [open]);

  function handleSave() {
    const trimmed = domain.trim().toLowerCase();
    if (!trimmed) return;
    saveMutation.mutate(
      { key: "panel_domain", value: trimmed },
      { onSuccess: () => setDomain(trimmed) },
    );
  }

  function handleVerify() {
    verify.mutate(domain.trim().toLowerCase());
  }

  function handleRemove() {
    saveMutation.mutate(
      { key: "panel_domain", value: "" },
      {
        onSuccess: () => {
          setDomain("");
          verify.reset();
          onOpenChange(false);
        },
      },
    );
  }

  const saved =
    domain.trim().toLowerCase() === (currentDomain || "").toLowerCase() || saveMutation.isSuccess;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{currentDomain ? "Panel Domain" : "Add Panel Domain"}</DialogTitle>
          <DialogDescription>
            Access your panel via a custom domain with automatic HTTPS.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Domain input + Save + Verify */}
          <div className="space-y-1.5">
            <Label className="text-xs">Domain</Label>
            <div className="flex items-center gap-2">
              <Input
                value={domain}
                onChange={(e) => {
                  setDomain(e.target.value);
                  verify.reset();
                }}
                placeholder="panel.example.com"
                className="font-mono text-sm"
                autoFocus
              />
              {!saved ? (
                <Button
                  size="sm"
                  onClick={handleSave}
                  disabled={saveMutation.isPending || !domain.trim()}
                >
                  {saveMutation.isPending ? "..." : "Save"}
                </Button>
              ) : (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleVerify}
                  disabled={verify.isPending || !domain.trim()}
                >
                  {verify.isPending ? "..." : "Verify"}
                </Button>
              )}
            </div>
          </div>

          {/* DNS record — visible when domain is entered */}
          {domain.trim() && (
            <div className="rounded-md bg-muted/50 p-3">
              <p className="mb-2 text-xs font-medium">Required DNS Record</p>
              <div className="grid grid-cols-3 gap-x-4 font-mono text-xs">
                <div>
                  <p className="text-muted-foreground">Type</p>
                  <p>A</p>
                </div>
                <div>
                  <p className="text-muted-foreground">Name</p>
                  <p>{domain.trim().split(".")[0] || "panel"}</p>
                </div>
                <div>
                  <p className="text-muted-foreground">Value</p>
                  <p>{serverIP}</p>
                </div>
              </div>
            </div>
          )}

          {/* Verification results — visible after clicking Verify */}
          {v && (
            <div className="space-y-2">
              <VerifyStep
                label="DNS"
                status={v.dns === "ok" ? "pass" : "fail"}
                detail={
                  v.dns === "ok" ? `Resolves to ${v.dns_ip}` : v.dns_message || "DNS not configured"
                }
              />
              {v.dns === "ok" && (
                <VerifyStep
                  label="HTTPS"
                  status={v.reachable === true ? "pass" : "fail"}
                  detail={
                    v.reachable === true
                      ? "Port 443 open"
                      : v.reachable_message || "Port 443 not reachable"
                  }
                />
              )}
              {v.reachable === true && (
                <VerifyStep
                  label="Certificate"
                  status={
                    v.cert === "valid"
                      ? "pass"
                      : v.cert === "self_signed"
                        ? "warn"
                        : v.cert === "cloudflare"
                          ? "warn"
                          : "fail"
                  }
                  detail={
                    v.cert === "valid"
                      ? `${v.cert_issuer} — expires ${v.cert_expiry}`
                      : v.cert === "self_signed"
                        ? "Certificate is being issued — verify again in a minute"
                        : v.cert === "cloudflare"
                          ? "Cloudflare proxy detected — set SSL to Full (Strict)"
                          : "No certificate found"
                  }
                />
              )}
              {allGood && (
                <p className="rounded-md border border-green-500/20 bg-green-500/5 p-2.5 text-center text-xs text-green-600">
                  Live at{" "}
                  <a
                    href={`https://${domain}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="font-medium underline"
                  >
                    https://{domain}
                  </a>
                </p>
              )}
            </div>
          )}
        </div>

        {currentDomain && (
          <DialogFooter>
            <Button
              variant="ghost"
              size="sm"
              className="mr-auto text-destructive"
              onClick={handleRemove}
              disabled={saveMutation.isPending}
            >
              Remove Domain
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  );
}

export function GeneralTab() {
  const { data: settings, isLoading } = useSettings();
  const saveDomain = useUpdateSetting();
  const savePanel = useUpdateSetting();
  const saveEmail = useUpdateSetting();
  const [baseDomain, setBaseDomain] = useState("");
  const [httpsEmail, setHttpsEmail] = useState("");

  useEffect(() => {
    if (settings) {
      setBaseDomain(settings.base_domain ?? "");
      setHttpsEmail(settings.https_email ?? "");
    }
  }, [settings]);

  if (isLoading) return <LoadingScreen />;
  if (!settings) return null;

  const defaultDomain = settings.server_ip ? `${settings.server_ip}.sslip.io` : "";

  return (
    <div className="space-y-6">
      {/* Server info */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Server className="h-4 w-4" /> Server
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Server IP</span>
            <Badge variant="outline" className="font-mono">
              {settings.server_ip || "detecting..."}
            </Badge>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Setup Status</span>
            <Badge variant={settings.setup_done === "true" ? "success" : "warning"}>
              {settings.setup_done === "true" ? "Configured" : "Pending"}
            </Badge>
          </div>
        </CardContent>
      </Card>

      {/* Wildcard Domain */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Globe className="h-4 w-4" /> Wildcard Domain
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            All services auto-generate subdomains under this domain:{" "}
            <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
              myapp-xxxx.{baseDomain || defaultDomain || "example.com"}
            </code>
          </p>
          <div className="space-y-2">
            <Label>Base Domain</Label>
            <div className="flex items-center gap-3">
              <Input
                value={baseDomain}
                onChange={(e) => setBaseDomain(e.target.value)}
                placeholder={defaultDomain}
                className="max-w-md font-mono"
              />
              <Button
                onClick={() =>
                  saveDomain.mutate({
                    key: "base_domain",
                    value: baseDomain || defaultDomain,
                  })
                }
                disabled={
                  saveDomain.isPending || (baseDomain || defaultDomain) === settings?.base_domain
                }
              >
                <Save className="h-3.5 w-3.5" /> {saveDomain.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              Leave empty to use the default:{" "}
              <code className="rounded bg-muted px-1">{defaultDomain}</code>
            </p>
          </div>
          <Separator />
          <div className="space-y-2 text-xs text-muted-foreground">
            <p className="font-medium text-foreground">How it works:</p>
            <ul className="list-inside list-disc space-y-1">
              <li>
                <strong>Development:</strong> Use{" "}
                <code className="rounded bg-muted px-1">{defaultDomain}</code> (auto-resolves to
                server IP)
              </li>
              <li>
                <strong>Production:</strong> Set your domain with wildcard DNS{" "}
                <code className="rounded bg-muted px-1">*.mysite.com &rarr; server IP</code>
              </li>
            </ul>
          </div>
        </CardContent>
      </Card>

      {/* Panel Domain */}
      <PanelDomainCard
        currentDomain={settings?.panel_domain ?? ""}
        serverIP={settings?.server_ip ?? ""}
        saveMutation={savePanel}
      />

      {/* TLS / HTTPS */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Shield className="h-4 w-4" /> TLS / HTTPS
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            TLS certificates are auto-managed by Traefik via Let's Encrypt. Provide an email for
            certificate registration and renewal notifications.
          </p>
          <div className="space-y-2">
            <Label>ACME Email</Label>
            <div className="flex items-center gap-3">
              <Input
                value={httpsEmail}
                onChange={(e) => setHttpsEmail(e.target.value)}
                placeholder="admin@example.com"
                className="max-w-md"
              />
              <Button
                onClick={() =>
                  saveEmail.mutate({
                    key: "https_email",
                    value: httpsEmail,
                  })
                }
                disabled={saveEmail.isPending || httpsEmail === (settings?.https_email ?? "")}
              >
                <Save className="h-3.5 w-3.5" /> {saveEmail.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            Required for Let's Encrypt. Certificates are issued automatically for custom domains
            (panel and app domains).
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
