import { expect, test, uiLogin } from "./helpers";

// Cleaning, damage and maintenance (docs/15 rules 6 to 8).
test("RM-01 a receptionist marks a room clean and it is vacant again", async ({ api }) => {
  const w = await api.as("linh");
  const { room } = await api.toClean(w);
  const task = await api.cleanTask(w, room.id);
  expect(task, "an open cleaning task for the room").toBeTruthy();
  const done = await api.post(w, `/v1/housekeeping/tasks/${task.id}/complete`);
  expect(done.status, JSON.stringify(done.body)).toBeLessThan(300);
  expect((await api.room(w, room.code)).status).toBe("VACANT");
});

test("RM-02 the owner marks a room clean and it is vacant again", async ({ api }) => {
  const w = await api.as("linh");
  const owner = await api.as("owner");
  const { room } = await api.toClean(w);
  const task = await api.cleanTask(owner, room.id);
  expect(task).toBeTruthy();
  const done = await api.post(owner, `/v1/housekeeping/tasks/${task.id}/complete`);
  expect(done.status, JSON.stringify(done.body)).toBeLessThan(300);
  expect((await api.room(owner, room.code)).status).toBe("VACANT");
});

test("RM-03 the to-clean screens offer clean, set maintenance and report damage", async ({
  api,
  page,
}, testInfo) => {
  const w = await api.as("linh");
  const owner = await api.as("owner");
  const { room } = await api.toClean(w);
  await uiLogin(page, w);
  await page.goto(`/en/clean?room=${room.id}`);
  await expect(page.getByRole("button", { name: /Cleaned/ })).toBeVisible();
  await expect(
    page
      .getByRole("button", { name: "Set maintenance" })
      .or(page.getByRole("link", { name: "Set maintenance" })),
  ).toBeVisible();
  await expect(
    page
      .getByRole("button", { name: "Report damage" })
      .or(page.getByRole("link", { name: "Report damage" })),
  ).toBeVisible();
  // The owner's desktop room map opens the same room in a side panel.
  const ownerPage = await page
    .context()
    .browser()!
    .newPage({ viewport: { width: 1280, height: 900 } });
  await uiLogin(ownerPage, owner);
  await ownerPage.goto("/en/owner/rooms");
  await ownerPage.locator("li", { hasText: room.code }).first().getByRole("button").click();
  const panel = ownerPage.locator("aside");
  await expect(panel).toContainText("To clean");
  for (const name of [/Cleaned/, "Set maintenance", "Report damage"])
    await expect(
      panel.getByRole("button", { name }).or(panel.getByRole("link", { name })),
    ).toBeVisible();
  await testInfo.attach("owner-panel", {
    body: await ownerPage.screenshot(),
    contentType: "image/png",
  });
  await ownerPage.close();
});

test("RM-04 a damage report that locks the room makes it maintenance, with a ticket and an alert", async ({
  api,
}) => {
  const w = await api.as("linh");
  const owner = await api.as("owner");
  const room = await api.vacantRoom(w);
  const rep = await api.post(w, `/v1/rooms/${room.id}/damage-reports`, {
    category: "AIR_CONDITIONER",
    description: "Rehearsal: the A/C leaks",
    severity: "LOCK_ROOM",
  });
  expect(rep.status, JSON.stringify(rep.body)).toBeLessThan(300);
  expect((await api.room(w, room.code)).status).toBe("MAINTENANCE");
  const tickets: any[] = (await api.get(owner, "/v1/owner/maintenance-tickets")).body.items;
  expect(tickets.some((t) => t.roomCode === room.code && t.roomLocked)).toBe(true);
  expect(
    (await api.alerts(owner)).some((a) => a.kind === "DAMAGE_REPORTED" && a.roomCode === room.code),
  ).toBe(true);
  // A room with a guest cannot be locked.
  const busy = await api.checkIn(w);
  const locked = await api.post(w, `/v1/rooms/${busy.roomId}/damage-reports`, {
    category: "TV",
    description: "Rehearsal: no signal",
    severity: "LOCK_ROOM",
  });
  expect(locked.status, "not while a guest is in the room").toBe(409);
});

test("RM-05 a done ticket with a cost becomes a maintenance expense and reopens the room", async ({
  api,
}) => {
  const w = await api.as("linh");
  const owner = await api.as("owner");
  const room = await api.vacantRoom(w);
  await api.post(w, `/v1/rooms/${room.id}/damage-reports`, {
    category: "PLUMBING",
    description: "Rehearsal: tap leaks",
    severity: "LOCK_ROOM",
  });
  const ticket = ((await api.get(owner, "/v1/owner/maintenance-tickets")).body.items as any[]).find(
    (t) => t.roomCode === room.code,
  );
  expect(ticket).toBeTruthy();
  const month = new Date().toISOString().slice(0, 7);
  const before = (
    (await api.get(owner, `/v1/owner/expenses?month=${month}`)).body.items as any[]
  ).filter((e) => e.category === "MAINTENANCE");
  const done = await api.patch(owner, `/v1/owner/maintenance-tickets/${ticket.id}`, {
    status: "DONE",
    partsCost: 120_000,
    labourCost: 80_000,
    repairer: "Rehearsal Plumbing",
  });
  expect(done.status, JSON.stringify(done.body)).toBeLessThan(300);
  const after = (
    (await api.get(owner, `/v1/owner/expenses?month=${month}`)).body.items as any[]
  ).filter((e) => e.category === "MAINTENANCE");
  expect(after.length, "one more maintenance expense").toBe(before.length + 1);
  const line = after.find((e) => !before.some((b) => b.id === e.id));
  expect(line.amount).toBe(200_000);
  expect(line.source).toBe("MAINTENANCE");
  expect((await api.room(w, room.code)).status, "the room reopens").toBe("VACANT");
});
