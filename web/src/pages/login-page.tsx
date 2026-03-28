import { Bolt, Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { useLocation, useNavigate } from "react-router-dom";

import { Button, Input } from "@/src/components/ui";
import { APIError, apiFetch } from "@/services/api";

type LoginFormValues = {
  email: string;
  password: string;
};

export default function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [error, setError] = useState("");
  const [checking, setChecking] = useState(true);
  const {
    register,
    handleSubmit,
    formState: { isSubmitting },
  } = useForm<LoginFormValues>({
    defaultValues: {
      email: "",
      password: "",
    },
  });
  const redirectTo = (location.state as { from?: string } | null)?.from || "/";

  useEffect(() => {
    let disposed = false;
    async function bootstrap() {
      try {
        await apiFetch<{ id: string }>("/api/auth/me", { method: "GET" });
        if (!disposed) {
          navigate(redirectTo, { replace: true });
        }
      } catch {
        if (!disposed) {
          setChecking(false);
        }
      }
    }
    void bootstrap();
    return () => {
      disposed = true;
    };
  }, [navigate, redirectTo]);

  const submit = handleSubmit(async ({ email, password }) => {
    setError("");
    try {
      await apiFetch<{ csrf_token: string }>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      navigate(redirectTo, { replace: true });
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Login failed");
    }
  });

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-surface-0">
        <div className="flex flex-col items-center gap-2">
          <Loader2 size={20} strokeWidth={1.4} className="animate-spin text-accent-light" />
          <p className="text-[12px] text-txt-secondary">Loading...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="grid min-h-screen place-items-center bg-surface-0 px-4">
      <div className="w-full max-w-[430px] rounded-card border border-border bg-surface-2 p-6">
        <div className="space-y-5">
          <div className="flex items-center gap-3">
            <div className="grid h-[42px] w-[42px] place-items-center rounded-btn bg-gradient-to-br from-accent to-accent-secondary text-white">
              <Bolt size={16} strokeWidth={1.4} />
            </div>
            <div>
              <h1 className="text-[20px] font-extrabold text-white">Hysteria 2 Panel</h1>
              <p className="text-[12px] text-txt-secondary">Admin login</p>
            </div>
          </div>

          {error ? <div className="rounded-btn border border-status-danger/20 bg-status-danger/10 px-3 py-2 text-[12px] text-status-danger">{error}</div> : null}

          <form onSubmit={submit} className="space-y-3">
            <Input label="Admin email" type="email" required autoComplete="username" {...register("email", { required: true })} />
            <Input label="Password" type="password" required autoComplete="current-password" {...register("password", { required: true })} />
            <Button type="submit" variant="primary" className="w-full justify-center py-2.5" disabled={isSubmitting}>
              {isSubmitting ? "Signing in..." : "Sign in"}
            </Button>
          </form>
        </div>
      </div>
    </div>
  );
}
