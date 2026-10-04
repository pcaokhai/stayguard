import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  // Vite's built-in transform handles JSX; Next's tsconfig uses "preserve", so set the runtime here.
  oxc: { jsx: { runtime: "automatic" } },
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  test: { include: ["src/**/*.test.{ts,tsx}", "messages/*.test.ts", "e2e/support/*.test.ts"] },
});
