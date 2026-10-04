import { createHmac } from "node:crypto";
import { execFileSync } from "node:child_process";
import { expect, type Page, test } from "@playwright/test";
import en from "../messages/en.json";
import vi from "../messages/vi.json";
import { backdateRefusal } from "./support/backdateGuard";

// TT-15: a pending transfer older than qrExpiryMinutes reads as EXPIRED (derived on read, the stored row stays pending).
// The pay screen flips by itself, a new QR keeps the bill code, and a signed webhook for the old code still reaches Paid.
// Run through `make smoke` (scripts/smoke.sh sets the variables); skipped otherwise.
const configured = Boolean(process.env.SMOKE_HOOK_PATH);
const env = (n: string) => process.env[n] ?? "";
const NEW_PIN = "482916";
const EXPIRY_MINUTES = 40; // past the default qrExpiryMinutes of 30
const vnd = (s: string) => Number(s.replace(/[^\d]/g, ""));

async function typePin(page: Page, nth: number, pin: string) {
  await page.locator("input").nth(nth).click();
  await page.keyboard.type(pin);
}

async function signIn(page: Page) {
  const attempt = async (pin: string) => {
    await page.goto("/en/sign-in");
    await page.locator('input[name="guesthouseCode"]').fill(env("SMOKE_GUESTHOUSE"));
    await page.locator('input[name="username"]').fill("linh");
    await typePin(page, 2, pin);
    await page.getByRole("button", { name: "Sign in" }).click();
  };
  await attempt(NEW_PIN);
  const next = await Promise.race([
    page.waitForURL(/\/en\/rooms/).then(() => "home"),
    page
      .getByText("is not right")
      .waitFor()
      .then(() => "first"),
  ]);
  if (next === "first") {
    await attempt(env("SMOKE_RECEPTIONIST_PIN"));
    await page.waitForURL(/set-pin/);
    await typePin(page, 0, NEW_PIN);
    await typePin(page, 1, NEW_PIN);
    await page.getByRole("button", { name: "Save PIN and continue" }).click();
    await page.waitForURL(/\/en\/rooms/);
  }
}

// Moves one bill's payments back in time, in the database of the stack this run started and nowhere else (backdateRefusal).
function backdatePayments(billCode: string, minutes: number) {
  const project = env("SMOKE_COMPOSE_PROJECT");
  const code = env("SMOKE_GUESTHOUSE");
  const files = ["-f", "deploy/compose.yaml", "-f", "deploy/compose.smoke.override.yaml"];
  const dbId = execFileSync("docker", ["compose", "-p", project, ...files, "ps", "-q", "db"], {
    cwd: "..",
    encoding: "utf8",
  })
    .trim()
    .split("\n")[0];
  const talked = dbId
    ? execFileSync(
        "docker",
        ["inspect", "-f", '{{ index .Config.Labels "com.docker.compose.project" }}', dbId],
        { encoding: "utf8" },
      ).trim()
    : "";
  const refusal = backdateRefusal(process.env, talked);
  if (refusal) throw new Error(`refusing to backdate: ${refusal}`);
  const sql = `SET session_replication_role = replica;
UPDATE app.payments p SET created_at = p.created_at - make_interval(mins => ${Number(minutes)})
FROM app.invoices iv JOIN app.tenants t ON t.id = iv.tenant_id
WHERE p.invoice_id = iv.id AND p.tenant_id = iv.tenant_id AND iv.bill_code = '${billCode.replace(/[^A-Za-z0-9]/g, "")}' AND t.guesthouse_code = '${code}';`;
  execFileSync(
    "docker",
    [
      "compose",
      "-p",
      project,
      "-f",
      "deploy/compose.yaml",
      "-f",
      "deploy/compose.smoke.override.yaml",
      "exec",
      "-T",
      "db",
      "psql",
      "-U",
      "stayguard",
      "-d",
      "stayguard",
      "-v",
      "ON_ERROR_STOP=1",
    ],
    { cwd: "..", input: sql, encoding: "utf8" },
  );
}

