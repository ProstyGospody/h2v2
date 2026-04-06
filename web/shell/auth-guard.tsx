import { useQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { ReactNode, useEffect } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { APIError, apiFetch } from "@/services/api";

export function AuthGuard({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const location = useLocation();
  const sessionQuery = useQuery({
    queryKey: ["auth", "session"],
    queryFn: () => apiFetch<{ id: string }>("/api/auth/me", { method: "GET", timeoutMs: 6000 }),
    retry: false,
    refetchOnWindowFocus: false,
  });

  useEffect(() => {
    if (!sessionQuery.isError) {
      return;
    }

    const target = location.pathname + location.search + location.hash;
    const error = sessionQuery.error;
    if (error instanceof APIError && error.status === 401) {
      navigate("/login", { replace: true, state: { from: target } });
      return;
    }
    navigate("/login", { replace: true, state: { from: target, error: "Server unavailable. Please try again later." } });
  }, [location.hash, location.pathname, location.search, navigate, sessionQuery.error, sessionQuery.isError]);

  if (sessionQuery.isPending || sessionQuery.isError) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-surface-0">
        <div className="flex flex-col items-center gap-2">
          <Loader2 size={20} strokeWidth={1.4} className="animate-spin text-txt-secondary" />
          <p className="text-[14px] text-txt-secondary">Checking session...</p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
