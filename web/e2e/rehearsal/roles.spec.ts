import { cfg, expect, formSignIn, NEW_PIN, oneTimePin, test, uiLogin } from "./helpers";

// Roles, sign-in and sessions (docs/15 §2 and rules 12 and 13).
test("RL-01 a page the role may not open shows the no-permission screen and keeps the person signed in", async ({
  api,
  page,
}) => {
  const w = await api.as("linh");
  await uiLogin(page, w);
  await page.goto("/en/owner");
  await expect(page.getByText("You do not have access here")).toBeVisible();
  await expect(page).not.toHaveURL(/sign-in/);
  expect((await api.get(w, "/v1/me")).status, "the session is still alive").toBe(200);
  expect((await api.get(w, "/v1/owner/overview")).status).toBe(403);
  await page.goto("/en/rooms");
  await expect(page).toHaveURL(/\/en\/rooms/);
  await expect(page.getByRole("link", { name: /A1\d\d/ }).first()).toBeVisible();
});

test("RL-02 the manager lands on the room map and cannot act on the owner's own matters", async ({
  api,
  page,
}) => {
  const owner = await api.as("owner");
  await formSignIn(page, "mina", oneTimePin("mina"), true);
  await expect(page).toHaveURL(/\/en\/(owner\/)?rooms/, { timeout: 15_000 }); // the manager's room map is the owner layout
  await expect(page.locator("li", { hasText: /A101/ }).first()).toBeVisible();
  const mina = await api.as("mina");
  expect(mina.role).toBe("MANAGER");
  // What the owner keeps: linking money, bank accounts, removing staff, and anything about the owner's own account.
  const q = await api.toQr(await api.as("r1"));
  await api.pay("", q.amount, { content: "manager tries to link" });
  const unmatched = (await api.transactions(owner)).find(
    (t) => t.amount === q.amount && t.reconciliation === "UNMATCHED" && !t.billCode,
  );
  const link = await api.post(mina, `/v1/owner/payment-events/${unmatched.paymentEventId}/link`, {
    invoiceId: q.invoice.id,
  });
  expect(link.status, "link").toBe(403);
  expect(
    (
      await api.post(mina, "/v1/owner/bank-accounts", {
        bankBin: "970436",
        accountNo: "1234567890",
        accountName: "X",
        ownerPin: NEW_PIN,
      })
    ).status,
    "add bank account",
  ).toBe(403);
  const staff = await api.newStaff(owner, "rl02victim");
  expect(
    (await api.post(mina, `/v1/owner/staff/${staff.id}/remove`, { ownerPin: NEW_PIN })).status,
    "remove staff",
  ).toBe(403);
  expect(
    (await api.post(mina, `/v1/owner/staff/${owner.id}/lock`)).status,
    "lock the owner",
  ).toBeGreaterThanOrEqual(400);
  expect(
    (await api.post(mina, `/v1/owner/staff/${owner.id}/pin-reset`)).status,
    "reset the owner's PIN",
  ).toBeGreaterThanOrEqual(400);
  expect((await api.get(owner, "/v1/me")).status, "the owner is untouched").toBe(200);
});

test("RL-03 five wrong PINs lock the account and alert the owner; the owner unlocks it", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const u = await api.newStaff(owner, "rl03user");
  const codes: string[] = [];
  for (let i = 0; i < 5; i++) {
    const r = await api.signInRaw(u.username, "135790");
    codes.push(r.body.code);
  }
  expect(codes.slice(0, 4)).toEqual(Array(4).fill("PIN_INVALID"));
  expect(codes[4]).toBe("ACCOUNT_LOCKED");
  const right = await api.signInRaw(u.username, u.pin);
  expect(right.status, "even the right PIN is refused while locked").toBe(401);
  expect(right.body.code).toBe("ACCOUNT_LOCKED");
  expect((await api.alerts(owner)).some((a) => a.kind === "ACCOUNT_LOCKED")).toBe(true);
  expect((await api.post(owner, `/v1/owner/staff/${u.id}/unlock`)).status).toBeLessThan(300);
  expect((await api.signInRaw(u.username, u.pin)).status, "signed in after unlock").toBe(200);
});

// Signs a new staff member in on several "devices" (sessions) and returns them.
async function sessions(
  api: import("./helpers").Api,
  owner: import("./helpers").Who,
  username: string,
  n: number,
) {
  const u = await api.newStaff(owner, username);
  const first = await api.signInRaw(u.username, u.pin);
  const t1 = { user: username, token: first.body.accessToken, id: u.id, role: "RECEPTIONIST" };
  expect((await api.put(t1, "/v1/me/pin", { currentPin: u.pin, newPin: "246802" })).status).toBe(
    204,
  );
  const rest = [];
  for (let i = 1; i < n; i++) {
    const r = await api.signInRaw(u.username, "246802");
    expect(r.status).toBe(200);
    rest.push({ ...t1, token: r.body.accessToken as string });
  }
  return { u, first: t1, rest };
}

