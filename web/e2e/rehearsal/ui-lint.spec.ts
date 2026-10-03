import { readFileSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "@playwright/test";
import { expect, test, uiLogin, type Who } from "./helpers";

// UI-01 (proposed new checklist row): every screen of every role, in vi and en at 390 and 1280, must read like a finished product.
// The routes are the ones scripts/rehearsal-shots.sh sweeps; the sessions are the ones rehearsal-session.py makes.
const ROLES: { role: string; user: string; routes: string[] }[] = [
  {
    role: "owner",
    user: "owner",
    routes:
      "/owner /owner/rooms /owner/alerts /owner/transactions /owner/stays /owner/shifts /owner/activity /owner/reports /owner/expenses /owner/maintenance /owner/items /owner/staff /owner/access /owner/property /owner/buildings /owner/rates /owner/roster /owner/payroll /account".split(
        " ",
      ),
  },
  {
    role: "manager",
    user: "mina",
    routes: "/owner/rooms /owner/alerts /owner/maintenance /owner/stays /account".split(" "),
  },
  {
    role: "receptionist",
    user: "linh",
    routes: "/rooms /stays /shift /shift/payout /me/schedule /me/leave /account".split(" "),
  },
  {
    role: "housekeeping",
    user: "hk",
    routes: "/housekeeping /me/schedule /me/leave /account".split(" "),
  },
];
const LOCALES = ["vi", "en"];
const WIDTHS = [390, 1280];
const NAMESPACES = Object.keys(
  JSON.parse(readFileSync(join(process.cwd(), "messages/vi.json"), "utf8")),
);
const RAW_KEY = new RegExp(
  `(?<![\\w@/.-])(?:${NAMESPACES.join("|")})\\.[A-Za-z_][\\w]*(?:\\.[\\w]+)*`,
);
const CHECKS: [string, RegExp][] = [
  ["placeholder {name}", /\{[A-Za-z_]+\}/],
  ["raw i18n key", RAW_KEY],
  ["undefined/NaN/null text", /\b(undefined|NaN|null|Invalid Date)\b|\[object Object\]/],
];
const UPPER_SNAKE = /\b[A-Z]{2,}(?:_[A-Z0-9]+)+\b/;
// Responses a screen may get without being broken: no open shift yet.
const EXPECTED = [/\/v1\/shifts\/current$/];
const noScroll = () => document.documentElement.scrollWidth > window.innerWidth + 1;

async function whoOf(page: Page) {
  return page.evaluate(() => document.body.innerText);
}

// On the activity log and alerts: raw action codes and an empty actor cell are bugs.
async function listProblems(page: Page, route: string): Promise<string[]> {
  if (!/\/owner\/(activity|alerts)/.test(route)) return [];
  const out: string[] = [];
  const rows = await page.locator("li, tr").allInnerTexts();
  const code = rows.map((t) => t.match(UPPER_SNAKE)?.[0]).find(Boolean);
  if (code) out.push(`raw action code "${code}" visible in a row`);
  const empty = await page.evaluate(() => {
    const th = [...document.querySelectorAll("th")].findIndex((h) =>
      /^(Người|Who|Actor|By)$/i.test(h.textContent?.trim() ?? ""),
    );
    if (th >= 0)
      return [...document.querySelectorAll("tbody tr")].filter(
        (tr) => !(tr.children[th]?.textContent ?? "").trim(),
      ).length;
    return [...document.querySelectorAll("dt, span, p")].filter(
      (e) =>
        /^(Người|Who|Actor|By)$/i.test(e.textContent?.trim() ?? "") &&
        !(e.nextElementSibling?.textContent ?? "").trim(),
    ).length;
  });
  if (empty) out.push(`${empty} row(s) with an empty Người/actor cell`);
  return out;
}

test("UI-01 every screen of every role reads like a finished product", async ({
  api,
  browser,
}, testInfo) => {
  test.setTimeout(900_000);
  const problems: { pattern: string; where: string; detail: string }[] = [];
  for (const { role, user, routes } of ROLES) {
    const who: Who = await api.as(user);
    const stay = ((await api.get(who, `/v1/stays?date=${new Date().toISOString().slice(0, 10)}`))
      .body?.items ?? [])[0]?.id;
    const all = stay
      ? [
          ...routes,
          role === "receptionist"
            ? `/stay?id=${stay}`
            : /owner|manager/.test(role)
              ? `/owner/stay?id=${stay}`
              : "",
        ]
      : routes;
    for (const width of WIDTHS) {
      const ctx = await browser.newContext({
        viewport: { width, height: width > 600 ? 900 : 844 },
      });
      const page = await ctx.newPage();
      await uiLogin(page, who);
      for (const locale of LOCALES) {
        for (const route of all.filter(Boolean)) {
          const where = `${route} · ${role} · ${locale} · ${width}`;
          const add = (pattern: string, detail: string) =>
            problems.push({ pattern, where, detail });
          const consoleErrors: string[] = [];
          const onConsole = (m: import("@playwright/test").ConsoleMessage) => {
            if (m.type() === "error" && !/Failed to load resource/.test(m.text()))
              consoleErrors.push(m.text().slice(0, 120));
          };
          const onPageError = (e: Error) =>
            consoleErrors.push(`pageerror: ${e.message.slice(0, 120)}`);
          const onResponse = (r: import("@playwright/test").Response) => {
            if (
              r.status() >= 400 &&
              r.url().startsWith(process.env.E2E_BASE_URL ?? "") &&
              !EXPECTED.some((x) => x.test(new URL(r.url()).pathname))
            )
              add("4xx/5xx response", `${r.status()} ${new URL(r.url()).pathname}`);
          };
          page.on("console", onConsole);
          page.on("pageerror", onPageError);
          page.on("response", onResponse);
          await page.goto(`/${locale}${route}`);
          await page.waitForLoadState("load");
          await page.waitForTimeout(1_500); // let the data settle: screens poll, so there is no network-idle
          const text = await whoOf(page);
          for (const [pattern, re] of CHECKS) {
            const m = text.match(re);
            if (m) add(pattern, `"${m[0]}"`);
          }
          for (const d of await listProblems(page, route)) add("activity/alerts list", d);
          for (const e of consoleErrors) add("console/page error", e);
          if (await page.evaluate(noScroll))
            add(
              "horizontal page scroll",
              `${await page.evaluate(() => document.documentElement.scrollWidth)} > ${width}`,
            );
          page.off("console", onConsole);
          page.off("pageerror", onPageError);
          page.off("response", onResponse);
        }
      }
      await ctx.close();
    }
  }
  const lines = problems.map((p) => `[${p.pattern}] ${p.where} — ${p.detail}`);
  const byPattern = new Map<string, number>();
  for (const p of problems) byPattern.set(p.pattern, (byPattern.get(p.pattern) ?? 0) + 1);
  const summary = [...byPattern].map(([k, n]) => `${k}: ${n}`).join("; ");
  await testInfo.attach("ui-lint.txt", {
    body: [summary, ...[...new Set(lines)]].join("\n"),
    contentType: "text/plain",
  });
  expect(problems.length, `${problems.length} problems — ${summary}`).toBe(0);
});
