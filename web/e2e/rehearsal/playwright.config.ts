import { defineConfig } from "@playwright/test";

// The rehearsal checklist (docs/rehearsal): run through `make rehearse-test`, which starts the stack, makes a fresh guesthouse and
// sets the RH_* variables. One worker and serial files: the specs share one guesthouse and its rooms.
const out = process.env.RH_OUT ?? "../../../docs/rehearsal";
const date = process.env.RH_DATE ?? "local";
export default defineConfig({
  testDir: ".",
  workers: 1,
  fullyParallel: false,
  retries: 0,
  timeout: 120_000,
  outputDir: `${out}/evidence-${date}/_pw`,
  reporter: [["list"], ["./csv-reporter.ts"]],
  use: {
    baseURL: process.env.E2E_BASE_URL,
    screenshot: "on",
    viewport: { width: 390, height: 844 },
  },
});