test("RL-04 changing the PIN signs out every other session but not this one", async ({ api }) => {
  const owner = await api.as("owner");
  const { first, rest } = await sessions(api, owner, "rl04user", 3);
  for (const s of [first, ...rest]) expect((await api.get(s, "/v1/me")).status).toBe(200);
  const change = await api.put(rest[0], "/v1/me/pin", { currentPin: "246802", newPin: "579135" });
  expect(change.status).toBe(204);
  expect((await api.get(rest[0], "/v1/me")).status, "the session that changed it").toBe(200);
  expect((await api.get(first, "/v1/me")).status, "another session").toBe(401);
  expect((await api.get(rest[1], "/v1/me")).status, "another session").toBe(401);
});

test("RL-05 locking an account signs out every session at once", async ({ api }) => {
  const owner = await api.as("owner");
  const { u, first, rest } = await sessions(api, owner, "rl05user", 2);
  expect((await api.post(owner, `/v1/owner/staff/${u.id}/lock`)).status).toBeLessThan(300);
  for (const s of [first, ...rest]) expect((await api.get(s, "/v1/me")).status).toBe(401);
  expect((await api.signInRaw(u.username, "246802")).status, "cannot sign in while locked").toBe(
    401,
  );
});

test("RL-06 removing a person signs them out at once and ends their sign-in", async ({ api }) => {
  const owner = await api.as("owner");
  const { u, first, rest } = await sessions(api, owner, "rl06user", 2);
  const wrong = await api.post(owner, `/v1/owner/staff/${u.id}/remove`, { ownerPin: "000000" });
  expect(wrong.status, "the owner's PIN is asked again").toBe(403);
  expect((await api.get(first, "/v1/me")).status, "a wrong owner PIN removes nobody").toBe(200);
  expect(
    (await api.post(owner, `/v1/owner/staff/${u.id}/remove`, { ownerPin: NEW_PIN })).status,
  ).toBeLessThan(300);
  for (const s of [first, ...rest]) expect((await api.get(s, "/v1/me")).status).toBe(401);
  expect((await api.signInRaw(u.username, "246802")).status).toBe(401);
});

test("RL-07 a second browser tab keeps the session through the cookie", async ({ api, page }) => {
  await api.as("linh"); // makes sure the PIN is chosen
  await formSignIn(page, "linh", NEW_PIN);
  await expect(page).toHaveURL(/\/en\/rooms/, { timeout: 15_000 });
  const cookie = (await page.context().cookies()).find((c) => c.name === "sg_session");
  expect(cookie?.httpOnly, "an HttpOnly session cookie").toBe(true);
  expect(
    await page.evaluate(() => JSON.stringify({ ...sessionStorage, ...localStorage })),
    "no token in the page",
  ).not.toContain(cookie!.value);
  const tab2 = await page.context().newPage();
  await tab2.goto("/en/rooms");
  await expect(tab2).toHaveURL(/\/en\/rooms/);
  await expect(tab2.getByRole("link", { name: /A1\d\d/ }).first()).toBeVisible();
  await tab2.goto("/en/shift");
  await expect(tab2).not.toHaveURL(/sign-in/);
});

test("RL-08 a cookie-authenticated write without X-Requested-With is rejected (CSRF)", async ({
  api,
}) => {
  const w = await api.as("linh");
  const cookie = { cookie: `sg_session=${w.token}`, origin: cfg.base };
  const bare = await api.call(null, "PUT", "/v1/me/locale", { locale: "en" }, { headers: cookie });
  expect(bare.status).toBe(403);
  expect(bare.body.code).toBe("CSRF_REJECTED");
  const foreign = await api.call(
    null,
    "PUT",
    "/v1/me/locale",
    { locale: "en" },
    { headers: { ...cookie, "x-requested-with": "stayguard", origin: "https://evil.example" } },
  );
  expect(foreign.status, "a foreign Origin").toBe(403);
  const ok = await api.call(
    null,
    "PUT",
    "/v1/me/locale",
    { locale: "en" },
    { headers: { ...cookie, "x-requested-with": "stayguard" } },
  );
  expect(ok.status, JSON.stringify(ok.body)).toBeLessThan(300);
  const bearer = await api.put(w, "/v1/me/locale", { locale: "en" });
  expect(bearer.status, "Bearer needs no header").toBeLessThan(300);
});

// Last on purpose: it uses up the sign-in budget (20 a minute per address) and then waits for it to come back.
test("RL-09 hitting the sign-in rate limit is not shown as a locked account", async ({
  api,
  page,
}) => {
  await api.as("linh");
  for (let i = 0; i < 22; i++) {
    // An unknown guesthouse code: nobody's account is touched, only the per-address counter.
    await api.post(null, "/v1/auth/sign-in", {
      guesthouseCode: "nosuchcode",
      username: "x",
      pin: "135790",
    });
  }
  await formSignIn(page, "linh", NEW_PIN);
  await page.waitForTimeout(2_500);
  const shown = await page.locator("main").innerText();
  await sleep(61_000); // let the limiter forget, so the cases after this one are not refused
  expect(shown, "a rate limit is not an account lock").not.toContain("Account temporarily locked");
  expect(shown, "nobody was notified of a lock").not.toContain("The owner has been notified");
});
