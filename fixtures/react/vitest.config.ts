import { defineConfig } from "vitest/config";

export default defineConfig({
  esbuild: { jsx: "automatic" },
  // mutate runs tests in symlink overlays (target/mutation-workers/...). The
  // jsdom environment loads files through Vite's dev server, which refuses
  // the symlinks' real paths outside the worker root; every mutant would be
  // "killed" by a load error.
  server: { fs: { strict: false } },
  test: {
    environment: "jsdom",
    include: ["tests/**/*.test.{ts,tsx}"],
  },
});
