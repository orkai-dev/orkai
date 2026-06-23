import { createFileRoute, redirect, useNavigate, useSearch } from "@tanstack/react-router";
import { KeyRound, Loader2, Lock, Mail, ShieldCheck, User } from "lucide-react";
import { useEffect, useState } from "react";
import {
  AuthDivider,
  AuthError,
  AuthField,
  AuthLinkButton,
  AuthShell,
} from "@/components/auth-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useOAuthProviders, useSetupStatus } from "@/features/auth";
import { useRestoreFromS3, useScanS3Backups } from "@/features/system-backup";
import type { S3BackupFile } from "@/features/system-backup/types";
import { api } from "@/lib/api";
import {
  clearTokens,
  formatApiError,
  getToken,
  login,
  register,
  resetVerification,
} from "@/lib/auth";
import { BRAND_NAME } from "@/lib/brand";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/auth/login")({
  validateSearch: (search: Record<string, unknown>) => ({
    error: typeof search.error === "string" ? search.error : undefined,
  }),
  beforeLoad: async () => {
    if (!getToken()) return;
    resetVerification();
    try {
      await api.get("/api/v1/auth/me");
      throw redirect({ to: "/dashboard" });
    } catch (err) {
      if (err && typeof err === "object" && "isRedirect" in err) throw err;
      clearTokens();
      resetVerification();
    }
  },
  component: AuthPage,
});

function AuthPage() {
  const { data: setup, isLoading: setupLoading, isError: setupError } = useSetupStatus();
  const [showRestore, setShowRestore] = useState(false);
  const [setupTimedOut, setSetupTimedOut] = useState(false);

  useEffect(() => {
    if (!setupLoading) {
      setSetupTimedOut(false);
      return;
    }
    const timer = window.setTimeout(() => setSetupTimedOut(true), 8000);
    return () => window.clearTimeout(timer);
  }, [setupLoading]);

  if (setupLoading && !setupTimedOut && !setupError) {
    return (
      <AuthShell title="Loading" description="Checking system status...">
        <div className="space-y-4">
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
        </div>
      </AuthShell>
    );
  }

  if (setup && !setup.initialized) {
    return showRestore ? (
      <RestoreForm onBack={() => setShowRestore(false)} />
    ) : (
      <SetupForm onRestore={() => setShowRestore(true)} />
    );
  }

  return <LoginForm />;
}

// ── First-time setup (registration) ──────────────────────────────

function SetupForm({ onRestore }: { onRestore: () => void }) {
  const navigate = useNavigate();
  const [form, setForm] = useState({
    email: "",
    password: "",
    displayName: "",
    orgName: BRAND_NAME,
  });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await register(form.email, form.password, form.displayName, form.orgName);
      resetVerification();
      navigate({ to: "/dashboard" });
    } catch (err: unknown) {
      setError(formatApiError(err, "Registration failed"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthShell
      title={`Welcome to ${BRAND_NAME}`}
      description="Create your admin account to get started"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && <AuthError message={error} />}
        <AuthField id="name" label="Display Name" icon={User}>
          <Input
            id="name"
            value={form.displayName}
            onChange={(e) => setForm({ ...form, displayName: e.target.value })}
            placeholder="Your name"
            className="pl-9"
            required
          />
        </AuthField>
        <AuthField id="email" label="Email" icon={Mail}>
          <Input
            id="email"
            type="email"
            value={form.email}
            onChange={(e) => setForm({ ...form, email: e.target.value })}
            placeholder="you@company.com"
            className="pl-9"
            required
          />
        </AuthField>
        <AuthField id="password" label="Password" icon={Lock}>
          <Input
            id="password"
            type="password"
            value={form.password}
            onChange={(e) => setForm({ ...form, password: e.target.value })}
            placeholder="Minimum 8 characters"
            className="pl-9"
            required
            minLength={8}
          />
        </AuthField>
        <Button type="submit" className="w-full" size="lg" disabled={loading}>
          {loading ? "Creating account..." : "Create Account"}
        </Button>
      </form>
      <AuthDivider />
      <AuthLinkButton onClick={onRestore}>Restore from backup</AuthLinkButton>
    </AuthShell>
  );
}

