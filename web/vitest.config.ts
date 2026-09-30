import { defineConfig } from "vitest/config";

export default defineConfig({
  // Vite's built-in transform handles JSX; Next's tsconfig uses "preserve", so set the runtime here.
  oxc: { jsx: { runtime: "automatic" } },
  test: { include: ["src/**/*.test.{ts,tsx}", "messages/*.test.ts"] },
});
