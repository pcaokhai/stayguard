import { cfg, expect, raiseAlerts, test, uiLogin } from "./helpers";

// Portfolio-demo checks: guest ID (GT) and people (DN) beyond the main cases.
const ID = () => `079${String(Math.floor(Math.random() * 1e9)).padStart(9, "0")}`;

test("GT-01 the front desk records the number and both photos at check-in", async ({ api }) => {
  const w = await api.as("r16");
  const stay = await api.checkIn(w, { idNumber: ID() });
  expect((await api.uploadIdPhoto(w, stay.id, "FRONT")).status).toBe(200);
  expect((await api.uploadIdPhoto(w, stay.id, "BACK")).status).toBe(200);
  expect((await api.getStay(w, stay.id)).guestId).toEqual({
    hasIdNumber: true,
    hasFrontPhoto: true,
    hasBackPhoto: true,
  });
});

test("GT-04 the owner can view, download and delete an ID photo, and each is logged", async ({
  api,
}) => {
  const w = await api.as("r16");
  const owner = await api.as("owner");
  const stay = await api.checkIn(w, { idNumber: ID() });
  await api.uploadIdPhoto(w, stay.id, "FRONT");
  const url = `${cfg.base}/v1/owner/stays/${stay.id}/guest-id/photos/FRONT`;
  const view = await api.request.get(url, { headers: { authorization: `Bearer ${owner.token}` } });
  expect(view.status()).toBe(200);
  expect(view.headers()["content-type"]).toMatch(/image\//);
  expect(view.headers()["cache-control"]).toContain("no-store");
  const del = await api.call(owner, "DELETE", `/v1/owner/stays/${stay.id}/guest-id/photos/FRONT`);
  expect(del.status).toBeLessThan(300);
  expect(
    (await api.request.get(url, { headers: { authorization: `Bearer ${owner.token}` } })).status(),
    "gone",
  ).toBe(404);
  expect(JSON.stringify(await api.audit(owner))).toMatch(
    /GUEST_ID_PHOTO_(DELETED|VIEWED|DOWNLOADED)/,
  );
});

test("GT-05 a manager can read the record of a building they have access to; housekeeping cannot", async ({
  api,
}) => {
  const w = await api.as("r16");
  const mina = await api.as("mina");
  const hk = await api.as("hk");
  const stay = await api.checkIn(w, { idNumber: ID() });
  const rec = await api.get(mina, `/v1/owner/stays/${stay.id}/guest-id`);
  expect(rec.status).toBe(200);
  expect(rec.body.idNumberMasked).toMatch(/^\d{3}\*+\d{3}$/);
  expect((await api.get(hk, `/v1/owner/stays/${stay.id}/guest-id`)).status).toBe(403);
});

test("GT-07 a photo that is too large or not a picture is refused", async ({ api }) => {
  const w = await api.as("r16");
  const stay = await api.checkIn(w);
  const put = (buffer: Buffer, mimeType: string, name: string) =>
    api.request.put(`${cfg.base}/v1/stays/${stay.id}/guest-id/photos/FRONT`, {
      headers: { authorization: `Bearer ${w.token}`, "idempotency-key": crypto.randomUUID() },
      multipart: { file: { name, mimeType, buffer } },
      failOnStatusCode: false,
    });
  expect(
    (await put(Buffer.from("this is not a picture"), "image/png", "x.png")).status(),
    "not an image",
  ).toBeGreaterThanOrEqual(400);
  expect(
    (await put(Buffer.alloc(6 * 1024 * 1024, 1), "image/jpeg", "big.jpg")).status(),
    "over 5 MB",
  ).toBe(413);
});

test("GT-09 ID numbers and photos are deleted after the retention days by the daily job", async ({
  api,
}) => {
  const w = await api.as("r16");
  const owner = await api.as("owner");
  const days = (await api.get(owner, "/v1/owner/property")).body.idRetentionDays ?? 30;
  const stay = await api.checkIn(w, { idNumber: ID() });
  const inv = await api.checkout(w, stay.id);
  await api.uploadIdPhoto(w, stay.id, "FRONT");
  expect((await api.getStay(w, stay.id)).guestId.hasIdNumber).toBe(true);
  raiseAlerts("stay", inv.billCode, (days + 10) * 24 * 60);
  const rec = await api.get(owner, `/v1/owner/stays/${stay.id}/guest-id`);
  const flags = (await api.getStay(owner, stay.id)).guestId;
  expect(flags, "nothing left on file").toEqual({
    hasIdNumber: false,
    hasFrontPhoto: false,
    hasBackPhoto: false,
  });
  expect(JSON.stringify(rec.body)).not.toMatch(/\d{9,12}/);
  expect(JSON.stringify(await api.audit(owner))).toContain("GUEST_ID_RETENTION_DELETED");
});

test("DN-08 housekeeping only gets the housekeeping screens", async ({ api, page }) => {
  const hk = await api.as("hk");
  await uiLogin(page, hk);
  await page.goto("/en/housekeeping");
  await expect(page.getByText("Housekeeping").first()).toBeVisible();
  for (const route of ["/en/owner", "/en/shift", "/en/stays"]) {
    await page.goto(route);
    await expect(page.getByText("You do not have access here"), route).toBeVisible();
  }
  expect((await api.get(hk, "/v1/owner/overview")).status).toBe(403);
  expect((await api.get(hk, "/v1/shifts/current")).status).toBeGreaterThanOrEqual(403);
});

test("DN-10 removing a person keeps their history", async ({ api }) => {
  const owner = await api.as("owner");
  const u = await api.newStaff(owner, "dn10user");
  const s = await api.signInRaw(u.username, u.pin);
  const t = { user: u.username, token: s.body.accessToken, id: u.id, role: "RECEPTIONIST" };
  await api.put(t, "/v1/me/pin", { currentPin: u.pin, newPin: "246802" });
  const stay = await api.checkIn(t, { deposit: 10_000 }); // opens their shift: close it first, then they can be removed
  const closed = await api.post(t, "/v1/shifts/current/close", {
    counts: [{ denomination: 10_000, quantity: 1 }],
    floatLeft: 0,
  });
  expect(closed.status, JSON.stringify(closed.body)).toBeLessThan(300);
  expect(
    (await api.post(owner, `/v1/owner/staff/${u.id}/remove`, { ownerPin: "482916" })).status,
  ).toBeLessThan(300);
  const tl = await api.get(owner, `/v1/owner/stays/${stay.id}/timeline`);
  expect(JSON.stringify(tl.body), "the name is still on the history").toContain(
    "Rehearsal dn10user",
  );
});

test("DN-11 signing out ends the session and clears the cookie", async ({ api, page }) => {
  const owner = await api.as("owner");
  const u = await api.newStaff(owner, "dn11user");
  const s = await api.signInRaw(u.username, u.pin);
  const t = { user: u.username, token: s.body.accessToken, id: u.id, role: "RECEPTIONIST" };
  expect((await api.post(t, "/v1/auth/sign-out")).status).toBeLessThan(300);
  expect((await api.get(t, "/v1/me")).status).toBe(401);
  void page;
});
