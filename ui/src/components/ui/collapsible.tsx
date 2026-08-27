import { useState } from "react";
import { cn } from "@/lib/utils";
import { ChevronRightIcon } from "./icons";

interface CollapsibleProps {
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  defaultOpen?: boolean;
  badge?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

// Minimal shadcn-styled collapsible (no Radix) — used for the advanced/expert
// tier group sections.
export function Collapsible({
  title,
  subtitle,
  defaultOpen = false,
  badge,
  children,
  className,
}: CollapsibleProps) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    // Depth from the shadow ring, matching Card; the divider below the header
    // stays a border, because separating two regions IS its job.
    <div className={cn("rounded-xl bg-card shadow-border", className)}>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className={cn(
          // The header button sits flush inside the container, so it carries the
          // SAME radius rather than a smaller one — there is no padding between
          // them for a concentric inset to account for. It also has to be `xl`,
          // not `lg`: a smaller radius here would let the hover fill square off
          // the container's rounded corners.
          "flex w-full items-center justify-between gap-2 rounded-xl px-4 py-3 text-left",
          "transition-colors duration-150 ease-out hover:bg-muted",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        )}
      >
        <span className="flex items-center gap-2">
          <ChevronRightIcon
            // 2px stroke to match the semibold-ish label beside it. Rotating via
            // a transition (not a keyframe) means a fast double-click retargets
            // smoothly instead of restarting.
            strokeWidth={2}
            className={cn(
              "text-muted-foreground transition-transform duration-150 ease-out",
              open && "rotate-90",
            )}
          />
          <span className="text-sm font-medium">{title}</span>
          {badge}
        </span>
        {subtitle && <span className="text-xs tabular-nums text-muted-foreground">{subtitle}</span>}
      </button>
      {open && <div className="space-y-4 border-t border-border px-4 py-4">{children}</div>}
    </div>
  );
}

export function Switch({
  checked,
  onCheckedChange,
  id,
  disabled,
}: {
  checked: boolean;
  onCheckedChange: (v: boolean) => void;
  id?: string;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      id={id}
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        // The BUTTON is the hit target and is a real 40px tall, so it occupies
        // the space it claims. The 24px pill is drawn by the inner span.
        //
        // Not a ::after overlay: that version spilled ~10px above and below the
        // control, painting over the neighbouring field's label and eating
        // clicks meant for it. Two controls' hit areas must never overlap, and
        // an overlay that the layout knows nothing about cannot honour that.
        "group inline-flex h-10 shrink-0 items-center rounded-md",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        "disabled:cursor-not-allowed disabled:opacity-50",
      )}
    >
      <span
        className={cn(
          "pointer-events-none inline-flex h-6 w-11 items-center rounded-full border",
          "transition-colors duration-150 ease-out",
          // Themed tokens, not hardcoded slate: the off state used to be a light
          // grey track that floated on the dark navy surface.
          checked ? "border-primary bg-primary" : "border-border bg-muted",
        )}
      >
        <span
          className={cn(
            "inline-block h-5 w-5 transform rounded-full shadow-border transition-transform duration-150 ease-out",
            checked ? "translate-x-[22px] bg-primary-foreground" : "translate-x-0.5 bg-background",
          )}
        />
      </span>
    </button>
  );
}
