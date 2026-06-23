import { createFileRoute, useNavigate } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import {
  Check,
  Copy,
  Key,
  Monitor,
  Moon,
  Palette,
  Save,
  Shield,
  ShieldCheck,
  Sun,
  User,
} from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { useEffect, useState } from "react";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  useAvatars,
  useChangePassword,
  useCurrentUser,
  useDisable2FA,
  useSetup2FA,
  useUpdateProfile,
  useVerify2FA,
} from "@/features/auth";
import { getTheme, setTheme, type Theme } from "@/lib/theme";
import { cn } from "@/lib/utils";
import { APIKeysSection } from "./-api-keys-section";

const PROFILE_SECTIONS = ["account", "security", "api-keys", "preferences"] as const;
type ProfileSection = (typeof PROFILE_SECTIONS)[number];

export const Route = createFileRoute("/_dashboard/profile")({
  component: ProfilePage,
  validateSearch: (search: Record<string, unknown>): { section: ProfileSection } => {
    const section = search.section as string | undefined;
    if (section && (PROFILE_SECTIONS as readonly string[]).includes(section)) {
      return { section: section as ProfileSection };
    }
    return { section: "account" };
  },
});

const AVATAR_EMOJI: Record<string, string> = {
  bear: "\u{1F43B}",
  cat: "\u{1F431}",
  dog: "\u{1F436}",
  fox: "\u{1F98A}",
  koala: "\u{1F428}",
  lion: "\u{1F981}",
  monkey: "\u{1F435}",
  owl: "\u{1F989}",
  panda: "\u{1F43C}",
  penguin: "\u{1F427}",
  rabbit: "\u{1F430}",
  tiger: "\u{1F42F}",
  whale: "\u{1F433}",
  wolf: "\u{1F43A}",
};

const SECTION_NAV: { id: ProfileSection; label: string; icon: LucideIcon; description: string }[] =
  [
    { id: "account", label: "Account", icon: User, description: "Name, avatar, and email" },
    { id: "security", label: "Security", icon: Shield, description: "Password and 2FA" },
    { id: "api-keys", label: "API keys", icon: Key, description: "Personal access tokens" },
    { id: "preferences", label: "Preferences", icon: Palette, description: "Theme and display" },
  ];

function avatarGlyph(key?: string) {
  if (!key) return "?";
  return AVATAR_EMOJI[key] || key[0]?.toUpperCase() || "?";
}

