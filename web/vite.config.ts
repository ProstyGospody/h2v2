import { fileURLToPath, URL } from "node:url";

import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const apiProxyTarget = (env.PANEL_API_INTERNAL_URL || env.VITE_API_PROXY_TARGET || "").trim().replace(/\/$/, "");
  const rawAllowedHosts = (env.VITE_ALLOWED_HOSTS || "*")
    .split(",")
    .map((value) => value.trim())
    .filter(Boolean)
    .map((value) => (value.startsWith("*.") ? `.${value.slice(2)}` : value));
  const allowedHosts = rawAllowedHosts.includes("*")
    ? true
    : ["localhost", "127.0.0.1", ...new Set(rawAllowedHosts)];

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        "@": fileURLToPath(new URL(".", import.meta.url)),
      },
    },
    server: {
      host: "0.0.0.0",
      port: 5173,
      strictPort: true,
      allowedHosts,
      proxy: apiProxyTarget
        ? {
            "/api": {
              target: apiProxyTarget,
              changeOrigin: true,
            },
          }
        : undefined,
    },
    preview: {
      host: "127.0.0.1",
      port: 13000,
      strictPort: true,
      allowedHosts,
    },
    build: {
      outDir: "dist",
      sourcemap: false,
    },
  };
});
