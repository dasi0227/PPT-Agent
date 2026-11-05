import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

export default defineConfig(({ command, mode }) => ({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: command === "serve" && mode !== "test" ? {
      "/api": {
        target: `http://${execFileSync("go", ["run", "./cmd/server", "--print-address"], {
          cwd: fileURLToPath(new URL("../backend", import.meta.url)),
          encoding: "utf8",
        }).trim()}`,
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
