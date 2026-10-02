import { http, HttpResponse } from "msw";
import { loadSession } from "../../lib/session";

// Deterministic demo data in front of the generated (random) mocks, so screens can be judged
// against the designs. Amounts are fixtures here; the real API computes them (pricing domain).
type Status = "VACANT" | "OCCUPIED" | "OVERDUE" | "TO_CLEAN" | "MAINTENANCE";
const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();
type BuildingId = "A" | "B" | "C" | "D";
// Building A matches the design boards (A101 .. A306); the others are generated from a short pattern.
const A_PLAN: [Status, number?][] = [
  ["OCCUPIED", 155],
  ["VACANT"],
  ["TO_CLEAN"],
  ["OCCUPIED", 300],
  ["VACANT"],
  ["VACANT"],
  ["OVERDUE", 120],
  ["OCCUPIED", 50],
  ["VACANT"],
  ["VACANT"],
  ["MAINTENANCE"],
  ["OCCUPIED", 80],
  ["VACANT"],
  ["TO_CLEAN"],
  ["OCCUPIED", 15],
  ["VACANT"],
  ["OCCUPIED", 400],
  ["VACANT"],
];
const PATTERN: Status[] = [
  "VACANT",
  "OCCUPIED",
  "VACANT",
  "TO_CLEAN",
  "VACANT",
  "OCCUPIED",
  "VACANT",
  "MAINTENANCE",
];
const SIZE: Record<BuildingId, number> = { A: 18, B: 17, C: 12, D: 10 };
const plan = (b: BuildingId): [Status, number?][] =>
  b === "A"
    ? A_PLAN
    : Array.from({ length: SIZE[b] }, (_, i) => [PATTERN[i % PATTERN.length], 45 + i * 7]);
const codeOf = (b: BuildingId, i: number) =>
  `${b}${Math.floor(i / 6) + 1}${String((i % 6) + 1).padStart(2, "0")}`;

const rooms = (b: BuildingId) =>
  plan(b).map(([status, minutes], i) => {
    const code = codeOf(b, i);
    const staying = status === "OCCUPIED" || status === "OVERDUE";
    const vip = i >= 12 && b === "A";
    return {
      id: code,
      code,
      buildingId: b,
      floor: 1 + Math.floor(i / 6),
      unitType: {
        code: vip ? "VIP" : "STD",
        name: { vi: vip ? "VIP" : "Phòng thường", en: vip ? "VIP" : "Standard" },
      },
      status,
      note: status === "MAINTENANCE" ? "Hỏng điều hòa" : null,
      activeStay: staying
        ? {
            id: code === "A101" ? "stay-1" : `stay-${code}`,
            rentalType: (minutes ?? 0) > 240 ? "DAILY" : "HOURLY",
            checkInAt: ago(minutes ?? 60),
            guestName: "Anh Tuấn",
            elapsedMinutes: minutes ?? 60,
            runningTotal: 140000,
          }
        : null,
    };
  });
const counts = (b: BuildingId) => {
  const r = rooms(b);
  const n = (s: Status) => r.filter((x) => x.status === s).length;
  return {
    vacant: n("VACANT"),
    occupied: n("OCCUPIED"),
    overdue: n("OVERDUE"),
    toClean: n("TO_CLEAN"),
    maintenance: n("MAINTENANCE"),
  };
};

const quote = () => ({
  asOf: new Date().toISOString(),
  stayAmount: 120000,
  extrasAmount: 20000,
  total: 140000,
  depositPaid: 100000,
  balanceDue: 40000,
  refundDue: 0,
  capped: false,
  lines: [
    { code: "FIRST_HOUR", quantity: 1, unitAmount: 80000, amount: 80000 },
    { code: "EXTRA_HOUR", quantity: 2, unitAmount: 20000, amount: 40000 },
  ],
});
const stay = () => ({
  id: "stay-1",
  roomId: "A101",
  roomCode: "A101",
  rentalType: "HOURLY",
  status: "ACTIVE",
  checkInAt: ago(155),
  checkOutAt: null,
  guestName: "Anh Tuấn",
  guestPhone: "0901 234 567",
  idNumberMasked: null,
  guestId: { hasIdNumber: true, hasFrontPhoto: true, hasBackPhoto: false },
  deposit: 100000,
  pricingVersion: 1,
  quote: quote(),
  extras: [
    {
      serviceCode: "WATER",
      name: { vi: "Nước suối", en: "Water" },
      quantity: 2,
      unitAmount: 10000,
      amount: 20000,
    },
  ],
});

