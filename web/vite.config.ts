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
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes("/recharts/") || id.includes("/d3-") || id.includes("/victory-vendor/")) {
              return "vendor-recharts";
            }
            if (id.includes("/@tanstack/react-table") || id.includes("/@tanstack/react-virtual")) {
              return "vendor-table";
            }
            if (id.includes("/framer-motion/") || id.includes("/motion/")) {
              return "vendor-motion";
            }
            if (id.includes("/@tanstack/react-query")) {
              return "vendor-query";
            }
            if (id.includes("/@radix-ui/")) {
              return "vendor-radix";
            }
            if (id.includes("/node_modules/react/") || id.includes("/node_modules/react-dom/") || id.includes("/node_modules/react-router")) {
              return "vendor-react";
            }
          },
        },
      },
    },
  };
});
