import type { Config } from "tailwindcss";

export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        surface: {
          0: "#0c0c0f",
          1: "#111115",
          2: "#18181c",
          3: "#1e1e24",
          4: "#27272a",
        },
        border: {
          DEFAULT: "#1e1e24",
          hover: "#27272a",
          focus: "#3f3f46",
        },
        txt: {
          primary: "#ffffff",
          DEFAULT: "#e4e4e7",
          secondary: "#a1a1aa",
          tertiary: "#71717a",
          muted: "#52525a",
        },
        accent: {
          DEFAULT: "#6366f1",
          light: "#818cf8",
          secondary: "#8b5cf6",
          "secondary-light": "#a78bfa",
        },
        status: {
          success: "#34d399",
          warning: "#f59e0b",
          danger: "#ef4444",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "-apple-system", "sans-serif"],
        mono: ["JetBrains Mono", "Fira Code", "monospace"],
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
