import { defineConfig } from "vitest/config";

// fixtures/react's config, with a setup file; no server.fs override.
export default defineConfig({
  esbuild: { jsx: "automatic" },
  test: {
    environment: "jsdom",
    include: ["tests/**/*.test.{ts,tsx}"],
    setupFiles: ["./tests/setup.ts"],
  },
});
