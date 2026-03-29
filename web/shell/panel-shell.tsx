import {
  Activity,
  ChevronLeft,
  ChevronRight,
  HardDrive,
  LogOut,
  Moon,
  PanelLeft,
  Shield,
  SlidersHorizontal,
  Sun,
  Zap,
} from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { cn } from "@/src/components/ui";
import { applyTheme, resolveTheme, type ThemeMode } from "@/src/theme";
import { apiFetch } from "@/services/api";

type NavItem = {
  href: string;
  label: string;
  icon: ReactNode;
  section: "MAIN" | "SYSTEM";
};

const SIDEBAR_COLLAPSED_KEY = "panel-sidebar-collapsed";
const SIDEBAR_WIDTH_EXPANDED = 280;
const SIDEBAR_WIDTH_COLLAPSED = 96;

const navItems: NavItem[] = [
  { href: "/", label: "Dashboard", icon: <Activity size={24} strokeWidth={1.8} />, section: "MAIN" },
  { href: "/users", label: "Users", icon: <Shield size={24} strokeWidth={1.8} />, section: "MAIN" },
  { href: "/config", label: "Settings", icon: <SlidersHorizontal size={24} strokeWidth={1.8} />, section: "SYSTEM" },
  { href: "/audit", label: "Audit Log", icon: <HardDrive size={24} strokeWidth={1.8} />, section: "SYSTEM" },
];

