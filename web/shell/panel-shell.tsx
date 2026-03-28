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
    <div className="rounded-card border border-border bg-surface-2 p-3">
      <div className="flex items-center gap-2">
        <div className="grid h-9 w-9 place-items-center rounded-btn bg-gradient-to-br from-accent to-accent-secondary text-[13px] font-semibold text-white">
          A
        </div>
        <div>
          <p className="text-[12px] font-semibold text-txt">Admin</p>
          <p className="text-[11px] text-txt-muted">root@nexus</p>
        </div>
      </div>
    </div>
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
      <div className="px-4 pb-5 pt-4">
        <div className="flex items-center gap-2">
          <div className="grid h-[34px] w-[34px] place-items-center rounded-btn bg-gradient-to-br from-accent to-accent-secondary text-white">
            <ReceiptText size={16} strokeWidth={1.4} />
          </div>
          <p className="text-[16px] font-semibold text-white">Nexus</p>
        </div>
      </div>

      <div className="space-y-5 px-3">
        <div>
          <p className="px-2 text-section-label uppercase text-txt-muted">MAIN</p>
          <div className="mt-2 space-y-1">
            {sectionMain.map((item) => {
              const selected = isNavItemSelected(pathname, item.href);
              return (
                <button
                  key={item.href}
                  type="button"
                  onClick={() => {
                    navigate(item.href);
                    setMobileOpen(false);
                  }}
                  className={cn(
                    "flex w-full items-center gap-2 rounded-btn px-3 py-2 text-[12px] transition-colors",
                    selected &&
                      "bg-[linear-gradient(135deg,rgba(99,102,241,0.10),rgba(139,92,246,0.05))] font-medium text-accent-secondary-light",
                    !selected && "text-txt-tertiary hover:bg-surface-3 hover:text-txt-secondary",
                  )}
                >
                  {item.icon}
                  <span>{item.label}</span>
                </button>
              );
            })}
          </div>
        </div>

        <Separator />

        <div>
          <p className="px-2 text-section-label uppercase text-txt-muted">SYSTEM</p>
          <div className="mt-2 space-y-1">
            {sectionSystem.map((item) => {
              const selected = isNavItemSelected(pathname, item.href);
              return (
                <button
                  key={item.href}
                  type="button"
                  onClick={() => {
                    navigate(item.href);
                    setMobileOpen(false);
                  }}
                  className={cn(
                    "flex w-full items-center gap-2 rounded-btn px-3 py-2 text-[12px] transition-colors",
                    selected &&
                      "bg-[linear-gradient(135deg,rgba(99,102,241,0.10),rgba(139,92,246,0.05))] font-medium text-accent-secondary-light",
                    !selected && "text-txt-tertiary hover:bg-surface-3 hover:text-txt-secondary",
                  )}
                >
                  {item.icon}
                  <span>{item.label}</span>
                </button>
              );
            })}
          </div>
        </div>
      </div>

      <div className="mt-auto space-y-3 p-3">
        <UserCard />
        <Button variant="ghost" className="w-full justify-start" onClick={() => void logout()}>
          <LogOut size={16} strokeWidth={1.4} />
          Sign out
        </Button>
      </div>
    </div>
  );

  return (
    <div className="min-h-screen bg-surface-0 text-txt">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-56 border-r border-border bg-surface-1 lg:block">{sidebarContent}</aside>

      <AnimatePresence>
        {mobileOpen ? (
          <motion.div className="fixed inset-0 z-40 bg-black/50 lg:hidden" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
            <motion.aside
              className="h-full w-56 border-r border-border bg-surface-1"
              initial={{ x: -40, opacity: 0 }}
              animate={{ x: 0, opacity: 1 }}
              exit={{ x: -40, opacity: 0 }}
              transition={{ duration: 0.2 }}
            >
              {sidebarContent}
            </motion.aside>
            <button type="button" className="absolute right-3 top-3 rounded-btn border border-border bg-surface-2 p-2 text-txt-secondary" onClick={() => setMobileOpen(false)}>
              <X size={16} strokeWidth={1.4} />
            </button>
          </motion.div>
        ) : null}
      </AnimatePresence>

      <div className="lg:pl-56">
        <header className="sticky top-0 z-20 border-b border-border bg-surface-0/95 px-6 py-[14px] backdrop-blur">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <button
                type="button"
                className="inline-flex h-8 w-8 items-center justify-center rounded-btn border border-border bg-surface-1 text-txt-secondary lg:hidden"
                onClick={() => setMobileOpen(true)}
                aria-label="Open sidebar"
              >
                <Menu size={16} strokeWidth={1.4} />
              </button>
              <h2 className="text-[18px] font-semibold text-white">{activeTitle}</h2>
            </div>
            <div className="inline-flex items-center gap-2 rounded-pill border border-status-success/20 bg-status-success/8 px-3 py-1.5 text-[11px] font-medium text-status-success">
              <span className="h-[6px] w-[6px] rounded-full bg-status-success shadow-[0_0_8px_#34d39960]" />
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