let paid = false;
const payment = (method: "CASH" | "TRANSFER") => ({
  id: `pay-${method}`,
  invoiceId: "inv-1",
  method,
  amount: 40000,
  status: method === "CASH" || paid ? "PAID" : "PENDING",
  receivedAmount: method === "CASH" || paid ? 40000 : null,
  paidAt: method === "CASH" || paid ? new Date().toISOString() : null,
  transactionId: paid ? "FT26273914652" : null,
  qr:
    method === "TRANSFER"
      ? {
          payload:
            "00020101021238570010A000000727012700069704220113VQRQ0000DEMO0208QRIBFTTA5303704540540000" +
            "5802VN6304ABCD",
          accountNoMasked: "•••• 6789",
          accountName: "NHA NGHI DEMO",
          transferNote: "PH0930A101",
          amount: 40000,
        }
      : null,
});

const json = (body: unknown, status = 200) =>
  HttpResponse.json(body as Record<string, unknown>, { status });

// Sign-in on mocks: guesthouse NNDEMO, users hoa (front desk), chu (owner), bep (housekeeping), all with
// PIN 482915; user moi has the one-time PIN 739104 and must set a new one. Five wrong PINs lock it.
const GOOD_PIN = "482915";
const ONE_TIME_PIN = "739104";
const USERS: Record<string, { name: string; role: string }> = {
  hoa: { name: "Chị Hoa", role: "RECEPTIONIST" },
  chu: { name: "Chủ nhà", role: "OWNER" },
  bep: { name: "Bác Lan", role: "HOUSEKEEPING" },
  moi: { name: "Anh Minh", role: "RECEPTIONIST" },
};
let wrongPins = 0;

