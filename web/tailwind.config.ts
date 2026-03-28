import type { Config } from "tailwindcss";

export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
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
        card: "12px",
        btn: "8px",
        badge: "6px",
        pill: "20px",
      },
      fontSize: {
        metric: ["28px", { fontWeight: "700", letterSpacing: "-1px" }],
        "metric-unit": ["13px", { fontWeight: "500" }],
        "section-label": ["10px", { fontWeight: "500", letterSpacing: "0.8px" }],
      },
    },
  },
  plugins: [],
} satisfies Config;
