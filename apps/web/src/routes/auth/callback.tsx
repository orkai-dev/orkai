import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Loader2, ShieldCheck } from "lucide-react";
import { useEffect, useState } from "react";
import { AuthError, AuthLinkButton, AuthShell } from "@/components/auth-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { completeOAuth2FA, formatApiError, resetVerification, setTokens } from "@/lib/auth";

export const Route = createFileRoute("/auth/callback")({
  component: OAuthCallbackPage,
});

function OAuthCallbackPage() {
  const navigate = useNavigate();
  const [error, setError] = useState("");
  const [challenge, setChallenge] = useState<string | null>(null);
  const [twoFACode, setTwoFACode] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const hash = window.location.hash.startsWith("#")
      ? window.location.hash.slice(1)
      : window.location.hash;
    const params = new URLSearchParams(hash);
    const accessToken = params.get("access_token");
    const refreshToken = params.get("refresh_token");
    const requires2FA = params.get("requires_2fa");
    const oauthChallenge = params.get("challenge");

    if (requires2FA === "1" && oauthChallenge) {
      setChallenge(oauthChallenge);
      window.history.replaceState({}, "", "/auth/callback");
      return;
    }

    if (!accessToken || !refreshToken) {
      setError("Sign-in failed — no session was returned. Try again.");
      return;
    }

    setTokens(accessToken, refreshToken);
    resetVerification();
    window.history.replaceState({}, "", "/auth/callback");
    navigate({ to: "/dashboard" });
  }, [navigate]);

  async function handle2FASubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!challenge) return;

    setError("");
    setLoading(true);
    try {
      const result = await completeOAuth2FA(challenge, twoFACode);
      if (!result.access_token || !result.refresh_token) {
        setError("Sign-in failed — no session was returned. Try again.");
        return;
      }
      resetVerification();
      navigate({ to: "/dashboard" });
    } catch (err: unknown) {
      setError(formatApiError(err, "Verification failed"));
    } finally {
      setLoading(false);
    }
  }

  if (challenge) {
    return (
      <AuthShell
        title="Two-factor authentication"
        description="Enter the 6-digit code from your authenticator app"
      >
        <form onSubmit={handle2FASubmit} className="space-y-5">
          <div className="flex justify-center">
            <div className="flex h-12 w-12 items-center justify-center border border-primary/30 bg-primary/10">
              <ShieldCheck className="h-6 w-6 text-primary" />
            </div>
          </div>
          {error && <AuthError message={error} />}
          <div className="space-y-2">
            <label htmlFor="oauth-2fa-code" className="text-label-caps text-muted-foreground">
              Verification code
            </label>
            <Input
              id="oauth-2fa-code"
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
          <AuthLinkButton onClick={() => navigate({ to: "/auth/login" })}>
            &larr; Back to login
          </AuthLinkButton>
        </form>
      </AuthShell>
    );
  }

  if (error) {
    return (
      <AuthShell title="Sign-in failed" description="We couldn't complete Google sign-in">
        <AuthError message={error} />
      </AuthShell>
    );
  }

  return (
    <AuthShell title="Signing in" description="Completing Google sign-in...">
      <div className="flex justify-center py-8">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    </AuthShell>
  );
}
