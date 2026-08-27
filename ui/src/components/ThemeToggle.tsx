import type { ReactElement } from "react";
import { Button } from "@/components/ui/button";
import { nextMode, useTheme, type ThemeMode } from "@/lib/theme";

// Inline SVGs rather than an icon package: the app has no icon dependency today,
// and three glyphs do not justify adding one to the bundle. 2px stroke because
// these render standalone in an icon button, not beside body text.

function SunIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true" className="h-5 w-5">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" strokeLinecap="round" />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true" className="h-5 w-5">
      <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function SystemIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true" className="h-5 w-5">
      <rect x="2" y="4" width="20" height="13" rx="2" />
      <path d="M8 21h8M12 17v4" strokeLinecap="round" />
    </svg>
  );
}

const ICONS: Record<ThemeMode, () => ReactElement> = {
  light: SunIcon,
  dark: MoonIcon,
  system: SystemIcon,
};

const LABELS: Record<ThemeMode, string> = {
  light: "Theme: light",
  dark: "Theme: dark",
  system: "Theme: system",
};

const MODES: ThemeMode[] = ["light", "dark", "system"];

export function ThemeToggle() {
  const { mode, setMode } = useTheme();

  // The label names the CURRENT mode, not the next one. A button that announces
  // where you are is honest; one that announces where clicking takes you leaves a
  // screen-reader user unable to tell what the theme actually is.
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      aria-label={LABELS[mode]}
      title={`${LABELS[mode]} — click to switch`}
      onClick={() => setMode(nextMode(mode))}
      className="relative text-brand-navy-fg hover:bg-white/10"
    >
      {/* All three glyphs stay mounted and cross-fade, so the swap has an exit as
          well as an enter. Toggling visibility would pop. There is no motion
          library in this project, so it's a CSS transition on opacity/scale/blur
          with the standard decelerate curve. */}
      {MODES.map((m) => {
        const Icon = ICONS[m];
        const active = m === mode;
        return (
          <span
            key={m}
            aria-hidden="true"
            className="absolute inset-0 flex items-center justify-center transition-[opacity,scale,filter] duration-300 ease-[cubic-bezier(0.2,0,0,1)]"
            style={{
              opacity: active ? 1 : 0,
              scale: active ? "1" : "0.25",
              filter: active ? "blur(0px)" : "blur(4px)",
            }}
          >
            <Icon />
          </span>
        );
      })}
    </Button>
  );
}