function ProfilePage() {
  const { section } = Route.useSearch();
  const navigate = useNavigate();
  const { data: user, isLoading } = useCurrentUser();
  const { data: avatars } = useAvatars();
  const updateProfile = useUpdateProfile();
  const changePassword = useChangePassword();
  const setup2FA = useSetup2FA();
  const verify2FA = useVerify2FA();
  const disable2FA = useDisable2FA();

  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [selectedAvatar, setSelectedAvatar] = useState<string | undefined>(undefined);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordError, setPasswordError] = useState("");

  const [tfaSetupData, setTfaSetupData] = useState<{ secret: string; qr_code: string } | null>(
    null,
  );
  const [tfaCode, setTfaCode] = useState("");
  const [disableCode, setDisableCode] = useState("");
  const [showDisable2FA, setShowDisable2FA] = useState(false);
  const [secretCopied, setSecretCopied] = useState(false);

  useEffect(() => {
    if (user) {
      setFirstName(user.first_name || "");
      setLastName(user.last_name || "");
      setDisplayName(user.display_name || "");
      setSelectedAvatar(user.avatar_url);
    }
  }, [user]);

  if (isLoading) return <LoadingScreen />;
  if (!user) return null;

  const passwordsMatch = newPassword === confirmPassword;
  const passwordLongEnough = newPassword.length >= 8;
  const canChangePassword =
    currentPassword.length > 0 &&
    newPassword.length > 0 &&
    confirmPassword.length > 0 &&
    passwordsMatch &&
    passwordLongEnough;

  const handleSaveProfile = () => {
    updateProfile.mutate({
      first_name: firstName,
      last_name: lastName,
      display_name: displayName,
      avatar_url: selectedAvatar,
    });
  };

  const handleChangePassword = () => {
    if (!passwordsMatch) {
      setPasswordError("Passwords do not match");
      return;
    }
    if (!passwordLongEnough) {
      setPasswordError("Password must be at least 8 characters");
      return;
    }
    setPasswordError("");
    changePassword.mutate(
      { current_password: currentPassword, new_password: newPassword },
      {
        onSuccess: () => {
          setCurrentPassword("");
          setNewPassword("");
          setConfirmPassword("");
        },
      },
    );
  };

  const handleEnable2FA = () => {
    setup2FA.mutate(undefined, {
      onSuccess: (data) => {
        setTfaSetupData(data);
        setTfaCode("");
      },
    });
  };

  const handleVerify2FA = () => {
    verify2FA.mutate(tfaCode, {
      onSuccess: () => {
        setTfaSetupData(null);
        setTfaCode("");
      },
    });
  };

  const handleDisable2FA = () => {
    disable2FA.mutate(disableCode, {
      onSuccess: () => {
        setDisableCode("");
        setShowDisable2FA(false);
      },
    });
  };

  const copySecret = () => {
    if (tfaSetupData?.secret) {
      navigator.clipboard.writeText(tfaSetupData.secret);
      setSecretCopied(true);
      setTimeout(() => setSecretCopied(false), 2000);
    }
  };

  const setSection = (next: ProfileSection) => {
    navigate({ to: "/profile", search: { section: next }, replace: true });
  };

  return (
    <div className="space-y-6">
      <IdentityBanner
        user={user}
        avatarKey={selectedAvatar ?? user.avatar_url}
        onEditAvatar={() => setSection("account")}
      />

      <div className="flex flex-col gap-6 lg:flex-row lg:items-start">
        <nav className="flex shrink-0 gap-1 overflow-x-auto lg:w-52 lg:flex-col lg:overflow-visible">
          {SECTION_NAV.map(({ id, label, icon: Icon, description }) => {
            const active = section === id;
            return (
              <button
                key={id}
                type="button"
                onClick={() => setSection(id)}
                className={cn(
                  "flex min-w-[140px] flex-col rounded-md border px-3 py-2.5 text-left transition-colors lg:min-w-0",
                  active
                    ? "border-primary/30 bg-primary/5"
                    : "border-transparent hover:border-border hover:bg-muted/50",
                )}
              >
                <span className="flex items-center gap-2 text-sm font-medium">
                  <Icon
                    className={cn("h-4 w-4", active ? "text-primary" : "text-muted-foreground")}
                  />
                  {label}
                </span>
                <span className="mt-0.5 hidden text-xs text-muted-foreground lg:block">
                  {description}
                </span>
              </button>
            );
          })}
        </nav>

        <div className="min-w-0 flex-1 rounded-lg border bg-card">
          {section === "account" && (
            <AccountPanel
              avatars={avatars}
              selectedAvatar={selectedAvatar}
              onSelectAvatar={setSelectedAvatar}
              firstName={firstName}
              onFirstNameChange={setFirstName}
              lastName={lastName}
              onLastNameChange={setLastName}
              displayName={displayName}
              onDisplayNameChange={setDisplayName}
              email={user.email}
              onSave={handleSaveProfile}
              saving={updateProfile.isPending}
            />
          )}

          {section === "security" && (
            <SecurityPanel
              twoFaEnabled={user.two_fa_enabled}
              tfaSetupData={tfaSetupData}
              tfaCode={tfaCode}
              onTfaCodeChange={setTfaCode}
              disableCode={disableCode}
              onDisableCodeChange={setDisableCode}
              showDisable2FA={showDisable2FA}
              onShowDisable2FA={setShowDisable2FA}
              secretCopied={secretCopied}
              onCopySecret={copySecret}
              currentPassword={currentPassword}
              onCurrentPasswordChange={setCurrentPassword}
              newPassword={newPassword}
              onNewPasswordChange={(value) => {
                setNewPassword(value);
                setPasswordError("");
              }}
              confirmPassword={confirmPassword}
              onConfirmPasswordChange={(value) => {
                setConfirmPassword(value);
                setPasswordError("");
              }}
              passwordError={passwordError}
              passwordsMatch={passwordsMatch}
              passwordLongEnough={passwordLongEnough}
              canChangePassword={canChangePassword}
              onChangePassword={handleChangePassword}
              changingPassword={changePassword.isPending}
              onEnable2FA={handleEnable2FA}
              enabling2FA={setup2FA.isPending}
              onVerify2FA={handleVerify2FA}
              verifying2FA={verify2FA.isPending}
              onDisable2FA={handleDisable2FA}
              disabling2FA={disable2FA.isPending}
            />
          )}

          {section === "api-keys" && <APIKeysSection isAdmin={user.role === "admin"} />}

          {section === "preferences" && <PreferencesPanel />}
        </div>
      </div>
    </div>
  );
}

