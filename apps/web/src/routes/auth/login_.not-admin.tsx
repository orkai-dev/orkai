import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { Lock, Mail, ShieldCheck } from "lucide-react";
import { useState } from "react";
import { AuthError, AuthField, AuthLinkButton, AuthShell } from "@/components/auth-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { clearTokens, formatApiError, getToken, login, resetVerification } from "@/lib/auth";

export const Route = createFileRoute("/auth/login_/not-admin")({
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
  component: AdminLoginPage,
});

function AdminLoginPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<"credentials" | "2fa">("credentials");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [twoFACode, setTwoFACode] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

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
      title={step === "credentials" ? "Admin sign in" : "Two-factor authentication"}
      description={
        step === "credentials"
          ? "Password sign-in for the bootstrap administrator"
          : "Enter the 6-digit code from your authenticator app"
      }
    >
      {step === "credentials" ? (
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && <AuthError message={error} />}
          <AuthField id="admin-login-email" label="Email" icon={Mail}>
            <Input
              id="admin-login-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@company.com"
              className="pl-9"
              autoComplete="email"
              required
            />
          </AuthField>
          <AuthField id="admin-login-password" label="Password" icon={Lock}>
            <Input
              id="admin-login-password"
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
          <AuthLinkButton onClick={() => navigate({ to: "/auth/login" })}>
            &larr; Back to sign in
          </AuthLinkButton>
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
            <label htmlFor="admin-2fa-code" className="text-label-caps text-muted-foreground">
              Verification code
            </label>
            <Input
              id="admin-2fa-code"
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
