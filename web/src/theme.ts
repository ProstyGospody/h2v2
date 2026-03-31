export type ThemeMode = "light" | "dark";

const THEME_KEY = "panel-theme";
const THEME_TRANSITION_CLASS = "theme-transition";
const THEME_TRANSITION_MS = 260;
let transitionTimer: number | null = null;

function isThemeMode(value: string | null): value is ThemeMode {
  return value === "light" || value === "dark";
}

export function resolveTheme(): ThemeMode {
  if (typeof window === "undefined") return "dark";

  const stored = window.localStorage.getItem(THEME_KEY);
  if (isThemeMode(stored)) return stored;

  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function applyTheme(theme: ThemeMode): void {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  root.classList.add(THEME_TRANSITION_CLASS);
  if (transitionTimer !== null && typeof window !== "undefined") {
    window.clearTimeout(transitionTimer);
  }
  if (typeof window !== "undefined") {
    transitionTimer = window.setTimeout(() => {
      root.classList.remove(THEME_TRANSITION_CLASS);
      transitionTimer = null;
    }, THEME_TRANSITION_MS);
  }
  root.classList.toggle("dark", theme === "dark");

  if (typeof window !== "undefined") {
    window.localStorage.setItem(THEME_KEY, theme);
  }
}
