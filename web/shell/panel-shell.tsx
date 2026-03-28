import { AnimatePresence, motion } from "framer-motion";
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
    if (typeof window !== "undefined") {
      window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? "1" : "0");
    }
  }, [collapsed]);

  const sectionMain = navItems.filter((n) => n.section === "MAIN");
  const sectionSystem = navItems.filter((n) => n.section === "SYSTEM");

  function changeTheme(next: ThemeMode) {
    setTheme(next);
    applyTheme(next);
  }

  async function logout() {
    try {
      await apiFetch<{ ok: boolean }>("/api/auth/logout", { method: "POST", body: JSON.stringify({}) });
    } catch {
      // no-op
    }
    navigate("/login", { replace: true });
  }

  function SidebarNavLink({ item, compact }: { item: NavItem; compact: boolean }) {
    const selected = isActive(pathname, item.href);

    return (
      <button
        type="button"
        title={compact ? item.label : undefined}
        onClick={() => {
          navigate(item.href);
          setMobileOpen(false);
        }}
        className={cn(
          "group relative flex w-full items-center rounded-2xl py-3 transition-all duration-200",
          compact ? "justify-center px-0" : "gap-3 px-4",
          selected
            ? "bg-surface-3/70 text-txt-primary shadow-[inset_0_1px_0_var(--shell-highlight)]"
            : "text-txt-secondary hover:bg-surface-3/45 hover:text-txt-primary",
        )}
      >
        <span className={cn("relative z-10 transition-colors", selected ? "text-accent-light" : "text-txt-tertiary group-hover:text-txt-primary")}>
          {item.icon}
        </span>
        {!compact && <span className="relative z-10 text-[14px] font-semibold">{item.label}</span>}
        {!compact && selected && <ChevronRight size={17} strokeWidth={1.8} className="relative z-10 ml-auto text-accent-light/75" />}
      </button>
    );
  }

  function SidebarContent({ compact, mobile }: { compact: boolean; mobile: boolean }) {
    return (
      <div className={cn("flex h-full flex-col", compact && "items-center")}>
        <div className={cn("flex w-full items-center border-b border-border/50 pb-4 pt-5", compact ? "justify-center px-2" : "justify-between px-5")}>
          <div className={cn("relative", compact ? "h-11 w-11" : "h-12 w-12")}>
            <div className="absolute inset-0 rounded-xl bg-gradient-to-br from-accent to-accent-secondary opacity-25 blur-lg" />
            <div className="relative grid h-full w-full place-items-center rounded-xl bg-gradient-to-br from-accent to-accent-secondary shadow-lg shadow-accent/20">
              <Zap size={compact ? 22 : 24} strokeWidth={2} className="text-white" />
            </div>
          </div>

          {!compact && (
            <div className="min-w-0 flex-1 pl-3">
              <p className="truncate text-[17px] font-bold text-txt-primary">Nexus</p>
              <p className="text-[12px] text-txt-muted">Control Panel</p>
            </div>
          )}

          {!mobile && (
            <button
              type="button"
              onClick={() => setCollapsed((prev) => !prev)}
              className="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-border bg-surface-2/80 text-txt-tertiary transition-colors hover:text-txt-primary"
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
            onClick={() => changeTheme(theme === "dark" ? "light" : "dark")}
            title={compact ? "Toggle theme" : undefined}
            className={cn(
              "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted transition-colors hover:bg-surface-3/40 hover:text-txt-primary",
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
              "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted transition-colors hover:bg-surface-3/40 hover:text-status-danger",
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
          "fixed inset-y-0 left-0 z-30 hidden border-r border-border/50 bg-surface-1 backdrop-blur-xl transition-[width] duration-300 lg:block",
          collapsed ? "w-[96px]" : "w-[288px]",
        )}
      >
        <SidebarContent compact={collapsed} mobile={false} />
      </aside>

      <AnimatePresence>
        {mobileOpen && (
          <motion.div
            className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm lg:hidden"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={() => setMobileOpen(false)}
          >
            <motion.aside
              className="h-full w-[288px] border-r border-border/50 bg-surface-1"
              initial={{ x: -56, opacity: 0 }}
              animate={{ x: 0, opacity: 1 }}
              exit={{ x: -56, opacity: 0 }}
              transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}
              onClick={(event) => event.stopPropagation()}
            >
              <SidebarContent compact={false} mobile />
            </motion.aside>
          </motion.div>
        )}
      </AnimatePresence>

      <div className={cn("transition-[padding] duration-300", collapsed ? "lg:pl-[96px]" : "lg:pl-[288px]")}>
        <header className="sticky top-0 z-20 border-b border-border/40 bg-surface-0/85 px-6 py-4 backdrop-blur-xl lg:hidden">
          <button
            type="button"
            className="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-border bg-surface-1 text-txt-secondary transition-colors hover:text-txt"
            onClick={() => setMobileOpen(true)}
            aria-label="Open menu"
          >
            <PanelLeft size={20} strokeWidth={1.8} />
          </button>
        </header>

        <main className="p-5 md:p-8">
          <AnimatePresence mode="wait">
            <motion.div
              key={pathname}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              transition={{ duration: 0.24, ease: [0.25, 0.1, 0.25, 1] }}
            >
              {children}
            </motion.div>
          </AnimatePresence>
        </main>
      </div>
    </div>
  );
}