function isActive(pathname: string, href: string): boolean {
  if (href === "/") return pathname === "/";
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function PanelShell({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const { pathname } = useLocation();

  const [mobileOpen, setMobileOpen] = useState(false);
  const [theme, setTheme] = useState<ThemeMode>(() => resolveTheme());
  const [collapsed, setCollapsed] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    return window.localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "1";
  });

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  useEffect(() => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? "1" : "0");
    }
  }, [collapsed]);

  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

  async function logout() {
    try {
      await apiFetch<{ ok: boolean }>("/api/auth/logout", { method: "POST", body: JSON.stringify({}) });
    } catch {
      // no-op
    }
    navigate("/login", { replace: true });
  }

  const sectionMain = navItems.filter((item) => item.section === "MAIN");
  const sectionSystem = navItems.filter((item) => item.section === "SYSTEM");

  function SidebarNavLink({ item, compact, onNavigate }: { item: NavItem; compact: boolean; onNavigate?: () => void }) {
    const selected = isActive(pathname, item.href);

    return (
      <button
        type="button"
        title={compact ? item.label : undefined}
        onClick={() => {
          navigate(item.href);
          onNavigate?.();
        }}
        className={cn(
          "group relative flex h-12 w-full items-center rounded-2xl transition-all duration-200",
          compact ? "justify-center px-0" : "gap-3 px-4",
          selected
            ? "bg-surface-3/70 text-txt-primary shadow-[inset_0_1px_0_var(--shell-highlight),0_2px_8px_var(--shell-shadow)]"
            : "text-txt-secondary hover:bg-surface-3/45 hover:text-txt-primary",
        )}
      >
        {selected && (
          <span className="absolute left-0 top-1/2 h-5 w-[3px] -translate-y-1/2 rounded-r-full bg-gradient-to-b from-accent to-accent-secondary" />
        )}
        <span className={cn("shrink-0 transition-colors duration-200", selected ? "text-accent-secondary" : "text-txt-tertiary group-hover:text-txt-primary")}>
          {item.icon}
        </span>

        {!compact && <span className="text-[14px] font-semibold whitespace-nowrap">{item.label}</span>}
      </button>
    );
  }

  function SidebarContent({ compact, mobile }: { compact: boolean; mobile: boolean }) {
    return (
      <div className={cn("flex h-full flex-col", compact && "items-center")}>
        <div className={cn("relative flex w-full items-center pb-4 pt-5", compact ? "justify-center px-2" : "justify-start px-5")}>
          <div className="relative">
            <div className="absolute inset-0 rounded-xl bg-gradient-to-br from-accent to-accent-secondary opacity-30 blur-lg" />
            <div className={cn("relative grid place-items-center rounded-xl bg-gradient-to-br from-accent to-accent-secondary shadow-lg shadow-accent/20", compact ? "h-11 w-11" : "h-12 w-12")}>
              <Zap size={compact ? 22 : 24} strokeWidth={2} className="text-white" />
            </div>
          </div>

          {!compact && (
            <div className="min-w-0 flex-1 pl-3">
              <p className="truncate text-[17px] font-bold text-txt-primary">Nexus</p>
              <p className="text-[12px] text-txt-muted">Control Panel</p>
            </div>
          )}
        </div>

        <div className={cn("w-full", compact ? "px-3" : "px-5")}>
          <div className="h-px bg-gradient-to-r from-transparent via-border to-transparent" />
        </div>

        {!mobile && (
          <div className={cn("w-full pb-1 pt-2", compact ? "px-2" : "px-4")}>
            <div className={cn("flex", compact ? "justify-center" : "justify-end")}>
              <button
                type="button"
                onClick={() => setCollapsed((prev) => !prev)}
                className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-txt-muted opacity-50 transition-all duration-200 hover:opacity-100 hover:bg-surface-3/40 hover:text-txt-secondary"
                aria-label={compact ? "Expand sidebar" : "Collapse sidebar"}
              >
                {compact ? <ChevronRight size={15} strokeWidth={1.6} /> : <ChevronLeft size={15} strokeWidth={1.6} />}
              </button>
            </div>
          </div>
        )}

        <nav className={cn("w-full flex-1 overflow-y-auto pt-4", compact ? "px-2" : "px-3")}>
          <div>
            {!compact && <p className="mb-2 px-4 text-section-label uppercase text-txt-muted">Main</p>}
            <div className="space-y-1.5">
              {sectionMain.map((item) => (
                <SidebarNavLink key={item.href} item={item} compact={compact} onNavigate={() => setMobileOpen(false)} />
              ))}
            </div>
          </div>

          <div className="mt-6">
            {!compact && <p className="mb-2 px-4 text-section-label uppercase text-txt-muted">System</p>}
            <div className="space-y-1.5">
              {sectionSystem.map((item) => (
                <SidebarNavLink key={item.href} item={item} compact={compact} onNavigate={() => setMobileOpen(false)} />
              ))}
            </div>
          </div>
        </nav>

        <div className={cn("w-full p-3", compact ? "space-y-2" : "space-y-2")}>
          <div className={cn("w-full mb-2", compact ? "px-1" : "px-0")}>
            <div className="h-px bg-gradient-to-r from-transparent via-border to-transparent" />
          </div>

          {!compact && (
            <div className="flex items-center gap-3 rounded-xl border border-border/30 bg-surface-3/30 px-3 py-3 transition-colors duration-200 hover:bg-surface-3/50">
              <div className="relative grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-accent/20 to-accent-secondary/20 text-[14px] font-bold text-txt-primary">
                <span className="absolute -right-0.5 -top-0.5 h-2.5 w-2.5 rounded-full border-2 border-surface-0 bg-status-success" />
                A
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-[13px] font-semibold text-txt">Admin</p>
                <p className="truncate text-[12px] text-txt-muted">root@nexus</p>
              </div>
            </div>
          )}

          <button
            type="button"
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            title={compact ? "Toggle theme" : undefined}
            className={cn(
              "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted transition-all duration-200 hover:bg-surface-3/40 hover:text-txt-primary",
              compact ? "justify-center px-2 py-2.5" : "gap-3 px-3 py-2.5",
            )}
          >
            {theme === "dark" ? <Sun size={compact ? 22 : 18} strokeWidth={1.8} /> : <Moon size={compact ? 22 : 18} strokeWidth={1.8} />}
            {!compact && (theme === "dark" ? "Light theme" : "Dark theme")}
          </button>

          <button
            type="button"
            onClick={() => void logout()}
            title={compact ? "Sign out" : undefined}
            className={cn(
              "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted transition-all duration-200 hover:bg-status-danger/8 hover:text-status-danger",
              compact ? "justify-center px-2 py-2.5" : "gap-3 px-3 py-2.5",
            )}
          >
            <LogOut size={compact ? 22 : 18} strokeWidth={1.8} />
            {!compact && "Sign out"}
          </button>
        </div>
      </div>
    );
  }

  const sidebarWidth = collapsed ? SIDEBAR_WIDTH_COLLAPSED : SIDEBAR_WIDTH_EXPANDED;

  return (
    <div className="min-h-screen bg-surface-0 text-txt">
      {/* Desktop sidebar */}
      <aside
        className="fixed inset-y-0 left-0 z-30 hidden border-r border-border/30 sidebar-glass lg:block"
        style={{ width: sidebarWidth, transition: "width 0.25s cubic-bezier(0.4,0,0.2,1)" }}
      >
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-accent/30 to-transparent" />
        <SidebarContent compact={collapsed} mobile={false} />
      </aside>

      {/* Main content area */}
      <div
        className="max-lg:!pl-0 transition-[padding] duration-[250ms] ease-[cubic-bezier(0.4,0,0.2,1)]"
        style={{ paddingLeft: sidebarWidth }}
      >
        <header className="sticky top-0 z-20 border-b border-border/30 bg-surface-0/90 px-6 py-4 backdrop-blur-lg lg:hidden">
          <button
            type="button"
            className="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-border/60 bg-surface-1 text-txt-secondary transition-colors duration-200 hover:bg-surface-2 hover:text-txt"
            onClick={() => setMobileOpen(true)}
            aria-label="Open menu"
          >
            <PanelLeft size={20} strokeWidth={1.8} />
          </button>
        </header>

        <main className="p-5 md:p-8">{children}</main>
      </div>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <div className="absolute inset-0 bg-black/55 backdrop-blur-sm" onClick={() => setMobileOpen(false)} />
          <aside className="absolute inset-y-0 left-0 w-[280px] border-r border-border/30 sidebar-glass shadow-2xl shadow-black/30 animate-[slide-in-left_0.25s_ease]">
            <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-accent/30 to-transparent" />
            <SidebarContent compact={false} mobile />
          </aside>
        </div>
      )}
    </div>
  );
}