// ── Restore from S3 backup ───────────────────────────────────────

function RestoreForm({ onBack }: { onBack: () => void }) {
  const scan = useScanS3Backups();
  const restore = useRestoreFromS3();

  const [s3, setS3] = useState({
    endpoint: "",
    bucket: "",
    access_key: "",
    secret_key: "",
    path: "orkai-backups",
    setup_secret: "",
  });
  const [backups, setBackups] = useState<S3BackupFile[]>([]);
  const [selected, setSelected] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [step, setStep] = useState<"credentials" | "select" | "restoring" | "done">("credentials");

  async function handleScan(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const files = await scan.mutateAsync(s3);
      if (!files || files.length === 0) {
        setError("No backup files found in the specified path");
        return;
      }
      setBackups(files);
      setStep("select");
    } catch (err: unknown) {
      setError(formatApiError(err, "Failed to scan S3 bucket"));
    }
  }

  async function handleRestore() {
    setError("");
    setStep("restoring");
    try {
      await restore.mutateAsync({
        endpoint: s3.endpoint,
        bucket: s3.bucket,
        access_key: s3.access_key,
        secret_key: s3.secret_key,
        s3_key: selected,
        setup_secret: s3.setup_secret,
      });
      setStep("done");
      setTimeout(() => {
        window.location.href = "/auth/login";
      }, 3000);
    } catch (err: unknown) {
      setError(formatApiError(err, "Restore failed"));
      setStep("select");
    }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  return (
    <AuthShell wide title={`Restore ${BRAND_NAME}`} description="Restore from a previous backup">
      {error && <AuthError message={error} />}

      {step === "credentials" && (
        <form onSubmit={handleScan} className="space-y-4">
          <AuthField id="s3-endpoint" label="S3 Endpoint">
            <Input
              id="s3-endpoint"
              value={s3.endpoint}
              onChange={(e) => setS3({ ...s3, endpoint: e.target.value })}
              placeholder="https://s3.amazonaws.com"
              required
            />
          </AuthField>
          <div className="grid gap-4 sm:grid-cols-2">
            <AuthField id="s3-bucket" label="Bucket">
              <Input
                id="s3-bucket"
                value={s3.bucket}
                onChange={(e) => setS3({ ...s3, bucket: e.target.value })}
                placeholder="my-backups"
                required
              />
            </AuthField>
            <AuthField id="s3-path" label="Path">
              <Input
                id="s3-path"
                value={s3.path}
                onChange={(e) => setS3({ ...s3, path: e.target.value })}
                placeholder="orkai-backups"
              />
            </AuthField>
          </div>
          <AuthField id="s3-access-key" label="Access Key" icon={KeyRound}>
            <Input
              id="s3-access-key"
              value={s3.access_key}
              onChange={(e) => setS3({ ...s3, access_key: e.target.value })}
              className="pl-9"
              required
            />
          </AuthField>
          <AuthField id="s3-secret-key" label="Secret Key" icon={Lock}>
            <Input
              id="s3-secret-key"
              type="password"
              value={s3.secret_key}
              onChange={(e) => setS3({ ...s3, secret_key: e.target.value })}
              className="pl-9"
              required
            />
          </AuthField>
          <AuthField id="setup-secret" label="Setup Secret" icon={ShieldCheck}>
            <Input
              id="setup-secret"
              type="password"
              value={s3.setup_secret}
              onChange={(e) => setS3({ ...s3, setup_secret: e.target.value })}
              placeholder="From /opt/orkai/.env"
              className="pl-9"
              required
            />
            <p className="mt-1.5 text-code-sm text-muted-foreground">
              Found in <code className="text-foreground">/opt/orkai/.env</code> on your server
            </p>
          </AuthField>
          <Button type="submit" className="w-full" size="lg" disabled={scan.isPending}>
            {scan.isPending ? "Scanning..." : "Scan for backups"}
          </Button>
        </form>
      )}

      {step === "select" && (
        <div className="space-y-4">
          <AuthField id="backup-select" label="Select a backup">
            <div className="max-h-60 space-y-1 overflow-y-auto border border-border p-2">
              {backups.map((b) => (
                <label
                  key={b.key}
                  className={cn(
                    "flex cursor-pointer items-center gap-3 border border-transparent p-2.5 text-sm transition-colors hover:border-border hover:bg-muted/50",
                    selected === b.key && "border-primary/30 bg-primary/5",
                  )}
                >
                  <input
                    type="radio"
                    name="backup"
                    value={b.key}
                    checked={selected === b.key}
                    onChange={() => setSelected(b.key)}
                    className="accent-primary"
                  />
                  <div className="flex-1">
                    <p className="font-medium">{b.file_name}</p>
                    <p className="text-code-sm text-muted-foreground">
                      {formatSize(b.size_bytes)} &middot; {b.last_modified}
                    </p>
                  </div>
                </label>
              ))}
            </div>
          </AuthField>

          <div className="border border-destructive/30 bg-destructive/5 p-4 space-y-2">
            <p className="text-label-caps text-destructive">Warning</p>
            <p className="text-xs text-destructive/80">
              This will completely overwrite all data in the current database. This action cannot be
              undone.
            </p>
            <p className="text-xs text-destructive/80">
              Only restore from backups you trust. Malicious backups could compromise your system.
            </p>
          </div>

          <AuthField id="confirm-restore" label="Type RESTORE to confirm">
            <Input
              id="confirm-restore"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder="RESTORE"
              className="font-mono uppercase tracking-wider"
            />
          </AuthField>

          <Button
            onClick={handleRestore}
            className="w-full"
            size="lg"
            variant="destructive"
            disabled={confirm !== "RESTORE" || !selected}
          >
            Restore
          </Button>

          <AuthLinkButton
            onClick={() => {
              setStep("credentials");
              setSelected("");
              setConfirm("");
              setError("");
            }}
          >
            &larr; Back to credentials
          </AuthLinkButton>
        </div>
      )}

      {step === "restoring" && (
        <div className="flex flex-col items-center gap-6 py-6">
          <div className="flex h-14 w-14 items-center justify-center border border-primary/30 bg-primary/10">
            <Loader2 className="h-7 w-7 animate-spin text-primary" />
          </div>
          <div className="text-center">
            <p className="text-sm font-semibold">Restoring your data...</p>
            <p className="mt-1 text-xs text-muted-foreground">
              This may take a minute. Please do not close this page.
            </p>
          </div>
          <div className="w-full space-y-2.5 text-code-sm text-muted-foreground">
            <div className="flex items-center gap-2.5">
              <span className="status-dot bg-primary" />
              Downloading backup from S3
            </div>
            <div className="flex items-center gap-2.5">
              <span className="status-dot bg-primary" />
              Restoring database
            </div>
            <div className="flex items-center gap-2.5">
              <span className="status-dot bg-muted-foreground/30" />
              Restarting system
            </div>
          </div>
        </div>
      )}

      {step === "done" && (
        <div className="flex flex-col items-center gap-5 py-6">
          <div className="flex h-14 w-14 items-center justify-center border border-success/30 bg-success/10">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-7 w-7 text-success"
              role="img"
              aria-label="Success"
            >
              <polyline points="20 6 9 17 4 12" />
            </svg>
          </div>
          <div className="text-center">
            <p className="text-sm font-semibold">Restore complete!</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Your system has been restored. Redirecting to login...
            </p>
          </div>
        </div>
      )}

      {step !== "restoring" && step !== "done" && (
        <AuthLinkButton onClick={onBack}>&larr; Back to setup</AuthLinkButton>
      )}
    </AuthShell>
  );
}