// ── Identity banner ─────────────────────────────────────────────────

function IdentityBanner({
  user,
  avatarKey,
  onEditAvatar,
}: {
  user: { display_name: string; email: string; role: string; two_fa_enabled: boolean };
  avatarKey?: string;
  onEditAvatar: () => void;
}) {
  return (
    <div className="relative overflow-hidden rounded-lg border bg-muted/30">
      <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent" />
      <div className="flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-4">
          <button
            type="button"
            onClick={onEditAvatar}
            className="group relative flex h-16 w-16 shrink-0 items-center justify-center rounded bg-primary/10 text-3xl transition-colors hover:bg-primary/15"
            title="Change avatar"
          >
            {avatarGlyph(avatarKey)}
            <span className="absolute inset-0 flex items-center justify-center rounded bg-background/80 text-[10px] font-mono uppercase tracking-widest text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100">
              Edit
            </span>
          </button>
          <div className="min-w-0">
            <h1 className="truncate text-headline-md">{user.display_name || "Your account"}</h1>
            <p className="truncate font-mono text-xs text-muted-foreground">{user.email}</p>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <Badge variant="secondary">{user.role}</Badge>
              {user.two_fa_enabled ? (
                <Badge variant="success">
                  <ShieldCheck className="mr-1 h-3 w-3" />
                  2FA on
                </Badge>
              ) : (
                <Badge variant="outline">2FA off</Badge>
              )}
            </div>
          </div>
        </div>
        <p className="max-w-xs text-xs text-muted-foreground sm:text-right">
          Personal settings for your Orka&apos;i account. Changes here only affect your login and
          workspace appearance.
        </p>
      </div>
    </div>
  );
}

// ── Shared layout helpers ───────────────────────────────────────────

function PanelHeader({ title, description }: { title: string; description: string }) {
  return (
    <div className="border-b px-5 py-4">
      <h2 className="text-sm font-semibold">{title}</h2>
      <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
    </div>
  );
}

function SettingRow({
  label,
  hint,
  children,
  last,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
  last?: boolean;
}) {
  return (
    <div
      className={cn(
        "grid gap-3 px-5 py-4 sm:grid-cols-[minmax(0,220px)_1fr] sm:items-start sm:gap-6",
        !last && "border-b",
      )}
    >
      <div className="space-y-0.5 pt-1">
        <Label className="text-sm font-medium">{label}</Label>
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
      <div className="min-w-0">{children}</div>
    </div>
  );
}

function PanelFooter({ children }: { children: React.ReactNode }) {
  return <div className="flex justify-end border-t bg-muted/20 px-5 py-3">{children}</div>;
}

// ── Account ─────────────────────────────────────────────────────────

