import { expect, test } from "./helpers";

// Portfolio-demo checks, API level: stays (LO), setup (CD) and the owner's monitoring (TD). Money is always what the API answers.
const iso = (offsetMin: number, from = Date.now()) =>
  new Date(from + offsetMin * 60_000).toISOString();

test("LO-01 check-in without ID papers works and takes the time from the server", async ({
  api,
}) => {
  const w = await api.as("r13");
  const before = Date.now();
  const stay = await api.checkIn(w);
  expect(Math.abs(Date.parse(stay.checkInAt) - before), "the server clock").toBeLessThan(15_000);
  expect(stay.guestId).toEqual({ hasIdNumber: false, hasFrontPhoto: false, hasBackPhoto: false });
  expect((await api.room(w, stay.roomCode)).status).toBe("OCCUPIED");
});

test("LO-02 extras update the total and the stock", async ({ api }) => {
  const w = await api.as("r13");
  const owner = await api.as("owner");
  const stock = async () =>
    (await api.get(w, "/v1/services")).body.items.find((s: any) => s.code === "WATER").stock;
  const before = await stock();
  const stay = await api.checkIn(w);
  await api.addWater(w, stay.id, 2);
  const now = await api.getStay(w, stay.id);
  expect(now.extras.length).toBeGreaterThan(0);
  expect(now.quote.extrasAmount, "2 bottles at the shelf price").toBe(2 * 10_000);
  expect(now.quote.total).toBe(now.quote.stayAmount + now.quote.extrasAmount);
  expect(await stock(), "stock went down by 2").toBe(before - 2);
});

test("LO-03 check-in time can be corrected within limits, with a reason, and the owner is told", async ({
  api,
}) => {
  const w = await api.as("r13");
  const owner = await api.as("owner");
  const stay = await api.checkIn(w);
  const at = Date.parse(stay.checkInAt);
  const edit = (t: string, note = "Guest arrived earlier than recorded") =>
    api.post(w, `/v1/stays/${stay.id}/check-in-time`, {
      newCheckInAt: t,
      reasonCode: "WRONG_TIME",
      note,
    });
  const ok = await edit(iso(-20, at));
  expect(ok.status, JSON.stringify(ok.body)).toBeLessThan(300);
  const fresh = await api.getStay(w, stay.id);
  expect(Date.parse(fresh.checkInAt)).toBe(Date.parse(iso(-20, at)));
  expect(
    (await api.alerts(owner)).some((a) => a.kind === "STAY_TIME_EDITED" && a.stayId === stay.id),
  ).toBe(true);
  expect(
    (await edit(iso(+90, at))).status,
    "more than 60 minutes later than recorded",
  ).toBeGreaterThanOrEqual(400);
  expect((await edit(iso(+10_000, at))).status, "in the future").toBeGreaterThanOrEqual(400);
});

test("LO-04 there is no way to edit the check-out time", async ({ api }) => {
  const w = await api.as("r13");
  const stay = await api.checkIn(w);
  const inv = await api.checkout(w, stay.id);
  const t = iso(-5);
  const tries = [
    await api.post(w, `/v1/stays/${stay.id}/check-in-time`, {
      newCheckInAt: t,
      reasonCode: "OTHER",
      note: "x y z",
      checkOutAt: t,
    }),
    await api.patch(w, `/v1/stays/${stay.id}`, { checkOutAt: t }),
    await api.put(w, `/v1/stays/${stay.id}`, { checkOutAt: t }),
  ];
  for (const r of tries) expect(r.status).toBeGreaterThanOrEqual(400);
  expect((await api.getStay(w, stay.id)).checkOutAt, "unchanged").toBe(inv.createdAt);
});

test("LO-05 moving a guest keeps the time and extras, prices by the new room, and the old room needs cleaning", async ({
  api,
}) => {
  const w = await api.as("r13");
  const stay = await api.checkIn(w);
  await api.addWater(w, stay.id, 1);
  const target = await api.vacantRoom(w);
  const moved = await api.post(w, `/v1/stays/${stay.id}/move`, {
    toRoomId: target.id,
    rentalType: "HOURLY",
  });
  expect(moved.status, JSON.stringify(moved.body)).toBeLessThan(300);
  const now = await api.getStay(w, stay.id);
  expect(now.roomId).toBe(target.id);
  expect(now.checkInAt, "check-in time kept").toBe(stay.checkInAt);
  expect(now.extras.length, "extras kept").toBeGreaterThan(0);
  expect((await api.room(w, stay.roomCode)).status, "old room").toBe("TO_CLEAN");
  const busy = await api.checkIn(w);
  expect(
    (
      await api.post(w, `/v1/stays/${stay.id}/move`, {
        toRoomId: busy.roomId,
        rentalType: "HOURLY",
      })
    ).status,
    "into a room with a guest",
  ).toBe(409);
});

