import { AnimatePresence, motion } from "framer-motion";
import {
  ChevronRight,
  FileText,
  LayoutGrid,
  LogOut,
  Moon,
  PanelLeft,
  Settings2,
  Sun,
  Users2,
  Zap,
} from "lucide-react";
import { type ReactNode, useMemo, useState } from "react";
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

const navItems: NavItem[] = [
  { href: "/", label: "Dashboard", icon: <LayoutGrid size={20} strokeWidth={1.6} />, section: "MAIN" },
  { href: "/users", label: "Users", icon: <Users2 size={20} strokeWidth={1.6} />, section: "MAIN" },
  { href: "/config", label: "Settings", icon: <Settings2 size={20} strokeWidth={1.6} />, section: "SYSTEM" },
  { href: "/audit", label: "Audit Log", icon: <FileText size={20} strokeWidth={1.6} />, section: "SYSTEM" },
];

function isActive(pathname: string, href: string): boolean {
  if (href === "/") return pathname === "/";
  return pathname === href || pathname.startsWith(`${href}/`);
}

function resolveTitle(pathname: string): string {
  if (pathname === "/") return "Dashboard";
  return navItems.find((item) => item.href === pathname)?.label || "Panel";
}

export function PanelShell({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [theme, setTheme] = useState<ThemeMode>(() => resolveTheme());

  const activeTitle = useMemo(() => resolveTitle(pathname), [pathname]);

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

  const sectionMain = navItems.filter((n) => n.section === "MAIN");
  const sectionSystem = navItems.filter((n) => n.section === "SYSTEM");

  function NavLink({ item }: { item: NavItem }) {
    const selected = isActive(pathname, item.href);
    return (
      <button
        type="button"
        onClick={() => { navigate(item.href); setMobileOpen(false); }}
        className={cn(
          "group relative flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-[14px] font-medium transition-all duration-200",
          selected
            ? "bg-accent/10 text-accent-light"
            : "text-txt-secondary hover:bg-surface-3/50 hover:text-txt",
        )}
      >
        {selected && (
          <motion.div
            layoutId="sidebar-active"
            className="absolute inset-0 rounded-xl bg-accent/10 border border-accent/15"
            transition={{ type: "spring", stiffness: 350, damping: 28 }}
          />
        )}
        <span className={cn("relative z-10 transition-colors", selected ? "text-accent-light" : "text-txt-tertiary group-hover:text-txt-secondary")}>
          {item.icon}
        </span>
        <span className="relative z-10">{item.label}</span>
        {selected && (
          <ChevronRight size={16} strokeWidth={1.6} className="relative z-10 ml-auto text-accent/50" />
        )}
      </button>
    );
  }

  const sidebar = (
    <div className="flex h-full flex-col">
      {/* Logo */}
      <div className="px-5 pb-8 pt-6">
        <div className="flex items-center gap-3">
          <div className="relative">
            <div className="absolute inset-0 rounded-xl bg-gradient-to-br from-accent to-accent-light opacity-25 blur-lg" />
            <div className="relative grid h-11 w-11 place-items-center rounded-xl bg-gradient-to-br from-accent to-accent-secondary shadow-lg shadow-accent/20">
              <Zap size={22} strokeWidth={2} className="text-white" />
            </div>
          </div>
          <div>
            <h1 className="text-[18px] font-bold text-txt-primary">Nexus</h1>
            <p className="text-[12px] text-txt-muted">Control Panel</p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-6 px-3 overflow-y-auto">
        <div>
          <p className="mb-2 px-3 text-section-label uppercase text-txt-muted">Overview</p>
          <div className="space-y-1">
            {sectionMain.map((item) => <NavLink key={item.href} item={item} />)}
          </div>
        </div>

        <div>
          <p className="mb-2 px-3 text-section-label uppercase text-txt-muted">System</p>
          <div className="space-y-1">
            {sectionSystem.map((item) => <NavLink key={item.href} item={item} />)}
          </div>
        </div>
      </nav>

      {/* Footer */}
      <div className="border-t border-border/50 p-4 space-y-3">
        {/* User card */}
        <div className="flex items-center gap-3 rounded-xl bg-surface-3/40 px-3 py-3">
          <div className="relative">
            <div className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-accent/20 to-accent-secondary/20 text-[14px] font-bold text-accent-light">
              A
            </div>
            <div className="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-surface-1 bg-status-success" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-[13px] font-semibold text-txt">Admin</p>
            <p className="truncate text-[12px] text-txt-muted">root@nexus</p>
          </div>
        </div>

        <button
          type="button"
          onClick={() => void logout()}
          className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-[13px] font-medium text-txt-muted transition-colors hover:bg-surface-3/40 hover:text-status-danger"
        >
          <LogOut size={18} strokeWidth={1.6} />
          Sign out
        </button>
      </div>
    </div>
  );

  return (
    <div className="min-h-screen bg-surface-0 text-txt">
      {/* Desktop sidebar */}
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-[260px] border-r border-border/50 bg-surface-1 lg:block">
        {sidebar}
      </aside>

      {/* Mobile sidebar */}
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
              className="h-full w-[280px] border-r border-border/50 bg-surface-1"
              initial={{ x: -50, opacity: 0 }}
              animate={{ x: 0, opacity: 1 }}
              exit={{ x: -50, opacity: 0 }}
              transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
              onClick={(e) => e.stopPropagation()}
            >
              {sidebar}
            </motion.aside>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Main area */}
      <div className="lg:pl-[260px]">
        <header className="sticky top-0 z-20 border-b border-border/40 bg-surface-0/85 px-6 py-4 backdrop-blur-xl">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <button
                type="button"
                className="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-border bg-surface-1 text-txt-secondary transition-colors hover:text-txt lg:hidden"
                onClick={() => setMobileOpen(true)}
                aria-label="Open menu"
              >
                <PanelLeft size={20} strokeWidth={1.6} />
              </button>
              <div>
                <h2 className="text-[20px] font-bold text-txt-primary">{activeTitle}</h2>
              </div>
            </div>

            <div className="flex items-center gap-3">
              <div className="hidden items-center rounded-xl border border-border/70 bg-surface-2/70 p-1 backdrop-blur-xl sm:inline-flex">
                {(["light", "dark"] as ThemeMode[]).map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    onClick={() => changeTheme(mode)}
                    className={cn(
                      "inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-[12px] font-semibold transition-colors",
                      theme === mode
                        ? "bg-surface-4 text-txt-primary shadow-sm"
                        : "text-txt-secondary hover:text-txt-primary",
                    )}
                  >
                    {mode === "light" ? <Sun size={14} strokeWidth={1.8} /> : <Moon size={14} strokeWidth={1.8} />}
                    {mode === "light" ? "Light" : "Dark"}
                  </button>
                ))}
              </div>
              <div className="hidden items-center gap-2 rounded-full border border-status-success/15 bg-status-success/6 px-4 py-2 text-[13px] font-medium text-status-success sm:flex">
                <span className="relative flex h-2 w-2">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-status-success opacity-50" />
                  <span className="relative inline-flex h-2 w-2 rounded-full bg-status-success" />
                </span>
                Systems online
              </div>
              <button
                type="button"
                onClick={() => changeTheme(theme === "dark" ? "light" : "dark")}
                className="grid h-10 w-10 place-items-center rounded-xl border border-border bg-surface-1 text-txt-tertiary transition-colors hover:text-txt sm:hidden"
                aria-label="Toggle theme"
              >
                {theme === "dark" ? <Sun size={18} strokeWidth={1.8} /> : <Moon size={18} strokeWidth={1.8} />}
              </button>
            </div>
          </div>
        </header>

        <main className="p-5 md:p-8">
          <AnimatePresence mode="wait">
            <motion.div
              key={pathname}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              transition={{ duration: 0.25, ease: [0.25, 0.1, 0.25, 1] }}
            >
              {children}
            </motion.div>
          </AnimatePresence>
        </main>
      </div>
    </div>
  );
}