function AccountPanel({
  avatars,
  selectedAvatar,
  onSelectAvatar,
  firstName,
  onFirstNameChange,
  lastName,
  onLastNameChange,
  displayName,
  onDisplayNameChange,
  email,
  onSave,
  saving,
}: {
  avatars?: string[];
  selectedAvatar?: string;
  onSelectAvatar: (key: string) => void;
  firstName: string;
  onFirstNameChange: (value: string) => void;
  lastName: string;
  onLastNameChange: (value: string) => void;
  displayName: string;
  onDisplayNameChange: (value: string) => void;
  email: string;
  onSave: () => void;
  saving: boolean;
}) {
  return (
    <>
      <PanelHeader
        title="Account details"
        description="How you appear across the panel and in team views."
      />

      {avatars && avatars.length > 0 && (
        <SettingRow label="Avatar" hint="Pick a mark for your account.">
          <div className="grid grid-cols-7 gap-2 sm:max-w-md">
            {avatars.map((key) => (
              <button
                key={key}
                type="button"
                onClick={() => onSelectAvatar(key)}
                className={cn(
                  "flex aspect-square items-center justify-center rounded text-xl transition-colors",
                  selectedAvatar === key
                    ? "bg-primary/15 ring-2 ring-primary ring-offset-2 ring-offset-background"
                    : "bg-muted/60 hover:bg-muted",
                )}
                title={key}
              >
                {avatarGlyph(key)}
              </button>
            ))}
          </div>
        </SettingRow>
      )}

      <SettingRow label="Legal name" hint="Used for account records.">
        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            value={firstName}
            onChange={(e) => onFirstNameChange(e.target.value)}
            placeholder="First name"
          />
          <Input
            value={lastName}
            onChange={(e) => onLastNameChange(e.target.value)}
            placeholder="Last name"
          />
        </div>
      </SettingRow>

      <SettingRow label="Display name" hint="Shown in the sidebar and activity feed.">
        <Input
          value={displayName}
          onChange={(e) => onDisplayNameChange(e.target.value)}
          placeholder="Display name"
          className="sm:max-w-md"
        />
      </SettingRow>

      <SettingRow label="Email" hint="Contact your admin to change this address." last>
        <Input value={email} disabled className="sm:max-w-md font-mono opacity-60" />
      </SettingRow>

      <PanelFooter>
        <Button onClick={onSave} disabled={saving}>
          <Save className="h-3.5 w-3.5" />
          {saving ? "Saving..." : "Save changes"}
        </Button>
      </PanelFooter>
    </>
  );
}

// ── Security ────────────────────────────────────────────────────────

function SecurityPanel({
  twoFaEnabled,
  tfaSetupData,
  tfaCode,
  onTfaCodeChange,
  disableCode,
  onDisableCodeChange,
  showDisable2FA,
  onShowDisable2FA,
  secretCopied,
  onCopySecret,
  currentPassword,
  onCurrentPasswordChange,
  newPassword,
  onNewPasswordChange,
  confirmPassword,
  onConfirmPasswordChange,
  passwordError,
  passwordsMatch,
  passwordLongEnough,
  canChangePassword,
  onChangePassword,
  changingPassword,
  onEnable2FA,
  enabling2FA,
  onVerify2FA,
  verifying2FA,
  onDisable2FA,
  disabling2FA,
}: {
  twoFaEnabled: boolean;
  tfaSetupData: { secret: string; qr_code: string } | null;
  tfaCode: string;
  onTfaCodeChange: (value: string) => void;
  disableCode: string;
  onDisableCodeChange: (value: string) => void;
  showDisable2FA: boolean;
  onShowDisable2FA: (value: boolean) => void;
  secretCopied: boolean;
  onCopySecret: () => void;
  currentPassword: string;
  onCurrentPasswordChange: (value: string) => void;
  newPassword: string;
  onNewPasswordChange: (value: string) => void;
  confirmPassword: string;
  onConfirmPasswordChange: (value: string) => void;
  passwordError: string;
  passwordsMatch: boolean;
  passwordLongEnough: boolean;
  canChangePassword: boolean;
  onChangePassword: () => void;
  changingPassword: boolean;
  onEnable2FA: () => void;
  enabling2FA: boolean;
  onVerify2FA: () => void;
  verifying2FA: boolean;
  onDisable2FA: () => void;
  disabling2FA: boolean;
}) {
  return (
    <>
      <PanelHeader
        title="Security"
        description="Keep your account protected with a strong password and two-factor auth."
      />

      <SettingRow label="Password" hint="Use at least 8 characters.">
        <div className="space-y-3 sm:max-w-md">
          <Input
            type="password"
            value={currentPassword}
            onChange={(e) => onCurrentPasswordChange(e.target.value)}
            placeholder="Current password"
          />
          <Input
            type="password"
            value={newPassword}
            onChange={(e) => onNewPasswordChange(e.target.value)}
            placeholder="New password"
          />
          <Input
            type="password"
            value={confirmPassword}
            onChange={(e) => onConfirmPasswordChange(e.target.value)}
            placeholder="Confirm new password"
          />
          {confirmPassword.length > 0 && !passwordsMatch && (
            <p className="text-xs text-destructive">Passwords do not match</p>
          )}
          {newPassword.length > 0 && !passwordLongEnough && (
            <p className="text-xs text-destructive">Password must be at least 8 characters</p>
          )}
          {passwordError && <p className="text-xs text-destructive">{passwordError}</p>}
          <Button
            variant="outline"
            size="sm"
            onClick={onChangePassword}
            disabled={!canChangePassword || changingPassword}
          >
            <Key className="h-3.5 w-3.5" />
            {changingPassword ? "Updating..." : "Update password"}
          </Button>
        </div>
      </SettingRow>

      <SettingRow
        label="Two-factor authentication"
        hint="Require a code from your authenticator app."
        last
      >
        <TwoFactorBlock
          twoFaEnabled={twoFaEnabled}
          tfaSetupData={tfaSetupData}
          tfaCode={tfaCode}
          onTfaCodeChange={onTfaCodeChange}
          disableCode={disableCode}
          onDisableCodeChange={onDisableCodeChange}
          showDisable2FA={showDisable2FA}
          onShowDisable2FA={onShowDisable2FA}
          secretCopied={secretCopied}
          onCopySecret={onCopySecret}
          onEnable2FA={onEnable2FA}
          enabling2FA={enabling2FA}
          onVerify2FA={onVerify2FA}
          verifying2FA={verifying2FA}
          onDisable2FA={onDisable2FA}
          disabling2FA={disabling2FA}
        />
      </SettingRow>
    </>
  );
}

