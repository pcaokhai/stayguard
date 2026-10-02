import { defineConfig } from "@playwright/test";

// e2e runs against the dev server with the mock layer (no API needed); smoke tests use E2E_BASE_URL.
const base = process.env.E2E_BASE_URL;
export default defineConfig({
  testDir: "e2e",
  use: { baseURL: base ?? "http://localhost:3100" },
  webServer: base
    ? undefined
    : {
        command: "npm run dev -- -p 3100",
        url: "http://localhost:3100/vi",
        env: { NEXT_PUBLIC_MOCK: "1" },
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      },
});