test("LO-07 stay history by day, with ID columns as flags and the day limit for the front desk", async ({
  api,
}) => {
  const w = await api.as("r13");
  const stay = await api.checkIn(w);
  const today = new Date().toISOString().slice(0, 10);
  const list = await api.get(w, `/v1/stays?date=${today}`);
  expect(list.status).toBe(200);
  const item = list.body.items.find((x: any) => x.id === stay.id);
  expect(item?.guestId).toBeTruthy();
  expect(Object.keys(item.guestId).sort()).toEqual([
    "hasBackPhoto",
    "hasFrontPhoto",
    "hasIdNumber",
  ]);
  const old = new Date(Date.now() - 60 * 86_400_000).toISOString().slice(0, 10);
  expect(
    (await api.get(w, `/v1/stays?date=${old}`)).status,
    "too far back for the front desk",
  ).toBe(422);
});

test("LO-08 the owner sees one stay as a timeline", async ({ api }) => {
  const w = await api.as("r13");
  const owner = await api.as("owner");
  const q = await api.toQr(w, { water: 1 });
  await api.pay(q.note, q.amount);
  await api.untilPayment(w, q.payment.id, "PAID");
  const task = await api.cleanTask(w, (await api.getStay(w, q.stay.id)).roomId);
  if (task) await api.post(w, `/v1/housekeeping/tasks/${task.id}/complete`);
  const tl = await api.get(owner, `/v1/owner/stays/${q.stay.id}/timeline`);
  expect(tl.status).toBe(200);
  const kinds = (tl.body.items ?? tl.body).map((e: any) => e.kind);
  for (const k of ["CHECKED_IN", "EXTRAS_ADDED", "CHECKED_OUT", "PAYMENT_RECEIVED", "CLEANED"])
    expect(kinds, k).toContain(k);
  const times = (tl.body.items ?? tl.body).map((e: any) => Date.parse(e.at));
  expect(times, "in time order").toEqual([...times].sort((a, b) => a - b));
  expect(
    (await api.get(w, `/v1/owner/stays/${q.stay.id}/timeline`)).status,
    "not for the front desk",
  ).toBe(403);
});

test("LO-09 history search by room, guest name and phone", async ({ api }) => {
  const w = await api.as("r13");
  const name = `Searchable${Date.now() % 100000}`;
  const stay = await api.checkIn(w, { name });
  const today = new Date().toISOString().slice(0, 10);
  const find = async (q: string) =>
    (
      (await api.get(w, `/v1/stays?date=${today}&q=${encodeURIComponent(q)}`)).body.items as any[]
    ).map((x) => x.id);
  expect(await find(name), "by guest name").toContain(stay.id);
  expect(await find(stay.roomCode), "by room").toContain(stay.id);
  expect(await find("0912345678"), "by phone").toContain(stay.id);
  expect(await find("nobody-has-this-name")).not.toContain(stay.id);
});

test("LO-10 the guest phone takes 9 to 11 digits with an optional 0 or +84", async ({ api }) => {
  const w = await api.as("r13");
  const tryPhone = async (phone: string) => {
    const room = await api.vacantRoom(w);
    return api.post(w, `/v1/rooms/${room.id}/stays`, {
      rentalType: "HOURLY",
      guestName: "Phone Test",
      guestPhone: phone,
      deposit: 0,
    });
  };
  for (const ok of ["012345678", "0912345678", "09123456789", "+84912345678", "912345678"])
    expect((await tryPhone(ok)).status, ok).toBe(201);
  for (const bad of ["01234567", "091234567890", "09123456789012", "abc"])
    expect((await tryPhone(bad)).status, bad).toBe(422);
});

