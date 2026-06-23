import type { LucideIcon } from "lucide-react";
import { Logo } from "@/components/logo";
import { Label } from "@/components/ui/label";
import { BRAND_NAME } from "@/lib/brand";
import { cn } from "@/lib/utils";

interface AuthShellProps {
  children: React.ReactNode;
  title: string;
  description?: string;
  wide?: boolean;
}

function OrcaBackground() {
  return (
    <div className="absolute inset-0 z-0 overflow-hidden pointer-events-none select-none">
      <style>{`
        @keyframes float-slow {
          0%, 100% { transform: translateY(0px) rotate(-12deg); }
          50% { transform: translateY(-15px) rotate(-10deg); }
        }
        @keyframes float-slower {
          0%, 100% { transform: translateY(0px) rotate(-25deg); }
          50% { transform: translateY(18px) rotate(-27deg); }
        }
        .animate-float-slow {
          animation: float-slow 15s ease-in-out infinite;
        }
        .animate-float-slower {
          animation: float-slower 20s ease-in-out infinite;
        }
      `}</style>

      {/* Top Left Orca */}
      <svg
        viewBox="0 0 1000 600"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        className="absolute -top-[5%] -left-[10%] w-[55%] h-[55%] min-w-[320px] max-w-[500px] text-primary/4 dark:text-primary/[0.03] animate-float-slower transition-all duration-1000"
      >
        <title>Orca background</title>
        {/* Main Body */}
        <path
          d="M100,300 C150,220 250,150 400,160 C420,130 450,80 490,40 C505,25 515,35 510,55 C495,110 480,180 500,210 C620,220 750,270 850,330 C870,300 890,280 910,290 C905,325 895,360 905,390 C885,395 865,380 855,365 C780,330 650,290 550,290 C450,290 380,350 280,410 C240,430 180,420 150,390 C120,360 90,320 100,300 Z"
          fill="currentColor"
        />
        {/* Eye Patch */}
        <path
          d="M180,240 C205,232 225,235 230,243 C222,251 198,255 180,245 Z"
          fill="var(--color-background)"
          className="opacity-80"
        />
        {/* Saddle Patch */}
        <path
          d="M525,215 C545,223 570,240 578,255 C558,260 538,245 520,230 Z"
          fill="currentColor"
          className="opacity-40"
        />
      </svg>

      {/* Bottom Right Orca */}
      <svg
        viewBox="0 0 1000 600"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        className="absolute -bottom-[8%] -right-[12%] w-[65%] h-[65%] min-w-[380px] max-w-[650px] text-primary/4 dark:text-primary/[0.03] animate-float-slow transition-all duration-1000"
      >
        <title>Orca background</title>
        {/* Main Body */}
        <path
          d="M100,300 C150,220 250,150 400,160 C420,130 450,80 490,40 C505,25 515,35 510,55 C495,110 480,180 500,210 C620,220 750,270 850,330 C870,300 890,280 910,290 C905,325 895,360 905,390 C885,395 865,380 855,365 C780,330 650,290 550,290 C450,290 380,350 280,410 C240,430 180,420 150,390 C120,360 90,320 100,300 Z"
          fill="currentColor"
        />
        {/* Eye Patch */}
        <path
          d="M180,240 C205,232 225,235 230,243 C222,251 198,255 180,245 Z"
          fill="var(--color-background)"
          className="opacity-80"
        />
        {/* Saddle Patch */}
        <path
          d="M525,215 C545,223 570,240 578,255 C558,260 538,245 520,230 Z"
          fill="currentColor"
          className="opacity-40"
        />
      </svg>
    </div>
  );
}

export function AuthShell({ children, title, description, wide }: AuthShellProps) {
  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center bg-background px-4 py-12 sm:px-6 lg:px-8 overflow-hidden font-mono">
      {/* Background patterns */}
      <div className="auth-grid-bg pointer-events-none absolute inset-0" aria-hidden="true" />
      <div
        className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent"
        aria-hidden="true"
      />

      {/* Central subtle grounded glow behind card */}
      <div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[480px] h-[480px] bg-primary/[0.04] dark:bg-primary/[0.02] rounded-full blur-3xl pointer-events-none"
        aria-hidden="true"
      />

      {/* Beautiful Orca SVGs swimming in the background */}
      <OrcaBackground />

      <div className={cn("w-full z-10 animate-fade-in", wide ? "max-w-xl" : "max-w-md")}>
        {/* Floating Translucent Login Card */}
        <div className="border border-border/80 bg-card/65 backdrop-blur-md shadow-2xl rounded-md">
          {/* Header (Branding Container) */}
          <header className="border-b border-border/60 pt-6 pb-4 px-6 sm:px-8 flex flex-col items-center justify-center text-center">
            <Logo className="h-11 w-11 text-primary mb-1.5" />
            <span className="text-xs font-mono font-semibold tracking-[0.25em] text-muted-foreground/70 uppercase">
              {BRAND_NAME}
            </span>
          </header>

          {/* Body (Form Container) - with tightened top and slightly elevated bottom padding */}
          <div className="px-6 pt-3 pb-8 sm:px-8 sm:pb-9 space-y-4">
            <div className="space-y-0.5 text-center">
              <h1 className="text-headline-sm font-semibold tracking-tight text-foreground">
                {title}
              </h1>
              {description && (
                <p className="text-xs text-muted-foreground/85 leading-normal max-w-xs mx-auto">
                  {description}
                </p>
              )}
            </div>
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}

export function AuthField({
  id,
  label,
  icon: Icon,
  children,
}: {
  id: string;
  label: string;
  icon?: LucideIcon;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id} className="text-label-caps text-muted-foreground">
        {label}
      </Label>
      <div className="relative">
        {Icon && (
          <Icon className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
        )}
        {children}
      </div>
    </div>
  );
}

export function AuthError({ message }: { message: string }) {
  return (
    <div className="border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
      {message}
    </div>
  );
}

export function AuthDivider({ label = "or" }: { label?: string }) {
  return (
    <div className="relative py-2">
      <div className="absolute inset-x-0 top-1/2 h-px bg-border" />
      <p className="relative mx-auto w-fit bg-card px-3 text-label-caps text-muted-foreground">
        {label}
      </p>
    </div>
  );
}

export function AuthLinkButton({
  children,
  onClick,
}: {
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full border border-transparent py-1 text-center text-sm text-primary transition-colors hover:border-primary/30 hover:bg-primary/5"
    >
      {children}
    </button>
  );
}
