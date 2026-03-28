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
const SIDEBAR_EXPANDED_WIDTH = 288;
const SIDEBAR_RAIL_WIDTH = 96;
const SIDEBAR_SHIFT = SIDEBAR_EXPANDED_WIDTH - SIDEBAR_RAIL_WIDTH;

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

  function SidebarNavLink({ item, compact }: { item: NavItem; compact: boolean }) {
    const selected = isActive(pathname, item.href);

    return (
      <button
        type="button"
        title={compact ? item.label : undefined}
        onClick={() => navigate(item.href)}
        className={cn(
          "group flex h-12 w-full items-center rounded-2xl",
          compact ? "justify-center px-0" : "gap-3 px-4",
          selected
            ? "bg-surface-3/70 text-txt-primary shadow-[inset_0_1px_0_var(--shell-highlight)]"
            : "text-txt-secondary hover:bg-surface-3/45 hover:text-txt-primary",
        )}
      >
        <span className={cn("shrink-0", selected ? "text-accent-light" : "text-txt-tertiary group-hover:text-txt-primary")}>
          {item.icon}
        </span>

        <span
          className={cn(
            "text-[14px] font-semibold whitespace-nowrap transition-opacity duration-150",
            compact ? "w-0 overflow-hidden opacity-0" : "opacity-100",
          )}
        >
          {item.label}
        </span>

        {!compact && selected && <ChevronRight size={17} strokeWidth={1.8} className="ml-auto shrink-0 text-accent-light/75" />}
      </button>
    );
  }

  function SidebarContent({ compact, mobile }: { compact: boolean; mobile: boolean }) {
    return (
      <div className={cn("flex h-full flex-col", compact && "items-center")}>
        <div className={cn("flex w-full items-center border-b border-border/50 pb-4 pt-5", compact ? "justify-center px-2" : "justify-between px-5")}>
          <div className={cn("grid place-items-center rounded-xl bg-gradient-to-br from-accent to-accent-secondary", compact ? "h-11 w-11" : "h-12 w-12")}>
            <Zap size={compact ? 22 : 24} strokeWidth={2} className="text-white" />
          </div>

          <div
            className={cn(
              "min-w-0 flex-1 pl-3 transition-opacity duration-150",
              compact && "w-0 overflow-hidden pl-0 opacity-0",
            )}
          >
            <p className="truncate text-[17px] font-bold text-txt-primary">Nexus</p>
            <p className="text-[12px] text-txt-muted">Control Panel</p>
          </div>

          {!mobile && (
            <button
              type="button"
              onClick={() => setCollapsed((prev) => !prev)}
              className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-border bg-surface-2/80 text-txt-tertiary hover:text-txt-primary"
              aria-label={compact ? "Expand sidebar" : "Collapse sidebar"}
            >
              {compact ? <ChevronRight size={20} strokeWidth={2} /> : <ChevronLeft size={20} strokeWidth={2} />}
            </button>
          )}
        </div>

        <nav className={cn("w-full flex-1 overflow-y-auto pt-4", compact ? "px-2" : "px-3")}>
          <div>
            {!compact && <p className="mb-2 px-4 text-section-label uppercase text-txt-muted">Main</p>}
            <div className="space-y-1.5">
              {sectionMain.map((item) => (
                <SidebarNavLink key={item.href} item={item} compact={compact} />
              ))}
            </div>
          </div>

          <div className="mt-6">
            {!compact && <p className="mb-2 px-4 text-section-label uppercase text-txt-muted">System</p>}
            <div className="space-y-1.5">
              {sectionSystem.map((item) => (
                <SidebarNavLink key={item.href} item={item} compact={compact} />
              ))}
            </div>
          </div>
        </nav>

        <div className={cn("w-full border-t border-border/50 p-3", compact ? "space-y-2" : "space-y-3")}>
          {!compact && (
            <div className="flex items-center gap-3 rounded-xl bg-surface-3/40 px-3 py-3">
              <div className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-accent/20 to-accent-secondary/20 text-[14px] font-bold text-accent-light">
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
              "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted hover:bg-surface-3/40 hover:text-txt-primary",
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
              "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted hover:bg-surface-3/40 hover:text-status-danger",
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

  return (
    <div className="min-h-screen bg-surface-0 text-txt">
      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-30 hidden border-r border-border/50 bg-surface-1 shadow-[inset_0_1px_0_var(--shell-highlight)] lg:block",
          "transform-gpu will-change-transform motion-reduce:transition-none",
          "transition-transform duration-200 ease-out",
        )}
        style={{
          width: `${SIDEBAR_EXPANDED_WIDTH}px`,
          transform: collapsed ? `translateX(-${SIDEBAR_SHIFT}px)` : "translateX(0)",
        }}
      >
        <SidebarContent compact={collapsed} mobile={false} />
      </aside>

      <div className="lg:pl-[96px]">
        <header className="sticky top-0 z-20 border-b border-border/40 bg-surface-0/90 px-6 py-4 lg:hidden">
          <button
            type="button"
            className="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-border bg-surface-1 text-txt-secondary hover:text-txt"
            onClick={() => setMobileOpen(true)}
            aria-label="Open menu"
          >
            <PanelLeft size={20} strokeWidth={1.8} />
          </button>
        </header>

        <main className="p-5 md:p-8">{children}</main>
      </div>

      <div
        className={cn(
          "fixed inset-0 z-50 lg:hidden",
          "transition-opacity duration-200 ease-out motion-reduce:transition-none",
          mobileOpen ? "pointer-events-auto opacity-100" : "pointer-events-none opacity-0",
        )}
      >
        <div className="absolute inset-0 bg-black/55" onClick={() => setMobileOpen(false)} />
        <aside
          className={cn(
            "absolute inset-y-0 left-0 h-full border-r border-border/50 bg-surface-1 shadow-2xl shadow-black/30",
            "transform-gpu will-change-transform motion-reduce:transition-none",
            "transition-transform duration-220 ease-out",
            mobileOpen ? "translate-x-0" : "-translate-x-full",
          )}
          style={{ width: `${SIDEBAR_EXPANDED_WIDTH}px` }}
        >
          <SidebarContent compact={false} mobile />
        </aside>
      </div>
    </div>
  );
}
