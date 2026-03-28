import { motion } from "framer-motion";
import { Bolt, Loader2, Lock, Mail } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { useLocation, useNavigate } from "react-router-dom";

import { Button, Input } from "@/src/components/ui";
import { APIError, apiFetch } from "@/services/api";

type LoginFormValues = {
  email: string;
  password: string;
};

function FloatingOrb({ className, delay = 0 }: { className?: string; delay?: number }) {
  return (
    <motion.div
      className={className}
      initial={{ opacity: 0, scale: 0.8 }}
      animate={{
        opacity: [0.15, 0.3, 0.15],
        scale: [0.8, 1.1, 0.8],
        y: [0, -30, 0],
        x: [0, 15, 0],
      }}
      transition={{
        duration: 8,
        repeat: Infinity,
        delay,
        ease: "easeInOut",
      }}
    />
  );
}

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
        <motion.div
          className="flex flex-col items-center gap-3"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
        >
          <div className="relative">
            <div className="absolute inset-0 animate-ping rounded-full bg-accent/20" />
            <Loader2 size={24} strokeWidth={1.4} className="animate-spin text-accent-light" />
          </div>
          <p className="text-[12px] text-txt-secondary">Loading...</p>
        </motion.div>
      </div>
    );
  }

  return (
    <div className="relative grid min-h-screen place-items-center overflow-hidden bg-surface-0 px-4">
      {/* Animated background orbs */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <FloatingOrb
          className="absolute -left-32 -top-32 h-[500px] w-[500px] rounded-full bg-accent/20 blur-[120px]"
          delay={0}
        />
        <FloatingOrb
          className="absolute -bottom-48 -right-32 h-[600px] w-[600px] rounded-full bg-accent-secondary/15 blur-[140px]"
          delay={2}
        />
        <FloatingOrb
          className="absolute left-1/2 top-1/3 h-[300px] w-[300px] -translate-x-1/2 rounded-full bg-accent/10 blur-[100px]"
          delay={4}
        />

        {/* Grid pattern */}
        <div
          className="absolute inset-0 opacity-[0.03]"
          style={{
            backgroundImage: `linear-gradient(rgba(99,102,241,0.3) 1px, transparent 1px), linear-gradient(90deg, rgba(99,102,241,0.3) 1px, transparent 1px)`,
            backgroundSize: "60px 60px",
          }}
        />
      </div>

      <motion.div
        initial={{ opacity: 0, y: 20, scale: 0.97 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
        className="relative w-full max-w-[420px]"
      >
        {/* Card with glass effect */}
        <div className="glass-strong gradient-border noise-bg rounded-[16px] p-8 shadow-2xl shadow-accent/5">
          <div className="space-y-6">
            {/* Brand header */}
            <div className="flex flex-col items-center gap-4 text-center">
              <motion.div
                initial={{ scale: 0, rotate: -180 }}
                animate={{ scale: 1, rotate: 0 }}
                transition={{ type: "spring", stiffness: 200, damping: 15, delay: 0.2 }}
                className="relative"
              >
                <div className="absolute inset-0 rounded-[14px] bg-gradient-to-br from-accent to-accent-secondary opacity-40 blur-xl" />
                <div className="relative grid h-[52px] w-[52px] place-items-center rounded-[14px] bg-gradient-to-br from-accent to-accent-secondary text-white shadow-lg shadow-accent/25">
                  <Bolt size={22} strokeWidth={1.6} />
                </div>
              </motion.div>
              <div>
                <h1 className="text-[22px] font-bold text-white">Welcome back</h1>
                <p className="mt-1 text-[13px] text-txt-secondary">Sign in to Hysteria 2 Panel</p>
              </div>
            </div>

            {/* Error */}
            {error ? (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: "auto" }}
                className="rounded-[10px] border border-status-danger/20 bg-status-danger/8 px-4 py-3 text-[12px] text-status-danger"
              >
                {error}
              </motion.div>
            ) : null}

            {/* Form */}
            <form onSubmit={submit} className="space-y-4">
              <div className="space-y-3">
                <div className="relative">
                  <Mail size={15} strokeWidth={1.4} className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-txt-muted" />
                  <input
                    type="email"
                    required
                    autoComplete="username"
                    placeholder="Admin email"
                    {...register("email", { required: true })}
                    className="w-full rounded-[10px] border border-border bg-surface-0/50 py-3 pl-10 pr-4 text-[13px] text-txt outline-none transition-all placeholder:text-txt-muted focus:border-accent/40 focus:bg-surface-0/80 focus:shadow-[0_0_0_3px_rgba(99,102,241,0.08)]"
                  />
                </div>
                <div className="relative">
                  <Lock size={15} strokeWidth={1.4} className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-txt-muted" />
                  <input
                    type="password"
                    required
                    autoComplete="current-password"
                    placeholder="Password"
                    {...register("password", { required: true })}
                    className="w-full rounded-[10px] border border-border bg-surface-0/50 py-3 pl-10 pr-4 text-[13px] text-txt outline-none transition-all placeholder:text-txt-muted focus:border-accent/40 focus:bg-surface-0/80 focus:shadow-[0_0_0_3px_rgba(99,102,241,0.08)]"
                  />
                </div>
              </div>

              <Button
                type="submit"
                variant="primary"
                className="w-full justify-center rounded-[10px] py-3 text-[13px] font-semibold shadow-lg shadow-accent/20"
                disabled={isSubmitting}
              >
                {isSubmitting ? (
                  <>
                    <Loader2 size={16} strokeWidth={1.6} className="animate-spin" />
                    Signing in...
                  </>
                ) : (
                  "Sign in"
                )}
              </Button>
            </form>

            {/* Footer */}
            <p className="text-center text-[11px] text-txt-muted">
              Secured connection &middot; Hysteria 2
            </p>
          </div>
        </div>
      </motion.div>
    </div>
  );
}