test("CD-01 bank accounts: new ones wait for SePay, the default cannot be removed", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const list = async () => (await api.get(owner, "/v1/owner/bank-accounts")).body.items as any[];
  const before = await list();
  const def = before.find((a) => a.isDefault ?? a.default);
  expect(def, "a default account").toBeTruthy();
  const made = await api.post(owner, "/v1/owner/bank-accounts", {
    bankBin: "970436",
    accountNo: "2222233333",
    accountName: "SECOND DEMO",
    ownerPin: "482916",
  });
  expect(made.status, JSON.stringify(made.body)).toBeLessThan(300);
  const added = (await list()).find((a) => a.id === made.body.id);
  expect(JSON.stringify(added), "waiting for SePay").toMatch(/PENDING|WAITING/i);
  expect(JSON.stringify(added), "the number is masked").not.toContain("2222233333");
  const mk = await api.post(owner, `/v1/owner/bank-accounts/${made.body.id}/make-default`, {
    ownerPin: "482916",
  });
  expect(mk.status, "cannot be default before SePay is connected").toBeGreaterThanOrEqual(400);
  const rm = await api.post(owner, `/v1/owner/bank-accounts/${def.id}/remove`, {
    ownerPin: "482916",
  });
  expect(rm.status, "the default cannot be removed").toBeGreaterThanOrEqual(400);
  expect(
    (
      await api.post(owner, `/v1/owner/bank-accounts/${made.body.id}/remove`, {
        ownerPin: "000000",
      })
    ).status,
    "owner PIN asked",
  ).toBe(403);
  expect(
    (
      await api.post(owner, `/v1/owner/bank-accounts/${made.body.id}/remove`, {
        ownerPin: "482916",
      })
    ).status,
  ).toBeLessThan(300);
});

test("CD-02 a new price applies to later stays; a guest already in the room keeps the old one", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const w = await api.as("r14");
  const plans = (await api.get(owner, "/v1/owner/rate-plans")).body;
  const std =
    (plans.items ?? plans).find((p: any) => (p.unitTypeCode ?? p.code) === "STANDARD") ??
    (plans.items ?? plans)[0];
  const plan = std.ratePlan ?? std;
  const draft = { ...plan, hourly: { ...plan.hourly, firstHour: plan.hourly.firstHour + 10_000 } };
  delete (draft as any).version;
  const old = await api.checkIn(w, { deposit: 0 });
  const preview = await api.post(owner, "/v1/owner/rate-plans/preview", {
    rentalType: "HOURLY",
    checkIn: iso(-30),
    checkOut: iso(0),
    ratePlan: { ...draft, version: plan.version ?? 1 },
  });
  expect(preview.status, JSON.stringify(preview.body)).toBeLessThan(300);
  expect(JSON.stringify(preview.body)).toContain(String(plan.hourly.firstHour + 10_000));
  const saved = await api.put(owner, "/v1/owner/unit-types/STANDARD/rate-plan", {
    ...draft,
    version: (plan.version ?? 1) + 1,
  });
  expect(saved.status, JSON.stringify(saved.body)).toBeLessThan(300);
  const later = await api.checkIn(w, { deposit: 0 });
  expect((await api.getStay(w, later.id)).quote.stayAmount, "later stay: new price").toBe(
    plan.hourly.firstHour + 10_000,
  );
  expect((await api.getStay(w, old.id)).quote.stayAmount, "guest already in: old price").toBe(
    plan.hourly.firstHour,
  );
  await api.put(owner, "/v1/owner/unit-types/STANDARD/rate-plan", {
    ...plan,
    version: (plan.version ?? 1) + 2,
  }); // back to the old price
});

test("CD-03 rooms by range, floors, buildings; a room with a guest cannot change type or be retired", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const w = await api.as("r14");
  const b = await api.post(owner, "/v1/owner/buildings", {
    code: "Y",
    name: "Building Y",
    floors: 1,
    roomsPerFloor: 0,
    unitTypeCode: "STANDARD",
  });
  expect(b.status, JSON.stringify(b.body)).toBeLessThan(300);
  const floors = await api.post(owner, `/v1/owner/buildings/${b.body.id}/floors`, {
    name: "2",
    rooms: { count: 3, startCode: "Y201", unitTypeCode: "STANDARD" },
  });
  expect(floors.status, JSON.stringify(floors.body)).toBeLessThan(300);
  const floor1 = (await api.get(owner, `/v1/buildings`)).body.items.find((x: any) => x.code === "Y")
    .floors[0];
  const range = await api.post(owner, "/v1/owner/rooms", {
    buildingId: b.body.id,
    floorId: floor1.id,
    fromCode: "Y101",
    toCode: "Y103",
    unitTypeCode: "STANDARD",
    availableNow: true,
  });
  expect(range.status, JSON.stringify(range.body)).toBeLessThan(300);
  const codes = ((await api.get(owner, `/v1/buildings/${b.body.id}/rooms`)).body.items as any[])
    .map((r) => r.code)
    .sort();
  expect(codes).toEqual(["Y101", "Y102", "Y103", "Y201", "Y202", "Y203"]);
  const busy = await api.checkIn(w);
  const room = await api.room(owner, busy.roomCode);
  expect(
    (await api.patch(owner, `/v1/owner/rooms/${room.id}`, { unitTypeCode: "DELUXE" })).status,
    "type of a room with a guest",
  ).toBeGreaterThanOrEqual(400);
  expect(
    (await api.patch(owner, `/v1/owner/rooms/${room.id}`, { retired: true })).status,
    "retire a room with a guest",
  ).toBe(409);
});

