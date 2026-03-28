import { AnimatePresence, motion } from "framer-motion";
import {
  LayoutDashboard,
  LogOut,
  Menu,
  ReceiptText,
  Server,
  Settings,
  Users,
  X,
  Zap,
} from "lucide-react";
import { type ReactNode, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { Button, Separator, cn } from "@/src/components/ui";
import { apiFetch } from "@/services/api";

type NavItem = {
  href: string;
  label: string;
  icon: ReactNode;
  section: "MAIN" | "SYSTEM";
};

const navItems: NavItem[] = [
  { href: "/", label: "Dashboard", icon: <LayoutDashboard size={16} strokeWidth={1.4} />, section: "MAIN" },
  { href: "/users", label: "Users", icon: <Users size={16} strokeWidth={1.4} />, section: "MAIN" },
  { href: "/config", label: "Settings", icon: <Settings size={16} strokeWidth={1.4} />, section: "SYSTEM" },
  { href: "/audit", label: "Audit", icon: <Server size={16} strokeWidth={1.4} />, section: "SYSTEM" },
];

function isNavItemSelected(pathname: string, href: string): boolean {
  if (href === "/") {
    return pathname === "/";
  }
  return pathname === href || pathname.startsWith(`${href}/`);
}

function resolveTitle(pathname: string): string {
  if (pathname === "/") {
    return "Dashboard";
  }
  return navItems.find((item) => item.href === pathname)?.label || "Panel";
}

function UserCard() {
  return (
    <div className="rounded-[10px] border border-border bg-surface-0/50 p-3">
      <div className="flex items-center gap-2.5">
        <div className="relative">
          <div className="grid h-9 w-9 place-items-center rounded-[8px] bg-gradient-to-br from-accent to-accent-secondary text-[13px] font-semibold text-white">
            A
          </div>
          <div className="absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border-2 border-surface-1 bg-status-success" />
        </div>
        <div>
          <p className="text-[12px] font-semibold text-txt">Admin</p>
          <p className="text-[11px] text-txt-muted">root@nexus</p>
        </div>
      </div>
    </div>
  );
}

function NavButton({
  item,
  selected,
  onClick,
}: {
  item: NavItem;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "group relative flex w-full items-center gap-2.5 rounded-[8px] px-3 py-2 text-[12px] transition-all duration-200",
        selected && "font-medium text-white",
        !selected && "text-txt-tertiary hover:bg-surface-3/50 hover:text-txt-secondary",
      )}
    >
      {selected ? (
        <motion.div
          layoutId="nav-active"
          className="absolute inset-0 rounded-[8px] bg-gradient-to-r from-accent/15 to-accent-secondary/8 border border-accent/10"
          transition={{ type: "spring", stiffness: 380, damping: 30 }}
        />
      ) : null}
      <span className={cn("relative z-10 transition-colors", selected && "text-accent-light")}>{item.icon}</span>
      <span className="relative z-10">{item.label}</span>
    </button>
  );
}

export function PanelShell({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);

  const activeTitle = useMemo(() => resolveTitle(pathname), [pathname]);

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

  const sidebarContent = (
    <div className="flex h-full flex-col">
      {/* Brand */}
      <div className="px-4 pb-6 pt-5">
        <div className="flex items-center gap-2.5">
          <div className="relative">
            <div className="absolute inset-0 rounded-[10px] bg-gradient-to-br from-accent to-accent-secondary opacity-30 blur-lg" />
            <div className="relative grid h-[36px] w-[36px] place-items-center rounded-[10px] bg-gradient-to-br from-accent to-accent-secondary text-white shadow-lg shadow-accent/20">
              <Zap size={17} strokeWidth={1.8} />
            </div>
          </div>
          <div>
            <p className="text-[15px] font-bold text-white">Nexus</p>
            <p className="text-[10px] font-medium text-txt-muted">Control Panel</p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <div className="space-y-5 px-3">
        <div>
          <p className="mb-2 px-2 text-section-label uppercase text-txt-muted/70">Main</p>
          <div className="space-y-0.5">
            {sectionMain.map((item) => (
              <NavButton
                key={item.href}
                item={item}
                selected={isNavItemSelected(pathname, item.href)}
                onClick={() => {
                  navigate(item.href);
                  setMobileOpen(false);
                }}
              />
            ))}
          </div>
        </div>

        <Separator className="opacity-50" />

        <div>
          <p className="mb-2 px-2 text-section-label uppercase text-txt-muted/70">System</p>
          <div className="space-y-0.5">
            {sectionSystem.map((item) => (
              <NavButton
                key={item.href}
                item={item}
                selected={isNavItemSelected(pathname, item.href)}
                onClick={() => {
                  navigate(item.href);
                  setMobileOpen(false);
                }}
              />
            ))}
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="mt-auto space-y-3 p-3">
        <UserCard />
        <Button
          variant="ghost"
          className="w-full justify-start border-transparent text-txt-muted hover:border-border hover:text-txt-secondary"
          onClick={() => void logout()}
        >
          <LogOut size={15} strokeWidth={1.4} />
          Sign out
        </Button>
      </div>
    </div>
  );

  return (
    <div className="min-h-screen bg-surface-0 text-txt">
      {/* Desktop sidebar */}
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-[220px] border-r border-border/60 bg-surface-1/95 backdrop-blur-sm lg:block">
        {sidebarContent}
      </aside>

      {/* Mobile sidebar */}
      <AnimatePresence>
        {mobileOpen ? (
          <motion.div
            className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm lg:hidden"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            <motion.aside
              className="h-full w-[220px] border-r border-border/60 bg-surface-1"
              initial={{ x: -40, opacity: 0 }}
              animate={{ x: 0, opacity: 1 }}
              exit={{ x: -40, opacity: 0 }}
              transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            >
              {sidebarContent}
            </motion.aside>
            <button
              type="button"
              className="absolute right-3 top-3 rounded-[8px] border border-border bg-surface-2 p-2 text-txt-secondary transition-colors hover:text-txt"
              onClick={() => setMobileOpen(false)}
            >
              <X size={16} strokeWidth={1.4} />
            </button>
          </motion.div>
        ) : null}
      </AnimatePresence>

      {/* Main content */}
      <div className="lg:pl-[220px]">
        <header className="sticky top-0 z-20 border-b border-border/60 bg-surface-0/80 px-6 py-3.5 backdrop-blur-xl">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-3">
              <button
                type="button"
                className="inline-flex h-8 w-8 items-center justify-center rounded-[8px] border border-border bg-surface-1 text-txt-secondary transition-colors hover:text-txt lg:hidden"
                onClick={() => setMobileOpen(true)}
                aria-label="Open sidebar"
              >
                <Menu size={16} strokeWidth={1.4} />
              </button>
              <h2 className="text-[17px] font-semibold text-white">{activeTitle}</h2>
            </div>
            <div className="inline-flex items-center gap-2 rounded-full border border-status-success/15 bg-status-success/6 px-3 py-1.5 text-[11px] font-medium text-status-success">
              <span className="relative flex h-[6px] w-[6px]">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-status-success opacity-50" />
                <span className="relative inline-flex h-[6px] w-[6px] rounded-full bg-status-success" />
              </span>
              All systems online
            </div>
          </div>
        </header>

        <main className="p-4 md:p-6">
          <AnimatePresence mode="wait">
            <motion.div
              key={pathname}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.2, ease: [0.25, 0.1, 0.25, 1] }}
            >
              {children}
            </motion.div>
          </AnimatePresence>
        </main>
      </div>
    </div>
  );
}
