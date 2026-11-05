import { resolve } from "node:path";
import { defineConfig } from "vite";

// Three routes: `/` (index.html), `/agents` (agents.html), `/math` (math.html).
// Vite's dev server maps `/agents` → `agents.html` and `/math` → `math.html`;
// for static hosting, serve them at those paths (clean URLs on).
export default defineConfig({
  server: { port: 5173, strictPort: true },
  build: {
    target: "es2022",
    rollupOptions: {
      input: {
        main: resolve(__dirname, "index.html"),
        agents: resolve(__dirname, "agents.html"),
        math: resolve(__dirname, "math.html"),
      },
    },
  },
});
