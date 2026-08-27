// Inline SVG icon set. No icon package: the app needs a handful of glyphs and a
// dependency would cost more than it saves in a distroless SPA.
//
// House rules (they are what make icons sit right next to text):
//   - Draw with `currentColor` and never a hardcoded fill/stroke, so hover,
//     selected and disabled states come from CSS colour alone — one asset per
//     glyph, not one per state.
//   - Stroke weight tracks the adjacent text weight: 1.5 beside regular body
//     text, 2 beside semibold. `strokeWidth` is a prop for that reason.
//   - 24px grid, sized at the call site in `em`/Tailwind units so the glyph
//     scales with its label.
import { cn } from "@/lib/utils";

interface IconProps {
  className?: string;
  strokeWidth?: number;
}

function Svg({
  className,
  strokeWidth = 1.5,
  children,
}: IconProps & { children: React.ReactNode }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={cn("h-4 w-4 shrink-0", className)}
    >
      {children}
    </svg>
  );
}

// Disclosure chevron. Points right when closed; the caller rotates it 90° for
// open, which is a transition (interruptible) rather than a keyframe.
export function ChevronRightIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M9 6l6 6-6 6" />
    </Svg>
  );
}

export function XIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M6 6l12 12M18 6L6 18" />
    </Svg>
  );
}

export function CheckIcon(props: IconProps) {
  return (
    <Svg {...props}>
      <path d="M20 6L9 17l-5-5" />
    </Svg>
  );
}
