import { lazy, Suspense } from "react";
import { Outlet, Navigate, createBrowserRouter } from "react-router-dom";

import { AuthGuard } from "@/shell/auth-guard";
import { PanelShell } from "@/shell/panel-shell";
import { ErrorBoundary } from "@/src/components/ErrorBoundary";
import { ConfirmDialogProvider } from "@/src/components/ui/ConfirmDialog";
import { ToastProvider } from "@/src/components/ui/Toast";
import { TooltipProvider } from "@/src/components/ui/Tooltip";

const DashboardPage = lazy(() => import("./pages/dashboard-page"));
const LoginPage = lazy(() => import("./pages/login-page"));
const UsersPage = lazy(() => import("./pages/users-page"));

function PanelLayout() {
  return (
    <AuthGuard>
      <TooltipProvider>
        <ConfirmDialogProvider>
          <ToastProvider>
            <PanelShell>
              <ErrorBoundary>
                <Suspense>
                  <Outlet />
                </Suspense>
              </ErrorBoundary>
            </PanelShell>
          </ToastProvider>
        </ConfirmDialogProvider>
      </TooltipProvider>
    </AuthGuard>
  );
}

export const router = createBrowserRouter([
  {
    path: "/login",
    element: <Suspense><LoginPage /></Suspense>,
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
    ],
  },
  {
    path: "*",
    element: <Navigate to="/" replace />,
  },
]);
