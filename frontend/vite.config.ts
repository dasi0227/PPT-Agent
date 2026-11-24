import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
export default defineConfig(({ command, mode }) => ({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: command === "serve" && mode !== "test" ? {
      "/api": {
        target: process.env.PPT_JAVA_ORIGIN || "http://127.0.0.1:8787",
        // Preserve the browser-facing Host so native file actions can validate Origin.
        changeOrigin: false,
      },
    } : undefined,
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/setupTests.ts"]
  },
}));
