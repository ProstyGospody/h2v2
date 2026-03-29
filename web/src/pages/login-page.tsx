import { motion } from "framer-motion";
import { Loader2, Lock, Mail, Moon, Sun, Zap } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { useLocation, useNavigate } from "react-router-dom";

import { Button } from "@/src/components/ui";
import { applyTheme, resolveTheme, type ThemeMode } from "@/src/theme";
import { APIError, apiFetch } from "@/services/api";

type LoginFormValues = { email: string; password: string };

export default function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [error, setError] = useState("");
  const [checking, setChecking] = useState(true);
  const [theme, setTheme] = useState<ThemeMode>(() => resolveTheme());
  const { register, handleSubmit, formState: { isSubmitting } } = useForm<LoginFormValues>({ defaultValues: { email: "", password: "" } });
  const redirectTo = (location.state as { from?: string } | null)?.from || "/";

  function toggleTheme() {
    const next: ThemeMode = theme === "dark" ? "light" : "dark";
    setTheme(next);
    applyTheme(next);
  }

  useEffect(() => {
    let disposed = false;
    (async () => {
      try { await apiFetch<{ id: string }>("/api/auth/me", { method: "GET" }); if (!disposed) navigate(redirectTo, { replace: true }); }
      catch { if (!disposed) setChecking(false); }
    })();
    return () => { disposed = true; };
  }, [navigate, redirectTo]);

  const submit = handleSubmit(async ({ email, password }) => {
    setError("");
    try { await apiFetch<{ csrf_token: string }>("/api/auth/login", { method: "POST", body: JSON.stringify({ email, password }) }); navigate(redirectTo, { replace: true }); }
    catch (err) { setError(err instanceof APIError ? err.message : "Login failed"); }
  });

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-surface-0">
        <motion.div className="flex flex-col items-center gap-3" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
          <p className="text-[14px] text-txt-secondary">Loading...</p>
        </motion.div>
      </div>
    );
  }

  return (
    <div className="relative grid min-h-screen place-items-center overflow-hidden bg-surface-0 px-5">
      <button
        type="button"
        onClick={toggleTheme}
        className="absolute right-5 top-5 inline-flex items-center gap-2 rounded-xl border border-border/80 bg-surface-2/70 px-3 py-2 text-[12px] font-semibold text-txt-secondary backdrop-blur-xl transition-colors hover:text-txt-primary"
        aria-label="Toggle theme"
      >
        {theme === "dark" ? <Sun size={14} strokeWidth={1.8} /> : <Moon size={14} strokeWidth={1.8} />}
        {theme === "dark" ? "Light" : "Dark"}
      </button>
      <motion.div initial={{ opacity: 0, y: 24, scale: 0.97 }} animate={{ opacity: 1, y: 0, scale: 1 }} transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }} className="relative w-full max-w-[440px]">
        <div className="glass-strong gradient-border rounded-2xl p-9 shadow-2xl shadow-accent/5">
          <div className="space-y-7">
            {/* Brand */}
            <div className="flex flex-col items-center gap-5 text-center">
              <motion.div initial={{ scale: 0, rotate: -180 }} animate={{ scale: 1, rotate: 0 }} transition={{ type: "spring", stiffness: 200, damping: 15, delay: 0.2 }} className="relative">
                <div className="absolute inset-0 rounded-2xl bg-gradient-to-br from-accent to-accent-secondary opacity-35 blur-xl" />
                <div className="relative grid h-14 w-14 place-items-center rounded-2xl bg-gradient-to-br from-accent to-accent-secondary text-white shadow-lg shadow-accent/25">
                  <Zap size={26} strokeWidth={1.8} />
                </div>
              </motion.div>
              <div>
                <h1 className="text-[24px] font-bold text-txt-primary">Welcome back</h1>
                <p className="mt-1.5 text-[15px] text-txt-secondary">Sign in to Nexus Panel</p>
              </div>
            </div>

            {error && (
              <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: "auto" }} className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">
                {error}
              </motion.div>
            )}

            <form onSubmit={submit} className="space-y-5">
              <div className="space-y-3.5">
                <div className="relative">
                  <Mail size={18} strokeWidth={1.6} className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-txt-muted" />
                  <input type="email" required autoComplete="username" placeholder="Admin email" {...register("email", { required: true })}
                    className="w-full rounded-xl border border-border bg-surface-0/50 py-3.5 pl-12 pr-4 text-[15px] text-txt outline-none transition-all placeholder:text-txt-muted focus:border-accent-secondary/40 focus:bg-surface-0/80 focus:shadow-[0_0_0_3px_var(--accent-soft)]" />
                </div>
                <div className="relative">
                  <Lock size={18} strokeWidth={1.6} className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-txt-muted" />
                  <input type="password" required autoComplete="current-password" placeholder="Password" {...register("password", { required: true })}
                    className="w-full rounded-xl border border-border bg-surface-0/50 py-3.5 pl-12 pr-4 text-[15px] text-txt outline-none transition-all placeholder:text-txt-muted focus:border-accent-secondary/40 focus:bg-surface-0/80 focus:shadow-[0_0_0_3px_var(--accent-soft)]" />
                </div>
              </div>
              <Button type="submit" variant="primary" className="w-full justify-center rounded-xl py-3.5 text-[15px]" disabled={isSubmitting}>
                {isSubmitting ? <><Loader2 size={18} strokeWidth={1.8} className="animate-spin" />Signing in...</> : "Sign in"}
              </Button>
            </form>

            <p className="text-center text-[13px] text-txt-muted">Secured connection &middot; Nexus Panel</p>
          </div>
        </div>
      </motion.div>
    </div>
  );
}
