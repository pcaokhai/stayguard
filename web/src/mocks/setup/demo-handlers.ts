import { http, HttpResponse } from "msw";
import { loadSession } from "../../lib/session";

// Deterministic demo data in front of the generated (random) mocks, so screens can be judged
// against the designs. Amounts are fixtures here; the real API computes them (pricing domain).
type Status = "VACANT" | "OCCUPIED" | "OVERDUE" | "TO_CLEAN" | "MAINTENANCE";
const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();
const PLAN = [
  "OCCUPIED",
  "VACANT",
  "TO_CLEAN",
  "OCCUPIED",
  "VACANT",
  "VACANT",
  "OVERDUE",
  "OCCUPIED",
  "VACANT",
  "VACANT",
  "MAINTENANCE",
  "OCCUPIED",
] as Status[];

const rooms = (b: "A" | "B") =>
  PLAN.map((status, i) => ({
    id: `${b}${101 + i}`,
    code: `${b}${101 + i}`,
    buildingId: b,
    floor: 1 + Math.floor(i / 6),
    unitType: {
      code: i > 8 ? "VIP" : "STD",
      name: { vi: i > 8 ? "VIP" : "Phòng thường", en: i > 8 ? "VIP" : "Standard" },
    },
    status,
    note: status === "MAINTENANCE" ? "Hỏng điều hòa" : null,
    activeStay:
      status === "OCCUPIED" || status === "OVERDUE"
        ? {
            id: "stay-1",
            rentalType: "HOURLY",
            checkInAt: ago(155),
            guestName: "Anh Tuấn",
            elapsedMinutes: 155,
            runningTotal: 140000,
          }
        : null,
  }));
const counts = (b: "A" | "B") => {
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

export const demoHandlers = [
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
      buildingAccess: [],
    }),
  ),
  http.get("*/v1/buildings", () =>
    json({
      items: [
        { id: "A", code: "A", name: "Tòa A", level: "EDIT", counts: counts("A") },
        { id: "B", code: "B", name: "Tòa B", level: "VIEW", counts: counts("B") },
      ],
    }),
  ),
  http.get("*/v1/buildings/:id/rooms", ({ params }) =>
    json({ items: rooms(params.id as "A" | "B") }),
  ),
  http.get("*/v1/rooms/:id", ({ params }) =>
    json(rooms("A").find((r) => r.id === params.id) ?? rooms("A")[0]),
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
