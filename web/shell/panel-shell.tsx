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
import { AnimatePresence, motion } from "framer-motion";
import { type ReactNode, useEffect, useMemo, useState } from "react";
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
  const pageContent = useMemo(() => children, [pathname]);

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
          "group relative flex h-12 w-full items-center rounded-2xl transition-[transform,background-color,color,box-shadow] duration-200 will-change-transform",
          compact ? "justify-center px-0 hover:scale-[1.03]" : "gap-3 px-4 hover:translate-x-0.5",
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
          <div className={cn("relative flex min-w-0 items-center", compact && "mx-auto")}>
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
        </div>

        <div className={cn("w-full", compact ? "px-3" : "px-5")}>
          <div className="h-px bg-gradient-to-r from-transparent via-border to-transparent" />
        </div>

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

          {!mobile && (
            <button
              type="button"
              onClick={() => setCollapsed((prev) => !prev)}
              title={compact ? (collapsed ? "Expand sidebar" : "Collapse sidebar") : undefined}
              className={cn(
                "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted transition-all duration-200 hover:bg-surface-3/40 hover:text-txt-primary",
                compact ? "justify-center px-2 py-2.5" : "gap-3 px-3 py-2.5",
              )}
              aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            >
              {collapsed ? <ChevronRight size={compact ? 22 : 18} strokeWidth={1.8} /> : <ChevronLeft size={compact ? 22 : 18} strokeWidth={1.8} />}
              {!compact && (collapsed ? "Expand sidebar" : "Collapse sidebar")}
            </button>
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

  return (
    <div className="min-h-screen bg-surface-0 text-txt">
      {/* Desktop sidebar */}
      <aside
        className="fixed inset-y-0 left-0 z-30 hidden overflow-x-hidden border-r border-border/30 sidebar-glass lg:block"
        style={{
          width: `${collapsed ? SIDEBAR_WIDTH_COLLAPSED : SIDEBAR_WIDTH_EXPANDED}px`,
          transition: "width 0.34s cubic-bezier(0.22,1,0.36,1)",
        }}
      >
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-accent/30 to-transparent" />
        <SidebarContent compact={collapsed} mobile={false} />
      </aside>

      {/* Main content */}
      <div className={cn("transition-[padding-left] duration-300 ease-out", collapsed ? "lg:pl-[96px]" : "lg:pl-[280px]")}>
        {!mobileOpen && (
          <button
            type="button"
            className="fixed left-4 top-4 z-20 inline-flex h-10 w-10 items-center justify-center rounded-xl border border-border/60 bg-surface-1/95 text-txt-secondary shadow-[0_8px_24px_var(--shell-shadow)] backdrop-blur-lg transition-colors duration-200 hover:bg-surface-2 hover:text-txt lg:hidden"
            onClick={() => setMobileOpen(true)}
            aria-label="Open menu"
          >
            <PanelLeft size={20} strokeWidth={1.8} />
          </button>
        )}

        <main className="p-4 pt-16 sm:p-5 sm:pt-20 md:p-8 md:pt-20 lg:pt-8">
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={pathname}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
            >
              {pageContent}
            </motion.div>
          </AnimatePresence>
        </main>
      </div>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <div className="absolute inset-0 bg-[var(--dialog-overlay)] backdrop-blur-sm" onClick={() => setMobileOpen(false)} />
          <aside className="absolute inset-y-0 left-0 w-[min(280px,100vw)] border-r border-border/30 sidebar-glass shadow-[0_22px_52px_-28px_var(--dialog-shadow)] animate-[slide-in-left_0.25s_ease]">
            <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-accent/30 to-transparent" />
            <SidebarContent compact={false} mobile />
          </aside>
        </div>
      )}
    </div>
  );
}
