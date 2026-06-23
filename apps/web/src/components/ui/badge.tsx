import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-sm border px-1.5 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-widest transition-colors focus:border-ring focus:outline-none",
  {
    variants: {
      variant: {
        default: "border-primary/25 bg-primary/5 text-primary",
        secondary: "border-border bg-muted text-muted-foreground",
        destructive: "border-destructive/25 bg-destructive/5 text-destructive",
        outline: "border-border text-foreground",
        success: "border-success/25 bg-success/5 text-success",
        warning: "border-warning/25 bg-warning/5 text-warning",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export { Badge, badgeVariants };