function TwoFactorBlock({
  twoFaEnabled,
  tfaSetupData,
  tfaCode,
  onTfaCodeChange,
  disableCode,
  onDisableCodeChange,
  showDisable2FA,
  onShowDisable2FA,
  secretCopied,
  onCopySecret,
  onEnable2FA,
  enabling2FA,
  onVerify2FA,
  verifying2FA,
  onDisable2FA,
  disabling2FA,
}: {
  twoFaEnabled: boolean;
  tfaSetupData: { secret: string; qr_code: string } | null;
  tfaCode: string;
  onTfaCodeChange: (value: string) => void;
  disableCode: string;
  onDisableCodeChange: (value: string) => void;
  showDisable2FA: boolean;
  onShowDisable2FA: (value: boolean) => void;
  secretCopied: boolean;
  onCopySecret: () => void;
  onEnable2FA: () => void;
  enabling2FA: boolean;
  onVerify2FA: () => void;
  verifying2FA: boolean;
  onDisable2FA: () => void;
  disabling2FA: boolean;
}) {
  if (!twoFaEnabled && !tfaSetupData) {
    return (
      <div className="space-y-3 sm:max-w-lg">
        <p className="text-sm text-muted-foreground">
          Add a second step at login with Google Authenticator, 1Password, or Authy.
        </p>
        <Button size="sm" onClick={onEnable2FA} disabled={enabling2FA}>
          <ShieldCheck className="h-3.5 w-3.5" />
          {enabling2FA ? "Preparing..." : "Set up 2FA"}
        </Button>
      </div>
    );
  }

  if (!twoFaEnabled && tfaSetupData) {
    return (
      <div className="space-y-4 sm:max-w-xl">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div className="shrink-0 self-start rounded border bg-background p-3">
            <QRCodeSVG value={tfaSetupData.qr_code} size={140} />
          </div>
          <div className="min-w-0 flex-1 space-y-3">
            <p className="text-sm text-muted-foreground">
              Scan the code, or paste this secret into your app:
            </p>
            <div className="flex items-center gap-2">
              <code className="flex-1 rounded border bg-muted px-3 py-2 font-mono text-xs break-all">
                {tfaSetupData.secret}
              </code>
              <Button variant="outline" size="sm" onClick={onCopySecret}>
                {secretCopied ? (
                  <Check className="h-3.5 w-3.5" />
                ) : (
                  <Copy className="h-3.5 w-3.5" />
                )}
              </Button>
            </div>
            <div className="flex flex-wrap items-end gap-2">
              <div className="space-y-1.5">
                <Label className="text-xs">Verification code</Label>
                <Input
                  value={tfaCode}
                  onChange={(e) => onTfaCodeChange(e.target.value.replace(/\D/g, "").slice(0, 6))}
                  placeholder="000000"
                  className="w-[140px] font-mono"
                  maxLength={6}
                />
              </div>
              <Button
                size="sm"
                onClick={onVerify2FA}
                disabled={tfaCode.length !== 6 || verifying2FA}
              >
                {verifying2FA ? "Verifying..." : "Enable 2FA"}
              </Button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (twoFaEnabled && !showDisable2FA) {
    return (
      <div className="flex flex-col gap-3 sm:max-w-lg sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-start gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded border border-success/30 bg-success/10">
            <Check className="h-4 w-4 text-success" />
          </div>
          <div>
            <p className="text-sm font-medium">Authenticator app connected</p>
            <p className="text-xs text-muted-foreground">
              A code is required every time you sign in.
            </p>
          </div>
        </div>
        <Button variant="outline" size="sm" onClick={() => onShowDisable2FA(true)}>
          Turn off 2FA
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-3 sm:max-w-md">
      <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3">
        <p className="text-sm font-medium text-destructive">Disable two-factor authentication</p>
        <p className="mt-1 text-xs text-destructive/80">
          Enter your current authenticator code to confirm.
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Input
          value={disableCode}
          onChange={(e) => onDisableCodeChange(e.target.value.replace(/\D/g, "").slice(0, 6))}
          placeholder="000000"
          className="w-[140px] text-center font-mono tracking-widest"
          maxLength={6}
          autoFocus
        />
        <Button
          variant="destructive"
          size="sm"
          onClick={onDisable2FA}
          disabled={disableCode.length !== 6 || disabling2FA}
        >
          {disabling2FA ? "Disabling..." : "Confirm"}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            onShowDisable2FA(false);
            onDisableCodeChange("");
          }}
        >
          Cancel
        </Button>
      </div>
    </div>
  );
}

// ── Preferences ─────────────────────────────────────────────────────

function PreferencesPanel() {
  const [currentTheme, setCurrentTheme] = useState<Theme>(getTheme);

  const options: {
    value: Theme;
    icon: LucideIcon;
    label: string;
    description: string;
    preview: string;
  }[] = [
    {
      value: "dark",
      icon: Moon,
      label: "Dark",
      description: "Deep navy surfaces tuned for long ops sessions.",
      preview: "bg-[#0c1324]",
    },
    {
      value: "light",
      icon: Sun,
      label: "Light",
      description: "High-contrast utility layout for bright environments.",
      preview: "bg-[#f8f9ff]",
    },
    {
      value: "system",
      icon: Monitor,
      label: "System",
      description: "Follow your OS appearance automatically.",
      preview: "bg-gradient-to-br from-[#0c1324] to-[#f8f9ff]",
    },
  ];

  return (
    <>
      <PanelHeader title="Preferences" description="Customize how Orka'i looks on your device." />

      <SettingRow label="Color theme" hint="Applies immediately across the panel." last>
        <div className="grid gap-3 sm:grid-cols-3">
          {options.map((option) => {
            const Icon = option.icon;
            const active = currentTheme === option.value;
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => {
                  setCurrentTheme(option.value);
                  setTheme(option.value);
                }}
                className={cn(
                  "flex flex-col overflow-hidden rounded-lg border text-left transition-colors",
                  active
                    ? "border-primary ring-1 ring-primary/30"
                    : "border-border hover:border-primary/30",
                )}
              >
                <div className={cn("h-14 border-b", option.preview)} />
                <div className="space-y-1 p-3">
                  <span className="flex items-center gap-1.5 text-sm font-medium">
                    <Icon className="h-3.5 w-3.5 text-muted-foreground" />
                    {option.label}
                  </span>
                  <p className="text-xs text-muted-foreground">{option.description}</p>
                </div>
              </button>
            );
          })}
        </div>
      </SettingRow>
    </>
  );
}
