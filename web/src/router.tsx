import { Outlet, Navigate, createBrowserRouter } from "react-router-dom";

import { AuthGuard } from "@/shell/auth-guard";
import { PanelShell } from "@/shell/panel-shell";
import { ErrorBoundary } from "@/src/components/ErrorBoundary";
import { ConfirmDialogProvider } from "@/src/components/ui/ConfirmDialog";
import { ToastProvider } from "@/src/components/ui/Toast";
import { TooltipProvider } from "@/src/components/ui/Tooltip";
import { AuditFeedProvider } from "@/src/state/audit-feed";

import AuditPage from "./pages/audit-page";
import ConfigPage from "./pages/config-page";
import DashboardPage from "./pages/dashboard-page";
import LoginPage from "./pages/login-page";
import UsersPage from "./pages/users-page";

function PanelLayout() {
  return (
    <AuthGuard>
      <TooltipProvider>
        <ConfirmDialogProvider>
          <ToastProvider>
            <AuditFeedProvider>
              <PanelShell>
                <ErrorBoundary>
                  <Outlet />
                </ErrorBoundary>
              </PanelShell>
            </AuditFeedProvider>
          </ToastProvider>
        </ConfirmDialogProvider>
      </TooltipProvider>
    </AuthGuard>
  );
}

export const router = createBrowserRouter([
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/",
    element: <PanelLayout />,
    children: [
      {
        index: true,
        element: <DashboardPage />,
      },
      {
        path: "users",
        element: <UsersPage />,
      },
      {
        path: "config",
        element: <ConfigPage />,
      },
      {
        path: "audit",
        element: <AuditPage />,
      },
    ],
  },
  {
    path: "*",
    element: <Navigate to="/" replace />,
  },
]);
