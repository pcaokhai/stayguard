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
      revenueTotal: 3460000,
      transfersReceived: 2180000,
      cashExpected: 1280000,
      byBuilding: [
        { buildingId: "A", name: "Tòa A", revenue: 2020000 },
        { buildingId: "B", name: "Tòa B", revenue: 1440000 },
      ],
      occupancy: { occupiedRooms: 10, totalRooms: 35, overdueRooms: 1 },
      alerts: [],
      latestPayments: [
        { paymentId: "p1", roomCode: "A101", method: "TRANSFER", amount: 40000, at: ago(10) },
        { paymentId: "p2", roomCode: "A103", method: "TRANSFER", amount: 300000, at: ago(55) },
        { paymentId: "p3", roomCode: "A302", method: "CASH", amount: 200000, at: ago(120) },
      ],
    }),
  ),
];