test("CD-04 items and stock: opening quantity, restock, a stocktake difference alerts, a sold item stops selling", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const w = await api.as("r14");
  const made = await api.post(owner, "/v1/owner/services", {
    name: { vi: "Kẹo", en: "Candy" },
    price: 5_000,
    unitCost: 2_000,
    openingQuantity: 10,
    unit: "pcs",
    lowStockAt: 3,
  });
  expect(made.status, JSON.stringify(made.body)).toBeLessThan(300);
  const code = made.body.code;
  const item = async () =>
    ((await api.get(w, "/v1/services")).body.items as any[]).find((s) => s.code === code);
  expect((await item()).stock).toBe(10);
  expect(
    (await api.post(owner, `/v1/owner/services/${code}/restock`, { quantity: 5, unitCost: 2_000 }))
      .status,
  ).toBeLessThan(300);
  expect((await item()).stock).toBe(15);
  const moves = (await api.get(owner, `/v1/owner/services/${code}/movements`)).body.items;
  expect(moves.length, "stock changes only through history rows").toBeGreaterThanOrEqual(2);
  expect(
    (await api.post(w, "/v1/stocktakes", { lines: [{ serviceCode: code, counted: 12 }] })).status,
  ).toBeLessThan(300);
  expect((await item()).stock).toBe(12);
  expect((await api.alerts(owner)).some((a) => a.kind === "STOCKTAKE_DIFFERENCE")).toBe(true);
  const stay = await api.checkIn(w);
  await api.post(w, `/v1/stays/${stay.id}/extras`, { items: [{ serviceCode: code, quantity: 1 }] });
  const gone = await api.post(owner, `/v1/owner/services/${code}/remove`);
  expect(gone.body.result, "sold items are never deleted").toBe("STOPPED_SELLING");
});

test("CD-05 a guard gets no sign-in; a receptionist's one-time PIN is shown once", async ({
  api,
}) => {
  const owner = await api.as("owner");
  await api.rooms(owner);
  const contract = {
    payType: "MONTHLY",
    rate: 5_000_000,
    fixedAllowance: 0,
    standardShifts: 26,
    startDate: "2026-01-01",
    annualLeaveDays: 12,
  };
  const guard = await api.post(owner, "/v1/owner/staff", {
    name: "Rehearsal Guard",
    position: "SECURITY",
    appAccess: "NONE",
    contract,
    buildingAccess: [],
  });
  expect(guard.status, JSON.stringify(guard.body)).toBe(201);
  expect(guard.body.oneTimePin ?? null, "no PIN for someone who cannot sign in").toBeNull();
  const rec = await api.newStaff(owner, "cd05desk");
  expect(rec.pin).toMatch(/^\d{6}$/);
  const staff = (await api.get(owner, "/v1/owner/staff")).body.items as any[];
  expect(JSON.stringify(staff.find((s) => s.id === rec.id)), "never shown again").not.toContain(
    rec.pin,
  );
});