// ── Login (with 2FA support) ─────────────────────────────────────

function LoginForm() {
  const navigate = useNavigate();
  const { error: oauthError } = useSearch({ from: "/auth/login" });
  const { data: providers } = useOAuthProviders();
  const googleEnabled = providers?.google?.enabled ?? false;
  const passwordEnabled = providers?.password_enabled ?? true;
  const [step, setStep] = useState<"credentials" | "2fa">("credentials");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [twoFACode, setTwoFACode] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (oauthError) {
      setError(formatOAuthError(oauthError));
      window.history.replaceState({}, "", "/auth/login");
    }
  }, [oauthError]);

  function handleGoogleLogin() {
    window.location.href = "/api/v1/auth/oauth/google/login";
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const result = await login(email, password, step === "2fa" ? twoFACode : undefined);

      if (result.requires_2fa) {
        setStep("2fa");
        return;
      }

      if (!result.access_token || !result.refresh_token) {
        setError("Sign-in failed — no session was returned. Try again.");
        return;
      }

      resetVerification();
      navigate({ to: "/dashboard" });
    } catch (err: unknown) {
      setError(formatApiError(err, "Login failed"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthShell
      title={step === "credentials" ? "Sign in" : "Two-factor authentication"}
      description={
        step === "credentials"
          ? "Enter your credentials to access the control plane"
          : "Enter the 6-digit code from your authenticator app"
      }
    >
      {step === "credentials" ? (
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && <AuthError message={error} />}
          {passwordEnabled ? (
            <>
              <AuthField id="login-email" label="Email" icon={Mail}>
                <Input
                  id="login-email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@company.com"
                  className="pl-9"
                  autoComplete="email"
                  required
                />
              </AuthField>
              <AuthField id="login-password" label="Password" icon={Lock}>
                <Input
                  id="login-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="pl-9"
                  autoComplete="current-password"
                  required
                />
              </AuthField>
              <Button type="submit" className="w-full" size="lg" disabled={loading}>
                {loading ? "Signing in..." : "Sign in"}
              </Button>
              {googleEnabled && (
                <>
                  <AuthDivider />
                  <Button
                    type="button"
                    variant="outline"
                    className="w-full"
                    size="lg"
                    onClick={handleGoogleLogin}
                  >
                    Continue with Google
                  </Button>
                </>
              )}
            </>
          ) : googleEnabled ? (
            <Button
              type="button"
              variant="outline"
              className="w-full"
              size="lg"
              onClick={handleGoogleLogin}
            >
              Continue with Google
            </Button>
          ) : (
            <p className="text-center text-sm text-muted-foreground">
              Sign-in is not configured. Contact your administrator.
            </p>
          )}
          <p className="text-center text-code-sm text-muted-foreground">
            Need access? Contact your team administrator.
          </p>
        </form>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-5">
          <div className="flex justify-center">
            <div className="flex h-12 w-12 items-center justify-center border border-primary/30 bg-primary/10">
              <ShieldCheck className="h-6 w-6 text-primary" />
            </div>
          </div>
          {error && <AuthError message={error} />}
          <div className="space-y-2">
            <label htmlFor="2fa-code" className="text-label-caps text-muted-foreground">
              Verification code
            </label>
            <Input
              id="2fa-code"
              value={twoFACode}
              onChange={(e) => setTwoFACode(e.target.value.replace(/\D/g, "").slice(0, 6))}
              placeholder="000000"
              className="h-12 text-center font-mono text-2xl tracking-[0.5em]"
              maxLength={6}
              inputMode="numeric"
              autoComplete="one-time-code"
              autoFocus
            />
          </div>
          <Button
            type="submit"
            className="w-full"
            size="lg"
            disabled={loading || twoFACode.length !== 6}
          >
            {loading ? "Verifying..." : "Verify"}
          </Button>
          <AuthLinkButton
            onClick={() => {
              setStep("credentials");
              setTwoFACode("");
              setError("");
            }}
          >
            &larr; Back to login
          </AuthLinkButton>
        </form>
      )}
    </AuthShell>
  );
}

function formatOAuthError(code: string): string {
  switch (code) {
    case "no_account":
      return `No ${BRAND_NAME} account found for that Google email. Ask your administrator for access.`;
    case "domain_not_allowed":
      return "Your email domain is not allowed to sign in with Google.";
    case "email_not_verified":
      return "Your Google email address is not verified.";
    case "oauth_disabled":
      return "Google sign-in is not enabled.";
    case "oauth_not_configured":
      return "Google sign-in is not configured yet.";
    case "invalid_state":
    case "missing_code":
      return "Google sign-in was interrupted. Please try again.";
    case "token_exchange_failed":
      return "Google sign-in failed during authorization. Please try again.";
    default:
      return "Google sign-in failed. Please try again or use email and password.";
  }
}
