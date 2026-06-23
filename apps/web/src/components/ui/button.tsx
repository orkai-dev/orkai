import { cva, type VariantProps } from "class-variance-authority";
import * as React from "react";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded text-sm font-medium transition-colors focus-visible:outline-none focus-visible:border-ring disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "border border-primary/90 bg-primary text-primary-foreground hover:bg-primary/95",
        destructive:
          "border border-destructive/90 bg-destructive text-destructive-foreground hover:bg-destructive/95",
        outline: "border border-border bg-card hover:border-primary/50 hover:bg-accent",
        secondary: "border border-border bg-secondary text-secondary-foreground hover:bg-accent",
        ghost: "border border-transparent text-foreground hover:border-border hover:bg-accent",
        link: "text-primary underline-offset-4 hover:underline border-transparent",
      },
      size: {
        default: "h-8 px-3 py-2",
        sm: "h-7 px-2 text-xs",
        lg: "h-9 px-6",
        icon: "h-8 w-8",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, type = "button", ...props }, ref) => {
    return (
      <button
        type={type}
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      />
    );
  },
);
Button.displayName = "Button";

export { Button, buttonVariants };