// Owner overview, alerts and activity log fixtures (boards TongQuan, CanhBao, NhatKy).
const OVERVIEW_BUILDINGS = [
  ["A", 18, 7, 8, 2, 1, 1, 39, 2020000],
  ["B", 17, 5, 9, 2, 0, 1, 29, 1440000],
  ["C", 12, 4, 7, 1, 1, 0, 33, 860000],
  ["D", 10, 2, 7, 1, 0, 0, 20, 540000],
].map(
  ([
    code,
    totalRooms,
    occupied,
    vacant,
    toClean,
    overdue,
    maintenance,
    occupancyPct,
    revenueToday,
  ]) => ({
    buildingId: code,
    code,
    totalRooms,
    occupied,
    vacant,
    toClean,
    overdue,
    maintenance,
    occupancyPct,
    revenueToday,
  }),
);
const readAlerts = new Set<string>(["al4"]);
const ALERTS = [
  {
    id: "al1",
    kind: "PAYMENT_MISMATCH",
    createdAt: ago(4),
    roomCode: "A101",
    actorName: null,
    amount: 30000,
    stayId: "stay-1",
    details: { billCode: "PH0930A101", expected: "40000", received: "30000" },
  },
  {
    id: "al2",
    kind: "STAY_TIME_EDITED",
    createdAt: ago(53),
    roomCode: "A202",
    actorName: "Lễ tân demo",
    amount: null,
    stayId: "stay-2",
    details: { oldTime: "12:50", newTime: "13:10", reason: "ghi nhầm giờ" },
  },
  {
    id: "al3",
    kind: "CASH_SHORT",
    createdAt: ago(120),
    shiftId: "sh1",
    roomCode: null,
    actorName: "Chị Hoa",
    amount: 50000,
    details: { shiftName: "Ca sáng", reason: "trả lại khách tiền thừa" },
  },
  {
    id: "al4",
    kind: "UNUSED_ROOM_REPORT",
    createdAt: ago(205),
    roomCode: "A205",
    actorName: "Chị Lan",
    amount: null,
    details: { note: "Phòng đang bảo trì nhưng giường có người nằm" },
  },
];
const AUDIT_PAGE = 8;
const localDay = (iso: string) => {
  const d = new Date(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};
const when = (daysAgo: number, h: number, m: number) => {
  const d = new Date();
  d.setDate(d.getDate() - daysAgo);
  d.setHours(h, m, 0, 0);
  return d.toISOString();
};
const AUDIT = [
  {
    id: "au1",
    at: when(0, 14, 5),
    actorName: "Chủ",
    category: "ACCESS_STAFF",
    action: "staff.access_changed",
    details: { staff: "Chị Hoa", building: "Tòa A", from: "Sửa", to: "Không" },
  },
  {
    id: "au2",
    at: when(0, 13, 12),
    actorName: "Lễ tân demo",
    category: "STAY_TIME",
    action: "stay.check_in_edited",
    details: { room: "A202", old: "12:50", new: "13:10" },
  },
  {
    id: "au3",
    at: when(0, 12, 5),
    actorName: "Chị Hoa",
    category: "SHIFT",
    action: "shift.closed",
    details: { difference: "-50000" },
  },
  {
    id: "au4",
    at: when(0, 11, 40),
    actorName: "Chủ",
    category: "ACCESS_STAFF",
    action: "staff.pin_reset",
    details: { staff: "Anh Minh" },
  },
  {
    id: "au5",
    at: when(1, 22, 10),
    actorName: "Anh Minh",
    category: "MONEY",
    action: "transfer.linked",
    details: { amount: "150000", bill: "PH0929B203" },
  },
  {
    id: "au6",
    at: when(1, 17, 30),
    actorName: "Chị Lan",
    category: "STOCK",
    action: "stocktake.created",
    details: { n: "2" },
  },
  {
    id: "au7",
    at: when(1, 9, 15),
    actorName: "Chủ",
    category: "RATES_SETTINGS",
    action: "rate.updated",
    details: { plan: "VIP qua đêm", from: "300000", to: "320000" },
  },
  {
    id: "au8",
    at: when(2, 20, 45),
    actorName: "Lễ tân demo",
    category: "STAY_TIME",
    action: "stay.moved",
    details: { from: "A104", to: "A301" },
  },
  {
    id: "au9",
    at: when(3, 8, 30),
    actorName: "Chị Hoa",
    category: "SHIFT",
    action: "shift.payout",
    details: { amount: "120000" },
  },
];

export const demoHandlers = [
  http.post("*/v1/auth/sign-in", async ({ request }) => {
    const b = (await request.json()) as { guesthouseCode: string; username: string; pin: string };
    if (wrongPins >= 5)
      return HttpResponse.json(
        { type: "about:blank", title: "Locked", status: 429, code: "ACCOUNT_LOCKED" },
        {
          status: 429,
          headers: { "Retry-After": "900", "Content-Type": "application/problem+json" },
        },
      );
    const user = USERS[b.username.toLowerCase()];
    const ok =
      user &&
      b.guesthouseCode.toUpperCase() === "NNDEMO" &&
      b.pin === (b.username === "moi" ? ONE_TIME_PIN : GOOD_PIN);
    if (!ok) {
      wrongPins += 1;
      return HttpResponse.json(
        {
          type: "about:blank",
          title: "Unauthorized",
          status: wrongPins >= 5 ? 429 : 401,
          code: wrongPins >= 5 ? "ACCOUNT_LOCKED" : "INVALID_CREDENTIALS",
        },
        {
          status: wrongPins >= 5 ? 429 : 401,
          headers: { "Retry-After": "900", "Content-Type": "application/problem+json" },
        },
      );
    }
    wrongPins = 0;
    return json(
      {
        accessToken: "demo-token",
        expiresAt: ago(-480),
        tenantId: "t1",
        user: { id: "u1", ...user, locale: "vi" },
        mustChangePin: b.username === "moi",
      },
      200,
    );
  }),
  http.post("*/v1/auth/sign-out", () => new HttpResponse(null, { status: 204 })),
  http.put("*/v1/me/pin", async ({ request }) => {
    const b = (await request.json()) as { currentPin: string };
    return b.currentPin === GOOD_PIN || b.currentPin === ONE_TIME_PIN
      ? new HttpResponse(null, { status: 204 })
      : HttpResponse.json(
          { type: "about:blank", title: "Unauthorized", status: 422, code: "INVALID_CURRENT_PIN" },
          { status: 422, headers: { "Content-Type": "application/problem+json" } },
        );
  }),
  http.put("*/v1/me/locale", () => new HttpResponse(null, { status: 204 })),
  http.get("*/v1/shifts/current", () =>
    json(
      {
        id: "sh1",
        userId: "u1",
        userName: "Chị Hoa",
        status: "OPEN",
        openedAt: ago(70),
        openingFloat: 500000,
        cashIn: 180000,
        cashOut: 0,
        expectedCash: 680000,
        transfersReceived: 220000,
        buildingIds: ["A"],
      },
      200,
    ),
  ),
  http.post("*/v1/demo/sessions", async ({ request }) => {
    const { role } = (await request.json()) as { role: string };
    return json(
      {
        accessToken: "demo-token",
        expiresAt: ago(-480),
        tenantId: "t1",
        user: { id: "u1", name: role === "OWNER" ? "Chủ nhà" : "Chị Lan", role, locale: "vi" },
      },
      201,
    );
  }),
  // The role follows the demo session the picker stored, so every shell can be seen on mocks.
  http.get("*/v1/me", () =>
    json({
      user: loadSession()?.user ?? {
        id: "u1",
        name: "Chị Lan",
        role: "RECEPTIONIST",
        locale: "vi",
      },
      tenant: { id: "t1", name: "Nhà nghỉ Demo", timezone: "Asia/Ho_Chi_Minh", currency: "VND" },
      buildingAccess: [
        { buildingId: "A", level: "NONE" },
        { buildingId: "B", level: "EDIT" },
      ],
    }),
  ),
  http.get("*/v1/buildings", () =>
    json({
      items: (["A", "B", "C", "D"] as const).map((id) => ({
        id,
        code: id,
        name: `Tòa ${id}`,
        level: id === "B" ? "VIEW" : "EDIT",
        counts: counts(id),
      })),
    }),
  ),
  http.get("*/v1/buildings/:id/rooms", ({ params }) =>
    json({ items: rooms(params.id as BuildingId) }),
  ),
  http.get("*/v1/rooms/:id", ({ params }) =>
    json(
      (["A", "B", "C", "D"] as const).flatMap(rooms).find((r) => r.id === params.id) ??
        rooms("A")[0],
    ),
  ),
  http.get("*/v1/services", () =>
    json({
      items: [
        { code: "WATER", name: { vi: "Nước suối", en: "Water" }, price: 10000, stock: 46 },
        { code: "SODA", name: { vi: "Nước ngọt", en: "Soda" }, price: 15000, stock: 30 },
        { code: "BEER", name: { vi: "Bia lon", en: "Beer" }, price: 20000, stock: 24 },
      ],
    }),
  ),
  http.post("*/v1/rooms/:id/stays", () => json(stay(), 201)),
  http.get("*/v1/stays/:id", () => json(stay())),
  http.post("*/v1/stays/:id/extras", () => json(stay())),
  http.post("*/v1/stays/:id/checkout", () =>
    json(
      {
        id: "inv-1",
        stayId: "stay-1",
        roomCode: "A101",
        billCode: "PH0930A101",
        status: "OPEN",
        createdAt: ago(0),
        quote: quote(),
      },
      201,
    ),
  ),
  http.post("*/v1/invoices/:id/payments", async ({ request }) =>
    json(payment(((await request.json()) as { method: "CASH" | "TRANSFER" }).method), 201),
  ),
  http.get("*/v1/payments/:id", ({ params }) =>
    json(payment(params.id === "pay-CASH" ? "CASH" : "TRANSFER")),
  ),
  http.post("*/v1/demo/payments/:id/simulate", () => {
    paid = true;
    return json({});
  }),
  http.get("*/v1/housekeeping/tasks", () =>
    json({
      items: [
        {
          id: "h1",
          roomId: "A103",
          roomCode: "A103",
          buildingId: "A",
          status: "OPEN",
          createdAt: ago(60),
        },
        {
          id: "h2",
          roomId: "B104",
          roomCode: "B104",
          buildingId: "B",
          status: "OPEN",
          createdAt: ago(130),
        },
      ],
    }),
  ),
  http.post("*/v1/housekeeping/tasks/:id/complete", () => json({})),
  http.get("*/v1/owner/overview", () =>
    json({
      date: new Date().toISOString().slice(0, 10),
      revenueTotal: 4860000,
      transfersReceived: 3120000,
      cashExpected: 1740000,
      byBuilding: OVERVIEW_BUILDINGS.map((b) => ({
        buildingId: b.buildingId,
        name: `Tòa ${b.code}`,
        revenue: b.revenueToday,
      })),
      occupancy: { occupiedRooms: 18, totalRooms: 57, overdueRooms: 2 },
      alerts: ALERTS.filter((a) => !readAlerts.has(a.id)),
      latestPayments: [
        { paymentId: "p1", roomCode: "A101", method: "TRANSFER", amount: 30000, at: ago(4) },
        { paymentId: "p2", roomCode: "A202", method: "TRANSFER", amount: 80000, at: ago(10) },
        { paymentId: "p3", roomCode: "B203", method: "CASH", amount: 80000, at: ago(15) },
        { paymentId: "p4", roomCode: null, method: "TRANSFER", amount: 150000, at: ago(45) },
        { paymentId: "p5", roomCode: "A104", method: "TRANSFER", amount: 200000, at: ago(83) },
      ],
      buildings: OVERVIEW_BUILDINGS,
      attention: [
        { kind: "OVERDUE_ROOM", ref: "A201", roomCode: "A201", minutes: 120, amount: null },
        { kind: "OVERDUE_ROOM", ref: "C105", roomCode: "C105", minutes: 40, amount: null },
        { kind: "PAYMENT_MISMATCH", ref: "al1", roomCode: "A101", minutes: null, amount: 30000 },
        { kind: "LONG_TO_CLEAN", ref: "A103", roomCode: "A103", minutes: 55, amount: null },
        { kind: "CASH_SHORT", ref: "al3", roomCode: null, minutes: null, amount: 50000 },
      ],
    }),
  ),
  http.get("*/v1/owner/alerts", ({ request }) => {
    const unread = new URL(request.url).searchParams.get("unread") === "true";
    return json({
      items: ALERTS.filter((a) => !unread || !readAlerts.has(a.id)),
      nextCursor: null,
    });
  }),
  http.post("*/v1/owner/alerts/:id/read", ({ params }) => {
    readAlerts.add(String(params.id));
    return new HttpResponse(null, { status: 204 });
  }),
  http.get("*/v1/owner/audit-logs", ({ request }) => {
    const q = new URL(request.url).searchParams;
    const from = q.get("from") ?? "";
    const to = q.get("to") ?? "";
    const text = (q.get("q") ?? "").toLowerCase();
    const rows = AUDIT.filter(
      (e) =>
        localDay(e.at) >= from &&
        localDay(e.at) <= to &&
        (!q.get("category") || e.category === q.get("category")) &&
        (!text || JSON.stringify(e).toLowerCase().includes(text)),
    );
    const start = q.get("cursor") ? Number(q.get("cursor")) : 0;
    return json({
      items: rows.slice(start, start + AUDIT_PAGE),
      nextCursor: start + AUDIT_PAGE < rows.length ? String(start + AUDIT_PAGE) : null,
    });
  }),
];
