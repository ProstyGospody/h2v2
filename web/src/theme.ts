export type ThemeMode = "light" | "dark";

const THEME_KEY = "panel-theme";

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
  document.documentElement.classList.toggle("dark", theme === "dark");

  if (typeof window !== "undefined") {
    window.localStorage.setItem(THEME_KEY, theme);
  }
}
