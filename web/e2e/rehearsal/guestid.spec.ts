import { expect, stack, test, uiLogin } from "./helpers";

// Guest ID (docs/15 rules 21 to 25): the front desk can write it and sees indicators only; the owner sees it masked and every reveal is logged.
// A number made for this run, so a search of the logs cannot match anything else.
const ID_NUMBER = `079${String(Math.floor(Math.random() * 1e9)).padStart(9, "0")}`;

test("GI-01 front desk and housekeeping see indicators only, never the number or the photo", async ({
  api,
  page,
}) => {
  const w = await api.as("r1");
  const hk = await api.as("hk");
  const stay = await api.checkIn(w, { idNumber: ID_NUMBER });
  expect((await api.uploadIdPhoto(w, stay.id, "FRONT")).status).toBe(200);
  const seen = await api.getStay(w, stay.id);
  expect(seen.guestId).toEqual({ hasIdNumber: true, hasFrontPhoto: true, hasBackPhoto: false });
  expect(JSON.stringify(seen), "no number in the stay the front desk reads").not.toContain(
    ID_NUMBER,
  );
  for (const who of [w, hk]) {
    expect(
      (await api.get(who, `/v1/owner/stays/${stay.id}/guest-id`)).status,
      `${who.user} reads the record`,
    ).toBe(403);
    expect(
      (await api.get(who, `/v1/owner/stays/${stay.id}/guest-id/photos/FRONT`)).status,
      `${who.user} reads the photo`,
    ).toBe(403);
    expect(
      (await api.post(who, `/v1/owner/stays/${stay.id}/guest-id/reveal`)).status,
      `${who.user} reveals`,
    ).toBe(403);
  }
  await uiLogin(page, w);
  await page.goto(`/en/stay?id=${stay.id}`);
  await expect(page.getByText("ID number on file")).toBeVisible();
  await expect(page.getByText("Front photo on file")).toBeVisible();
  await expect(page.getByText("No back photo")).toBeVisible();
  await expect(page.locator("body")).not.toContainText(ID_NUMBER);
});

test("GI-02 the owner sees a masked number, and a reveal is written to the activity log without the number", async ({
  api,
}) => {
  const w = await api.as("r1");
  const owner = await api.as("owner");
  const stay = await api.checkIn(w, { idNumber: ID_NUMBER });
  await api.uploadIdPhoto(w, stay.id, "FRONT");
  const rec = await api.get(owner, `/v1/owner/stays/${stay.id}/guest-id`);
  expect(rec.status).toBe(200);
  expect(rec.body.idNumberMasked, "first 3 and last 3 digits").toBe(
    `${ID_NUMBER.slice(0, 3)}${"*".repeat(ID_NUMBER.length - 6)}${ID_NUMBER.slice(-3)}`,
  );
  expect(JSON.stringify(rec.body)).not.toContain(ID_NUMBER);
  const reveal = await api.post(owner, `/v1/owner/stays/${stay.id}/guest-id/reveal`);
  expect(reveal.status).toBe(200);
  expect(reveal.body.idNumber).toBe(ID_NUMBER);
  expect(reveal.headers["cache-control"]).toContain("no-store");
  const all = await api.audit(owner);
  const line = all.find((e) => e.action === "GUEST_ID_REVEALED" && e.actorName === "Smoke Owner");
  expect(line, "the reveal is in the activity log").toBeTruthy();
  expect(JSON.stringify(all), "the log never holds the number").not.toContain(ID_NUMBER);
  // Rule 23: a GUEST_ID audit entry, so the log can be filtered by it.
  expect.soft(line.category, "the reveal is filed under GUEST_ID").toBe("GUEST_ID");
  expect
    .soft((await api.audit(owner, "&category=GUEST_ID")).length, "the GUEST_ID filter finds it")
    .toBeGreaterThan(0);
  const photo = await api.request.get(
    `${(await import("./helpers")).cfg.base}/v1/owner/stays/${stay.id}/guest-id/photos/FRONT`,
    { headers: { authorization: `Bearer ${owner.token}` } },
  );
  expect(photo.status()).toBe(200);
  expect(photo.headers()["cache-control"]).toContain("no-store");
});

test("GI-03 no ID number appears in the container logs", async ({ api }) => {
  const w = await api.as("r1");
  const owner = await api.as("owner");
  const stay = await api.checkIn(w, { idNumber: ID_NUMBER });
  await api.post(owner, `/v1/owner/stays/${stay.id}/guest-id/reveal`);
  for (const service of ["api", "jobs", "db"]) {
    expect(stack.logs(service), `${service} logs`).not.toContain(ID_NUMBER);
  }
});
