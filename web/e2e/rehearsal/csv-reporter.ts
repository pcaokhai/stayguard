import { copyFileSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import type { FullResult, Reporter, TestCase, TestResult } from "@playwright/test/reporter";

// One CSV row per test: id (first word of the title), title, status pass/fail/skip, seconds, evidence folder, notes.
// Evidence (screenshots, the API log each test attaches, the error) is copied to evidence-<date>/<id>/.
const out = process.env.RH_OUT ?? "docs/rehearsal";
const date = process.env.RH_DATE ?? "local";
const slug = process.env.RH_SLUG ?? "local"; // the clone: two clones never write the same file
const stem = `${date}-${slug}`;
const evidence = join(out, `evidence-${stem}`);
const csv = (v: string) => `"${v.replace(/"/g, '""')}"`;

class CsvReporter implements Reporter {
  private rows: string[] = [];
  private counts = { pass: 0, fail: 0, skip: 0 };
  private suites = new Map<string, { pass: number; fail: number; skip: number; secs: number }>();

  onTestEnd(test: TestCase, result: TestResult) {
    const id = test.title.split(" ")[0];
    const status =
      result.status === "passed" ? "pass" : result.status === "skipped" ? "skip" : "fail";
    this.counts[status] += 1;
    const suite = test.location.file
      .split("/")
      .pop()!
      .replace(/\.spec\.ts$/, "");
    const sc = this.suites.get(suite) ?? { pass: 0, fail: 0, skip: 0, secs: 0 };
    sc[status] += 1;
    sc.secs += result.duration / 1000;
    this.suites.set(suite, sc);
    const dir = join(evidence, id);
    mkdirSync(dir, { recursive: true });
    result.attachments.forEach((a, i) => {
      const ext =
        a.contentType === "image/png"
          ? ".png"
          : a.contentType === "application/json"
            ? ".json"
            : ".txt";
      const name = `${i}-${a.name}`.replace(/[^\w.-]/g, "_").replace(/(\.\w+)?$/, (m) => m || ext);
      if (a.path) copyFileSync(a.path, join(dir, name));
      else if (a.body) writeFileSync(join(dir, name), a.body);
    });
    const skipNote = test.annotations.find((x) => x.type === "skip")?.description ?? "";
    const err = result.error?.message?.replace(/\u001b\[[0-9;]*m/g, "") ?? "";
    if (err) writeFileSync(join(dir, "error.txt"), err);
    const notes =
      status === "fail" ? err.split("\n").slice(0, 3).join(" ").slice(0, 400) : skipNote;
    this.rows.push(
      [
        id,
        csv(test.title),
        status,
        (result.duration / 1000).toFixed(1),
        csv(`docs/rehearsal/evidence-${stem}/${id}`),
        csv(notes),
      ].join(","),
    );
  }

  onEnd(_result: FullResult) {
    rmSync(join(evidence, "_pw"), { recursive: true, force: true });
    mkdirSync(out, { recursive: true });
    // The demo-check: one line per suite (spec file), after the cases.
    const demo = [...this.suites].map(
      ([n, c]) =>
        `${n},${c.fail ? "fail" : "pass"},${c.pass},${c.fail},${c.skip},${c.secs.toFixed(1)}`,
    );
    writeFileSync(
      join(out, `results-${stem}.csv`),
      [
        "id,title,status,duration_s,evidence,notes",
        ...this.rows,
        "",
        "# demo-check",
        `# commit ${process.env.RH_COMMIT ?? "unknown"}`,
        "suite,status,pass,fail,skip,duration_s",
        ...demo,
      ].join("\n") + "\n",
    );
    const c = this.counts;
    console.log(`rehearsal: pass ${c.pass}  fail ${c.fail}  skip ${c.skip}`);
  }
}

export default CsvReporter;
