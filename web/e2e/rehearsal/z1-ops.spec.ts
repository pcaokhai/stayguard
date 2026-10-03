import { cfg, expect, NEW_PIN, oneTimePin, stack, test, uiLogin } from "./helpers";

// Operations: a restart, the readiness check and what the logs may hold (docs/15 rule 12; root CLAUDE.md rule 9).
test("VH-06 restarting the API keeps people signed in", async ({ api, page }) => {
  const owner = await api.as("owner");
  expect((await api.get(owner, "/v1/me")).status).toBe(200);
  await uiLogin(page, owner);
  stack.restartApi();
  await stack.ready();
  expect((await api.get(owner, "/v1/me")).status, "the same session after the restart").toBe(200);
  await page.goto("/en/owner");
  await expect(page).not.toHaveURL(/sign-in/);
  await expect(page.locator("main")).toBeVisible();
});

test("VH-05 /readyz and /healthz answer 200", async ({ api }) => {
  for (const path of ["/readyz", "/healthz"]) {
    const r = await api.get(null, path);
    expect(r.status, path).toBe(200);
  }
});

test("BM-03 PINs, the SePay secret, bank account numbers and session tokens are not in the logs", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const linh = await api.as("linh");
  // Six-digit PINs and the account number are digits, so match them as whole numbers: a timestamp or trace id may contain the same digits.
  const whole = (v: string) => new RegExp(`(?<![0-9A-Za-z])${v}(?![0-9A-Za-z])`);
  const numeric: Record<string, string> = {
    "chosen PIN": NEW_PIN,
    "one-time PIN of the owner": oneTimePin("owner"),
    "bank account number": cfg.account,
  };
  const secrets: Record<string, string> = {
    "SePay secret": cfg.secret,
    "owner session token": owner.token,
    "front desk session token": linh.token,
  };
  for (const service of ["api", "jobs", "db"]) {
    const logs = stack.logs(service);
    for (const [what, value] of Object.entries(secrets))
      expect(logs, `${what} in ${service} logs`).not.toContain(value);
    for (const [what, value] of Object.entries(numeric))
      expect(logs, `${what} in ${service} logs`).not.toMatch(whole(value));
  }
});
