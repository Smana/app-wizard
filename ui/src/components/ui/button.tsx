import { forwardRef } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  [
    "inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium",
    // Never `transition-all`: naming the properties keeps the browser off
    // layout-affecting ones and makes the intent readable.
    "transition-[background-color,border-color,color,box-shadow,scale] duration-150 ease-out",
    // Tactile press. 0.96 is the floor — below ~0.95 the control reads as
    // bouncing rather than depressing. Suppressed under reduced-motion by the
    // global media query in index.css.
    "active:scale-[0.96]",
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
    "disabled:pointer-events-none disabled:opacity-50 disabled:active:scale-100",
  ].join(" "),
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90",
        // The outline variant's edge is decorative depth, so it is a shadow ring
        // that adapts to the surface under it rather than a fixed border colour.
        outline: "bg-background shadow-border hover:bg-muted hover:shadow-border-hover",
        ghost: "hover:bg-muted",
        destructive: "bg-destructive text-destructive-foreground hover:bg-destructive/90",
      },
      size: {
        // 40px minimum on every interactive size: the dense-desktop floor for a
        // hit area. `sm` was h-8 (32px) and `icon` h-9 (36px) — both were small
        // enough to miss, and this form is full of them.
        default: "h-10 px-4 py-2",
        sm: "h-10 rounded-md px-3 text-xs",
        icon: "h-10 w-10",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />
  ),
);
Button.displayName = "Button";