test("CD-08 manual and monthly-repeating expenses; automatic lines cannot be edited by hand", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const month = new Date().toISOString().slice(0, 7);
  const manual = await api.post(owner, "/v1/owner/expenses", {
    category: "SUPPLIES",
    amount: 150_000,
    month,
    note: "Rehearsal soap",
  });
  expect(manual.status, JSON.stringify(manual.body)).toBeLessThan(300);
  const rec = await api.post(owner, "/v1/owner/expenses", {
    category: "INTERNET_TV",
    amount: 300_000,
    month,
    recurring: true,
    note: "Rehearsal internet",
  });
  expect(rec.status).toBeLessThan(300);
  const rows = async () =>
    (await api.get(owner, `/v1/owner/expenses?month=${month}`)).body.items as any[];
  expect((await rows()).filter((e) => e.note === "Rehearsal internet")).toHaveLength(1);
  const auto = (await rows()).find((e) => e.source !== "MANUAL" && e.source !== "RECURRING");
  if (auto)
    expect(
      (
        await api.patch(owner, `/v1/owner/expenses/${auto.id}`, {
          category: auto.category,
          amount: 1,
          month,
        })
      ).status,
      "automatic line",
    ).toBeGreaterThanOrEqual(400);
  expect(
    (
      await api.patch(owner, `/v1/owner/expenses/${manual.body.id}`, {
        category: "SUPPLIES",
        amount: 160_000,
        month,
      })
    ).status,
  ).toBeLessThan(300);
});

test("CD-09 the income report adds up: revenue is the paid bills, costs are the expenses", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const month = new Date().toISOString().slice(0, 7);
  const rep = (await api.get(owner, `/v1/owner/reports/income-costs?from=${month}&to=${month}`))
    .body;
  const ov = await api.overview(owner);
  expect(rep.revenue, "same revenue as the overview for a month of one day").toBe(ov.revenueTotal);
  const exp = (await api.get(owner, `/v1/owner/expenses?month=${month}`)).body.items as any[];
  expect(rep.expenses).toBe(exp.reduce((s, e) => s + e.amount, 0));
  expect(rep.profit).toBe(rep.revenue - rep.expenses);
  expect(rep.months.reduce((s: number, m: any) => s + m.revenue, 0)).toBe(rep.revenue);
});

test("TD-01 the overview agrees with itself and with the report", async ({ api }) => {
  const owner = await api.as("owner");
  const ov = await api.overview(owner);
  expect(
    ov.byBuilding.reduce((s: number, b: any) => s + b.revenue, 0),
    "revenue by building adds up",
  ).toBe(ov.revenueTotal);
  const rooms = (await api.rooms(owner)).length;
  expect(ov.buildings.reduce((s: number, b: any) => s + b.totalRooms, 0)).toBeGreaterThanOrEqual(
    rooms,
  );
  const rows = (await api.rooms(owner)).filter((r) => r.status === "OCCUPIED").length;
  expect(
    ov.buildings.find((b: any) => b.code === "A").occupied,
    "occupied matches the room map",
  ).toBe(rows);
});

test("TD-03 alerts can be marked read; money and time alerts stay on the list", async ({ api }) => {
  const owner = await api.as("owner");
  const money = (await api.alerts(owner)).find(
    (a) => a.kind === "UNMATCHED_TRANSFER" || a.kind === "PAYMENT_PARTIAL" || a.kind === "OVERPAID",
  );
  expect(money, "a money alert exists from the earlier cases").toBeTruthy();
  const r = await api.post(owner, `/v1/owner/alerts/${money.id}/read`);
  expect(r.status, JSON.stringify(r.body)).toBeLessThan(300);
  const after = (await api.alerts(owner)).find((a) => a.id === money.id);
  expect(after, "a read alert is still on the list").toBeTruthy();
  expect.soft(JSON.stringify(after), "the API says the alert was read").toMatch(/read/i);
});

test("TD-04 the activity log: important actions are in it and nothing can edit or delete it", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const has = async (q: string) => (await api.audit(owner, `&q=${q}`)).length > 0;
  expect(await has("check_in"), "check-in").toBe(true);
  expect(await has("link"), "linking money").toBe(true);
  expect(await has("GUEST_ID"), "ID reveal").toBe(true);
  const all = await api.audit(owner);
  expect(
    (await api.audit(owner, `&category=${all[0].category}`)).every(
      (e) => e.category === all[0].category,
    ),
  ).toBe(true);
  const who = await api.audit(owner, `&actorId=${all[0].actorId ?? ""}`);
  expect(who.length).toBeGreaterThan(0);
  for (const m of ["put", "patch", "post"] as const)
    expect(
      (await api[m](owner, `/v1/owner/audit-logs/${all[0].id}`, {})).status,
    ).toBeGreaterThanOrEqual(400);
  expect(
    (await api.call(owner, "DELETE", `/v1/owner/audit-logs/${all[0].id}`)).status,
  ).toBeGreaterThanOrEqual(400);
});