async function deliver(
  request: import("@playwright/test").APIRequestContext,
  note: string,
  amount: number,
) {
  const body = JSON.stringify({
    id: Date.now() + Math.floor(Math.random() * 1000),
    gateway: "Vietcombank",
    transactionDate: "2026-10-02 14:30:00",
    accountNumber: env("SMOKE_ACCOUNT_NO"),
    subAccount: "",
    code: null,
    content: `${note} chuyen tien`,
    transferType: "in",
    description: "late transfer to an expired QR",
    transferAmount: amount,
    accumulated: 0,
    referenceCode: "FT-EXPIRY",
  });
  const timestamp = String(Math.floor(Date.now() / 1000));
  const signature =
    "sha256=" +
    createHmac("sha256", env("SMOKE_SEPAY_SECRET")).update(`${timestamp}.${body}`).digest("hex");
  const res = await request.post(env("E2E_BASE_URL") + env("SMOKE_HOOK_PATH"), {
    data: body,
    headers: {
      "content-type": "application/json",
      "x-sepay-signature": signature,
      "x-sepay-timestamp": timestamp,
    },
  });
  expect(res.status(), "the webhook must be accepted").toBe(200);
}

const LOCALES = [
  {
    loc: "vi",
    room: "A104",
    m: vi,
    confirm: "Xác nhận nhận phòng",
    out: "Trả phòng",
    bank: /Chuyển khoản/,
    name: "Nội dung",
  },
  {
    loc: "en",
    room: "A105",
    m: en,
    confirm: "Confirm check-in",
    out: "Check out",
    bank: /Bank transfer/,
    name: "Transfer note",
  },
] as const;

test("an expired QR flips by itself, the new QR keeps the bill code, a late transfer to the old code reaches Paid", async ({
  page,
  request,
}) => {
  test.skip(!configured, "needs the stack and test guesthouse that `make smoke` creates");
  test.setTimeout(240_000);
  await page.setViewportSize({ width: 390, height: 844 });
  await signIn(page);

  for (const { loc, room, m, confirm, out, bank, name } of LOCALES) {
    // Check in and open the QR (deposit below the bill, so a transfer is due).
    await page.goto(`/${loc}/rooms`);
    await page
      .getByRole("link", { name: new RegExp(room) })
      .first()
      .click();
    await page.locator('input[name="guestName"]').fill(`Expiry ${loc}`);
    await page.locator('input[name="guestPhone"]').fill(loc === "vi" ? "0912345672" : "0912345673");
    await page.locator('input[name="deposit"]').fill("10000");
    await page.getByRole("button", { name: confirm }).click();
    await expect(page).toHaveURL(new RegExp(`/${loc}/stay\\?id=`));
    await page.getByRole("link", { name: out }).click();
    await page.getByRole("button", { name: bank }).click();
    await expect(page.getByText(name, { exact: true })).toBeVisible();
    const text = await page.locator("body").innerText();
    const note = text.match(new RegExp(`${name}\\s+(\\S+)`))?.[1] ?? "";
    const due = vnd(text.match(/[\d.,]{5,}/)?.[0] ?? "");
    expect(note, `${loc}: the bill code`).toMatch(/^PH\d{4}/);
    expect(due).toBeGreaterThan(0);
    const oldUrl = page.url();

    // Time passes: the screen polls and flips to the expired state by itself (no reload), at both widths.
    backdatePayments(note, EXPIRY_MINUTES);
    await expect(page.getByText(m.pay.expiredTitle)).toBeVisible({ timeout: 15_000 });
    await expect(page.getByRole("button", { name: m.pay.newQr })).toBeVisible();
    await expect(page.locator("body")).not.toContainText(note);
    await page.setViewportSize({ width: 1280, height: 800 });
    await expect(page.getByText(m.pay.expiredTitle)).toBeVisible();
    await expect(page.getByRole("button", { name: m.pay.newQr })).toBeVisible();

    // A reload of the old payment shows the same expired state, never a blank page.
    await page.reload();
    await expect(page.getByText(m.pay.expiredTitle)).toBeVisible();

    // The new QR keeps the same bill code.
    await page.getByRole("button", { name: m.pay.newQr }).click();
    await expect(page.getByText(name, { exact: true })).toBeVisible();
    await expect(page).not.toHaveURL(oldUrl);
    expect(
      (await page.locator("body").innerText()).match(new RegExp(`${name}\\s+(\\S+)`))?.[1],
    ).toBe(note);
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(page.getByText(name, { exact: true })).toBeVisible();

    // The guest pays the OLD code late: the signed webhook still settles the bill and the screen turns Paid by itself.
    await deliver(request, note, due);
    await expect(page).toHaveURL(new RegExp(`/${loc}/paid\\?payment=`), { timeout: 20_000 });
  }
});
