import { defineConfig } from "@playwright/test";

// The rehearsal checklist (docs/rehearsal): run through `make rehearse-test`, which starts the stack, makes a fresh guesthouse and
// sets the RH_* variables. One worker and serial files: the specs share one guesthouse and its rooms.
const out = process.env.RH_OUT ?? "../../../docs/rehearsal";
const date = `${process.env.RH_DATE ?? "local"}-${process.env.RH_SLUG ?? "local"}`;
export default defineConfig({
  testDir: ".",
  workers: 1,
  fullyParallel: false,
  retries: 0,
  timeout: 120_000,
  outputDir: `${out}/evidence-${date}/_pw`,
  // Visual baselines go to docs/rehearsal/baseline, one file per screen and width, the same on every machine.
  snapshotPathTemplate: "{testDir}/../../../docs/rehearsal/baseline/{arg}{ext}",
  expect: { toHaveScreenshot: { maxDiffPixelRatio: 0.01, animations: "disabled" } },
  reporter: [["list"], ["./csv-reporter.ts"]],
  use: {
    baseURL: process.env.E2E_BASE_URL,
    screenshot: "on",
    viewport: { width: 390, height: 844 },
  },
  // Files run in name order: data-creating cases, then ui-lint, then the z* files that restart or stop the API, lock accounts and use up the rate limit.
});
