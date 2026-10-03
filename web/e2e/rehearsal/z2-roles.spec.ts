import {
  cfg,
  chosenPin,
  expect,
  formSignIn,
  NEW_PIN,
  oneTimePin,
  sleep,
  test,
  uiLogin,
} from "./helpers";

// Roles, sign-in and sessions (docs/15 §2 and rules 12 and 13).
test("DN-07 a page the role may not open shows the no-permission screen and keeps the person signed in", async ({
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

test("DN-06 the manager lands on the room map", async ({ api, page }) => {
  const owner = await api.as("owner");
  const chosen = chosenPin("mina"); // the UI lint may already have signed mina in
  await formSignIn(page, "mina", chosen ?? oneTimePin("mina"), !chosen);
  await expect(page).toHaveURL(/\/en\/(owner\/)?rooms/, { timeout: 15_000 }); // the manager's room map is the owner layout
  await expect(page.locator("li", { hasText: /A101/ }).first()).toBeVisible();
  expect((await api.as("mina")).role).toBe("MANAGER");
});

test("DN-05 the manager cannot act on the owner's own matters", async ({ api }) => {
  const owner = await api.as("owner");
  const mina = await api.as("mina");
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

test("DN-01 five wrong PINs lock the account and alert the owner; the owner unlocks it", async ({
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

test("DN-09 changing the PIN, locking or removing a person ends their other sessions", async ({
  api,
}) => {
  await test.step("changing the PIN signs out every other session but not this one", async () => {
    const owner = await api.as("owner");
    const { first, rest } = await sessions(api, owner, "rl04user", 3);
    for (const s of [first, ...rest]) expect((await api.get(s, "/v1/me")).status).toBe(200);
    const change = await api.put(rest[0], "/v1/me/pin", { currentPin: "246802", newPin: "579135" });
    expect(change.status).toBe(204);
    expect((await api.get(rest[0], "/v1/me")).status, "the session that changed it").toBe(200);
    expect((await api.get(first, "/v1/me")).status, "another session").toBe(401);
    expect((await api.get(rest[1], "/v1/me")).status, "another session").toBe(401);
  });
  await test.step("locking an account signs out every session at once", async () => {
    const owner = await api.as("owner");
    const { u, first, rest } = await sessions(api, owner, "rl05user", 2);
    expect((await api.post(owner, `/v1/owner/staff/${u.id}/lock`)).status).toBeLessThan(300);
    for (const s of [first, ...rest]) expect((await api.get(s, "/v1/me")).status).toBe(401);
    expect((await api.signInRaw(u.username, "246802")).status, "cannot sign in while locked").toBe(
      401,
    );
  });
  await test.step("removing a person signs them out at once; the owner's PIN is asked again (DN-04)", async () => {
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
});

test("DN-13 a second browser tab keeps the session through the cookie", async ({ api, page }) => {
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

test("DN-15 a cookie-authenticated write without X-Requested-With is rejected (CSRF)", async ({
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

const day = (offset: number) =>
  new Date(Date.now() + offset * 86_400_000).toISOString().slice(0, 10);

test("CD-10 a receptionist asking for stays older than the front-desk days is refused", async ({
  api,
}) => {
  const w = await api.as("r1");
  const owner = await api.as("owner");
  const days = (await api.get(owner, "/v1/owner/property")).body.frontDeskHistoryDays ?? 7;
  expect((await api.get(w, `/v1/stays?date=${day(-1)}`)).status, "yesterday").toBe(200);
  const old = await api.get(w, `/v1/stays?date=${day(-(days + 5))}`);
  expect(old.status, `${days + 5} days back, the limit is ${days}`).toBe(422);
  expect((await api.get(owner, `/v1/stays?date=${day(-(days + 5))}`)).status, "the owner may").toBe(
    200,
  );
});

test("GT-06 a receptionist's stay list carries ID indicator flags, never the number", async ({
  api,
}) => {
  const w = await api.as("r1");
  const number = `079${String(Math.floor(Math.random() * 1e9)).padStart(9, "0")}`;
  const stay = await api.checkIn(w, { idNumber: number });
  const list = await api.get(w, `/v1/stays?date=${day(0)}`);
  expect(list.status).toBe(200);
  const item = list.body.items.find((x: any) => x.id === stay.id);
  expect(item, "the new stay is listed").toBeTruthy();
  expect(JSON.stringify(item.guestId ?? item), "flags only").toMatch(/hasIdNumber/);
  expect(JSON.stringify(list.body), "no number in the list").not.toContain(number);
  expect(JSON.stringify(list.body)).not.toMatch(/"idNumber"/);
});

test("DN-12 a manager sees only the buildings they have access to, with VIEW or more", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const mina = await api.as("mina");
  const made = await api.post(owner, "/v1/owner/buildings", {
    code: "Z",
    name: "Building Z",
    floors: 1,
    roomsPerFloor: 2,
    unitTypeCode: "STANDARD",
  });
  expect(made.status, JSON.stringify(made.body)).toBeLessThan(300);
  const zId = made.body.id;
  const codes = async (w: typeof owner) =>
    ((await api.get(w, "/v1/buildings")).body.items as any[]).map((b) => b.code);
  expect(await codes(owner), "the owner sees every building").toContain("Z");
  expect(await codes(mina), "a new building starts with no staff access").not.toContain("Z");
  expect((await api.get(mina, `/v1/buildings/${zId}/rooms`)).status).toBeGreaterThanOrEqual(400);
  const grant = await api.put(owner, `/v1/owner/staff/${mina.id}/building-permissions/${zId}`, {
    level: "VIEW",
  });
  expect(grant.status, JSON.stringify(grant.body)).toBeLessThan(300);
  const seen = ((await api.get(mina, "/v1/buildings")).body.items as any[]).find(
    (b) => b.code === "Z",
  );
  expect(seen?.level, "visible with VIEW").toBe("VIEW");
  expect((await api.get(mina, `/v1/buildings/${zId}/rooms`)).status).toBe(200);
  const room = (await api.get(mina, `/v1/buildings/${zId}/rooms`)).body.items[0];
  expect(
    (
      await api.post(mina, `/v1/rooms/${room.id}/stays`, {
        rentalType: "HOURLY",
        guestName: "X",
        guestPhone: "0912345678",
        deposit: 0,
      })
    ).status,
    "VIEW cannot check in",
  ).toBe(403);
  await api.put(owner, `/v1/owner/staff/${mina.id}/building-permissions/${zId}`, { level: "NONE" });
  expect(await codes(mina), "NONE hides it again").not.toContain("Z");
});

// Last on purpose: it uses up the sign-in budget (20 a minute per address) and then waits for it to come back.
test("DN-16 hitting the sign-in rate limit is not shown as a locked account", async ({
  api,
  page,
}) => {
  test.setTimeout(150_000);
  await api.as("linh");
  for (let i = 0; i < 22; i++) {
    // An unknown guesthouse code: nobody's account is touched, only the per-address counter. Sent raw, past the helper's own throttle.
    await api.request.post(`${cfg.base}/v1/auth/sign-in`, {
      data: { guesthouseCode: "nosuchcode", username: "x", pin: "135790" },
      failOnStatusCode: false,
    });
  }
  await formSignIn(page, "linh", NEW_PIN);
  await page.waitForTimeout(2_500);
  const shown = await page.locator("main").innerText();
  await sleep(61_000); // let the limiter forget, so the cases after this one are not refused
  expect(shown, "a rate limit is not an account lock").not.toContain("Account temporarily locked");
  expect(shown, "nobody was notified of a lock").not.toContain("The owner has been notified");
});

test("DN-02 a wrong code, user name or PIN all get the same answer", async ({ api }) => {
  const owner = await api.as("owner");
  const u = await api.newStaff(owner, "dn02user");
  const bodies = [
    await api.post(null, "/v1/auth/sign-in", {
      guesthouseCode: "nosuchcode",
      username: u.username,
      pin: "135790",
    }),
    await api.post(null, "/v1/auth/sign-in", {
      guesthouseCode: cfg.guesthouse,
      username: "nosuchuser",
      pin: "135790",
    }),
    await api.post(null, "/v1/auth/sign-in", {
      guesthouseCode: cfg.guesthouse,
      username: u.username,
      pin: "135790",
    }),
  ];
  for (const b of bodies) expect(b.status).toBe(401);
  expect(new Set(bodies.map((b) => JSON.stringify(b.body))).size, "one and the same answer").toBe(
    1,
  );
});

test("DN-03 a PIN that is easy to guess is refused", async ({ api }) => {
  const owner = await api.as("owner");
  const u = await api.newStaff(owner, "dn03user");
  const s = await api.signInRaw(u.username, u.pin);
  const t = { user: u.username, token: s.body.accessToken, id: u.id, role: "RECEPTIONIST" };
  for (const weak of ["111111", "123456", "000000", "654321"]) {
    expect((await api.put(t, "/v1/me/pin", { currentPin: u.pin, newPin: weak })).status, weak).toBe(
      422,
    );
  }
  expect((await api.put(t, "/v1/me/pin", { currentPin: u.pin, newPin: "482916" })).status).toBe(
    204,
  );
});
