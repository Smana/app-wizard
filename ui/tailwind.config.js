/** @type {import('tailwindcss').Config} */
// Neutral default palette. Light theme is the default; a `.dark` block maps
// surfaces to a deep navy. Every colour is wired through a CSS variable declared
// in src/index.css, so a deployment restyles the wizard via branding.theme in
// wizard.yaml without a rebuild.
export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        border: "var(--border)",
        input: "var(--input)",
        ring: "var(--ring)",
        background: "var(--background)",
        foreground: "var(--foreground)",
        muted: "var(--muted)",
        "muted-foreground": "var(--muted-foreground)",
        primary: "var(--primary)",
        "primary-foreground": "var(--primary-foreground)",
        accent: "var(--accent)",
        "accent-foreground": "var(--accent-foreground)",
        success: "var(--success)",
        "success-foreground": "var(--success-foreground)",
        destructive: "var(--destructive)",
        "destructive-foreground": "var(--destructive-foreground)",
        warning: "var(--warning)",
        "warning-foreground": "var(--warning-foreground)",
        card: "var(--card)",
        "card-foreground": "var(--card-foreground)",
        placeholder: "var(--placeholder)",
        brand: {
          navy: "var(--brand-navy)",
          "navy-fg": "var(--brand-navy-foreground)",
        },
      },
      // Concentric scale: a nested surface's radius is the parent's minus the
      // padding between them, so corners stay parallel instead of pinching.
      // Cards use `xl` (12px) around `p-4`-separated `md` (6px) controls.
      borderRadius: {
        xl: "0.75rem",
        lg: "0.5rem",
        md: "calc(0.5rem - 2px)",
        sm: "calc(0.5rem - 4px)",
      },
      boxShadow: {
        border: "var(--shadow-border)",
        "border-hover": "var(--shadow-border-hover)",
      },
    },
  },
  plugins: [],
};
