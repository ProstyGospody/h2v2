import type { Config } from "tailwindcss";

export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}", "./shell/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        surface: {
          0: "var(--surface-0)",
          1: "var(--surface-1)",
          2: "var(--surface-2)",
          3: "var(--surface-3)",
          4: "var(--surface-4)",
        },
        border: {
          DEFAULT: "var(--border)",
          hover: "var(--border-hover)",
          focus: "var(--border-focus)",
        },
        txt: {
          primary: "var(--txt-display)",
          DEFAULT: "var(--txt-body)",
          secondary: "var(--txt-label)",
          tertiary: "var(--txt-utility)",
          muted: "var(--txt-icon)",
        },
        accent: {
          DEFAULT: "var(--primary)",
          light: "var(--primary)",
          secondary: "var(--accent)",
          "secondary-light": "var(--accent)",
        },
        status: {
          success: "var(--status-success)",
          warning: "var(--status-warning)",
          danger: "var(--status-danger)",
          info: "var(--status-info)",
        },
      },
      fontFamily: {
        sans: ["Almost Mono", "JetBrains Mono", "Fira Code", "system-ui", "-apple-system", "sans-serif"],
        mono: ["Almost Mono", "JetBrains Mono", "Fira Code", "monospace"],
      },
      borderRadius: {
        card: "14px",
        btn: "10px",
        badge: "8px",
        pill: "24px",
      },
      fontSize: {
        metric: ["34px", { fontWeight: "700", lineHeight: "1", letterSpacing: "-1px" }],
        "metric-unit": ["15px", { fontWeight: "500", lineHeight: "1" }],
        "section-label": ["11px", { fontWeight: "600", lineHeight: "1", letterSpacing: "0.6px" }],
      },
    },
  },
  plugins: [],
} satisfies Config;
