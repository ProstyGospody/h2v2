import {
  Activity,
  ChevronLeft,
  ChevronRight,
  LogOut,
  Moon,
  PanelLeft,
  Settings,
  Sun,
  Users2,
  Zap,
} from "lucide-react";
import { memo, type ReactNode, type TouchEvent, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { Tooltip, cn } from "@/src/components/ui";
import { useConfirmDialog } from "@/src/components/ui/ConfirmDialog";
import { hasUnsavedChangesGuard } from "@/src/state/navigation-guard";
import { applyTheme, resolveTheme, type ThemeMode } from "@/src/theme";
import { apiFetch } from "@/services/api";

type NavItem = {
  href: string;
  label: string;
  icon: ReactNode;
};

type SidebarNavLinkProps = {
  item: NavItem;
  compact: boolean;
  selected: boolean;
  onNavigate: (href: string) => void;
};

const SidebarNavLink = memo(function SidebarNavLink({ item, compact, selected, onNavigate }: SidebarNavLinkProps) {
  const content = (
    <button
      type="button"
      aria-label={item.label}
      onClick={() => onNavigate(item.href)}
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

  if (compact) {
    return <Tooltip content={item.label} side="right">{content}</Tooltip>;
  }
  return content;
});

type SidebarContentProps = {
  compact: boolean;
  mobile: boolean;
  pathname: string;
  collapsed: boolean;
  theme: ThemeMode;
  onNavigate: (href: string) => void;
  onToggleCollapsed: () => void;
  onToggleTheme: () => void;
  onLogout: () => void;
};

const SidebarContent = memo(function SidebarContent({
  compact,
  mobile,
  pathname,
  collapsed,
  theme,
  onNavigate,
  onToggleCollapsed,
  onToggleTheme,
  onLogout,
}: SidebarContentProps) {
  return (
    <div className={cn("flex h-full flex-col", compact && "items-center")}>
      <div className={cn("relative flex w-full items-center pb-4 pt-5", compact ? "justify-center px-2" : "justify-start px-5")}>
        <div className={cn("relative flex min-w-0 items-center", compact && "mx-auto")}>
          <div className="relative">
            <div className={cn("relative grid place-items-center rounded-xl bg-gradient-to-br from-accent to-accent-secondary shadow-[inset_0_1px_0_rgba(255,255,255,0.22)]", compact ? "h-11 w-11" : "h-12 w-12")}>
              <Zap size={compact ? 22 : 24} strokeWidth={2} className="text-white" />
            </div>
          </div>

          {!compact && (
            <div className="min-w-0 flex-1 pl-3">
              <p className="truncate text-[17px] font-bold text-txt-primary">H2V2</p>
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
            {navItems.map((item) => (
              <SidebarNavLink
                key={item.href}
                item={item}
                compact={compact}
                selected={isActive(pathname, item.href)}
                onNavigate={onNavigate}
              />
            ))}
          </div>
        </div>
      </nav>

      <div className="w-full space-y-2 p-3">
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
              <p className="truncate text-[12px] text-txt-muted">root@h2v2</p>
            </div>
          </div>
        )}

        {!mobile && (
          <Tooltip content={compact ? (collapsed ? "Expand sidebar" : "Collapse sidebar") : undefined} side="right">
            <button
              type="button"
              onClick={onToggleCollapsed}
              className={cn(
                "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted transition-all duration-200 hover:bg-surface-3/40 hover:text-txt-primary",
                compact ? "justify-center px-2 py-2.5" : "gap-3 px-3 py-2.5",
              )}
              aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            >
              {collapsed ? <ChevronRight size={compact ? 22 : 18} strokeWidth={1.8} /> : <ChevronLeft size={compact ? 22 : 18} strokeWidth={1.8} />}
              {!compact && (collapsed ? "Expand sidebar" : "Collapse sidebar")}
            </button>
          </Tooltip>
        )}

        <Tooltip content={compact ? "Toggle theme" : undefined} side="right">
          <button
            type="button"
            aria-label="Toggle theme"
            onClick={onToggleTheme}
            className={cn(
              "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted transition-all duration-200 hover:bg-surface-3/40 hover:text-txt-primary",
              compact ? "justify-center px-2 py-2.5" : "gap-3 px-3 py-2.5",
            )}
          >
            {theme === "dark" ? <Sun size={compact ? 22 : 18} strokeWidth={1.8} /> : <Moon size={compact ? 22 : 18} strokeWidth={1.8} />}
            {!compact && (theme === "dark" ? "Light theme" : "Dark theme")}
          </button>
        </Tooltip>

        <Tooltip content={compact ? "Sign out" : undefined} side="right">
          <button
            type="button"
            aria-label="Sign out"
            onClick={onLogout}
            className={cn(
              "flex w-full items-center rounded-xl text-[13px] font-medium text-txt-muted transition-all duration-200 hover:bg-status-danger/8 hover:text-status-danger",
              compact ? "justify-center px-2 py-2.5" : "gap-3 px-3 py-2.5",
            )}
          >
            <LogOut size={compact ? 22 : 18} strokeWidth={1.8} />
            {!compact && "Sign out"}
          </button>
        </Tooltip>
      </div>
    </div>
  );
});

const SIDEBAR_COLLAPSED_KEY = "panel-sidebar-collapsed";

const navItems: NavItem[] = [
  { href: "/", label: "Dashboard", icon: <Activity size={24} strokeWidth={1.8} /> },
  { href: "/users", label: "Users", icon: <Users2 size={24} strokeWidth={1.8} /> },
  { href: "/settings", label: "Settings", icon: <Settings size={24} strokeWidth={1.8} /> },
];

function isActive(pathname: string, href: string): boolean {
  if (href === "/") return pathname === "/";
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function PanelShell({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { confirm } = useConfirmDialog();

  const [mobileOpen, setMobileOpen] = useState(false);
  const [theme, setTheme] = useState<ThemeMode>(() => resolveTheme());
  const swipeStartXRef = useRef<number | null>(null);
  const swipeStartYRef = useRef<number | null>(null);
  const mobileSidebarRef = useRef<HTMLElement | null>(null);
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

  useEffect(() => {
    if (!("scrollRestoration" in window.history)) {
      return;
    }
    const previous = window.history.scrollRestoration;
    window.history.scrollRestoration = "manual";
    return () => {
      window.history.scrollRestoration = previous;
    };
  }, []);

  useLayoutEffect(() => {
    window.scrollTo({ top: 0, left: 0, behavior: "auto" });
  }, [pathname]);

  useEffect(() => {
    if (!mobileOpen) return;
    mobileSidebarRef.current?.focus();
  }, [mobileOpen]);

  const confirmDiscardChanges = useCallback(async () => {
    if (!hasUnsavedChangesGuard()) {
      return true;
    }
    return confirm({
      title: "Discard changes?",
      description: "Unsaved changes will be lost.",
      confirmText: "Discard",
      cancelText: "Stay",
    });
  }, [confirm]);

  const logout = useCallback(async () => {
    const shouldProceed = await confirmDiscardChanges();
    if (!shouldProceed) {
      return;
    }
    try {
      await apiFetch<{ ok: boolean }>("/api/auth/logout", { method: "POST", body: JSON.stringify({}) });
    } catch {
      // no-op
    }
    navigate("/login", { replace: true });
  }, [confirmDiscardChanges, navigate]);

  const handleNavigate = useCallback(
    async (href: string) => {
      if (href !== pathname) {
        const shouldProceed = await confirmDiscardChanges();
        if (!shouldProceed) {
          return;
        }
      }
      navigate(href);
      setMobileOpen(false);
    },
    [confirmDiscardChanges, navigate, pathname],
  );

  const onNavigate = useCallback(
    (href: string) => {
      void handleNavigate(href);
    },
    [handleNavigate],
  );

  const onToggleCollapsed = useCallback(() => {
    setCollapsed((prev) => !prev);
  }, []);

  const onToggleTheme = useCallback(() => {
    setTheme((prev) => (prev === "dark" ? "light" : "dark"));
  }, []);

  const onLogout = useCallback(() => {
    void logout();
  }, [logout]);

  const sidebarProps = useMemo(
    () => ({
      pathname,
      collapsed,
      theme,
      onNavigate,
      onToggleCollapsed,
      onToggleTheme,
      onLogout,
    }),
    [pathname, collapsed, theme, onNavigate, onToggleCollapsed, onToggleTheme, onLogout],
  );

  function resetSwipeTrack() {
    swipeStartXRef.current = null;
    swipeStartYRef.current = null;
  }
  function onMobileSidebarTouchStart(event: TouchEvent<HTMLElement>) {
    const touch = event.touches[0];
    swipeStartXRef.current = touch.clientX;
    swipeStartYRef.current = touch.clientY;
  }
  function onMobileSidebarTouchEnd(event: TouchEvent<HTMLElement>) {
    if (swipeStartXRef.current === null || swipeStartYRef.current === null) {
      return;
    }
    const touch = event.changedTouches[0];
    const deltaX = touch.clientX - swipeStartXRef.current;
    const deltaY = touch.clientY - swipeStartYRef.current;
    resetSwipeTrack();
    if (deltaX < -70 && Math.abs(deltaY) < 50) {
      setMobileOpen(false);
    }
  }
  function onMobileSidebarTouchCancel() {
    resetSwipeTrack();
  }

  return (
    <div className={cn("panel-layout min-h-screen bg-surface-0 text-txt", collapsed ? "sidebar-collapsed" : "sidebar-expanded")}>
      {/* Desktop sidebar */}
      <aside
        className="panel-desktop-sidebar fixed inset-y-0 left-0 z-30 hidden overflow-x-hidden border-r border-border/30 sidebar-glass lg:block"
      >
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-accent/30 to-transparent" />
        <SidebarContent {...sidebarProps} compact={collapsed} mobile={false} />
      </aside>

      {/* Main content */}
      <div className="panel-main min-w-0">
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

        <main className="min-w-0 p-4 pt-16 sm:p-5 sm:pt-20 md:p-8 md:pt-20 lg:pt-8">
          <div key={pathname} className="panel-route-transition">
            {children}
          </div>
        </main>
      </div>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden" onKeyDown={(event) => {
          if (event.key === "Escape") {
            event.preventDefault();
            setMobileOpen(false);
          }
        }}>
          <div className="absolute inset-0 bg-[var(--dialog-overlay)] backdrop-blur-sm" onClick={() => setMobileOpen(false)} aria-hidden="true" />
          <aside
            ref={mobileSidebarRef}
            role="dialog"
            aria-modal="true"
            aria-label="Navigation menu"
            tabIndex={-1}
            className="absolute inset-y-0 left-0 w-[min(280px,100vw)] border-r border-border/30 sidebar-glass shadow-[0_22px_52px_-28px_var(--dialog-shadow)] panel-mobile-sidebar"
            onTouchStart={onMobileSidebarTouchStart}
            onTouchEnd={onMobileSidebarTouchEnd}
            onTouchCancel={onMobileSidebarTouchCancel}
            onKeyDown={(event) => {
              if (event.key !== "Tab") {
                return;
              }
              const root = mobileSidebarRef.current;
              if (!root) {
                return;
              }
              const focusable = Array.from(
                root.querySelectorAll<HTMLElement>(
                  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
                ),
              );
              if (focusable.length === 0) {
                event.preventDefault();
                root.focus();
                return;
              }
              const first = focusable[0];
              const last = focusable[focusable.length - 1];
              const active = document.activeElement as HTMLElement | null;
              if (event.shiftKey && (active === first || active === root)) {
                event.preventDefault();
                last.focus();
                return;
              }
              if (!event.shiftKey && active === last) {
                event.preventDefault();
                first.focus();
              }
            }}
          >
            <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-accent/30 to-transparent" />
            <SidebarContent {...sidebarProps} compact={false} mobile />
          </aside>
        </div>
      )}
    </div>
  );
}
