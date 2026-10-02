import { http, HttpResponse } from "msw";

// Owner money, stay-history and shift fixtures (boards GiaoDich*, LichSuLuotOPC, ChiTietLuotO*, DoiSoatCa*).
const json = (body: unknown, status = 200) =>
  HttpResponse.json(body as Record<string, unknown>, { status });
const at = (daysAgo: number, h: number, m: number) => {
  const d = new Date();
  d.setDate(d.getDate() - daysAgo);
  d.setHours(h, m, 0, 0);
  return d.toISOString();
};

const linked = new Set<string>();
const TRANSACTIONS = () => [
  {
    id: "t1",
    at: at(0, 14, 1),
    amount: 30000,
    method: "TRANSFER",
    roomCode: "A101",
    billCode: "PH0930A101",
    reconciliation: "MISMATCH",
    paymentEventId: "e1",
  },
  {
    id: "t2",
    at: at(0, 13, 55),
    amount: 80000,
    method: "TRANSFER",
    roomCode: "A202",
    billCode: "PH0930A202",
    reconciliation: "MATCHED",
    paymentEventId: "e2",
  },
  {
    id: "t3",
    at: at(0, 13, 50),
    amount: 80000,
    method: "CASH",
    roomCode: "B203",
    billCode: "PH0930B203",
    reconciliation: "CASH",
    shiftId: "sh1",
  },
  linked.has("e4")
    ? {
        id: "t4",
        at: at(0, 13, 20),
        amount: 150000,
        method: "TRANSFER",
        roomCode: "A206",
        billCode: "PH0930A206",
        reconciliation: "MATCHED",
        transferNote: "chuyen tien phong",
        paymentEventId: "e4",
      }
    : {
        id: "t4",
        at: at(0, 13, 20),
        amount: 150000,
        method: "TRANSFER",
        reconciliation: "UNMATCHED",
        transferNote: "chuyen tien phong",
        paymentEventId: "e4",
      },
  {
    id: "t5",
    at: at(0, 12, 42),
    amount: 200000,
    method: "TRANSFER",
    roomCode: "A104",
    billCode: "PH0930A104",
    reconciliation: "MATCHED",
    paymentEventId: "e5",
  },
];

const STAYS = [
  {
    id: "s1",
    roomCode: "A101",
    guestName: "Nguyễn Văn An",
    rentalType: "HOURLY",
    checkInAt: at(0, 11, 25),
    checkOutAt: at(0, 14, 0),
    total: 140000,
    paymentMethod: "TRANSFER",
    state: "PAID",
    status: "CHECKED_OUT",
    frontDeskName: "Lễ tân demo",
    guestId: { hasIdNumber: true, hasFrontPhoto: true, hasBackPhoto: true },
  },
  {
    id: "s2",
    roomCode: "A202",
    guestName: "Phạm Hùng",
    rentalType: "HOURLY",
    checkInAt: at(0, 13, 10),
    checkOutAt: at(0, 13, 55),
    total: 80000,
    paymentMethod: "TRANSFER",
    state: "TIME_EDITED",
    status: "CHECKED_OUT",
    frontDeskName: "Lễ tân demo",
    guestId: { hasIdNumber: true, hasFrontPhoto: true, hasBackPhoto: false },
  },
  {
    id: "s3",
    roomCode: "B203",
    guestName: "Trần Thị Bình",
    rentalType: "HOURLY",
    checkInAt: at(0, 12, 40),
    checkOutAt: at(0, 13, 50),
    total: 80000,
    paymentMethod: "CASH",
    state: "PAID",
    status: "CHECKED_OUT",
    frontDeskName: "Chị Hoa",
    guestId: { hasIdNumber: false, hasFrontPhoto: false, hasBackPhoto: false },
  },
  {
    id: "s4",
    roomCode: "A104",
    guestName: "Đỗ Lan",
    rentalType: "OVERNIGHT",
    checkInAt: at(1, 22, 5),
    checkOutAt: at(0, 10, 10),
    total: 200000,
    paymentMethod: "TRANSFER",
    state: "PAID",
    status: "CHECKED_OUT",
    frontDeskName: "Anh Minh",
    guestId: { hasIdNumber: true, hasFrontPhoto: true, hasBackPhoto: true },
  },
  {
    id: "s5",
    roomCode: "A305",
    guestName: "Lê Minh",
    rentalType: "DAILY",
    checkInAt: at(1, 14, 0),
    checkOutAt: null,
    total: 300000,
    paymentMethod: null,
    state: "IN_STAY",
    status: "ACTIVE",
    frontDeskName: "Anh Minh",
    guestId: { hasIdNumber: true, hasFrontPhoto: false, hasBackPhoto: false },
  },
  {
    id: "s6",
    roomCode: "A206",
    guestName: "Hoàng Tú",
    rentalType: "HOURLY",
    checkInAt: at(1, 9, 30),
    checkOutAt: at(1, 10, 35),
    total: 150000,
    paymentMethod: null,
    state: "UNPAID",
    status: "CHECKED_OUT",
    frontDeskName: "Chị Hoa",
    guestId: { hasIdNumber: false, hasFrontPhoto: false, hasBackPhoto: false },
    invoiceId: "inv-a206",
    billCode: "PH0930A206",
  },
  {
    id: "s7",
    roomCode: "B102",
    guestName: "Vũ Hà",
    rentalType: "HOURLY",
    checkInAt: at(0, 12, 0),
    checkOutAt: at(0, 13, 5),
    total: 150000,
    paymentMethod: null,
    state: "UNPAID",
    status: "CHECKED_OUT",
    frontDeskName: "Chị Hoa",
    guestId: { hasIdNumber: false, hasFrontPhoto: false, hasBackPhoto: false },
    invoiceId: "inv-b102",
    billCode: "PH0930B102",
  },
];
const day = (iso: string) => iso.slice(0, 10);

const SHIFTS = [
  {
    id: "sh1",
    userName: "Chị Hoa",
    shift: "MORNING",
    openedAt: at(0, 6, 0),
    closedAt: at(0, 14, 8),
    difference: -50000,
  },
  {
    id: "sh2",
    userName: "Anh Minh",
    shift: "NIGHT",
    openedAt: at(2, 22, 0),
    closedAt: at(1, 6, 0),
    difference: 0,
  },
  {
    id: "sh3",
    userName: "Lễ tân demo",
    shift: "AFTERNOON",
    openedAt: at(1, 14, 0),
    closedAt: at(1, 22, 0),
    difference: 20000,
  },
  {
    id: "sh4",
    userName: "Chị Hoa",
    shift: "MORNING",
    openedAt: at(1, 6, 0),
    closedAt: at(1, 14, 0),
    difference: 0,
  },
  {
    id: "sh5",
    userName: "Anh Minh",
    shift: "NIGHT",
    openedAt: at(3, 22, 0),
    closedAt: at(2, 6, 0),
    difference: 0,
  },
];
const review = (id: string) => {
  const s = SHIFTS.find((x) => x.id === id) ?? SHIFTS[0];
  const expected = 1750000;
  return {
    shift: {
      id: s.id,
      userId: "u2",
      userName: s.userName,
      status: "CLOSED",
      openedAt: s.openedAt,
      closedAt: s.closedAt,
      openingFloat: 500000,
      cashIn: 1280000,
      cashOut: 30000,
      expectedCash: expected,
      transfersReceived: 3120000,
      buildingIds: ["A"],
    },
    countedCash: expected + s.difference,
    difference: s.difference,
    reason: s.difference ? "Trả lại tiền thừa cho khách A104, lúc thu ghi nhầm số tiền." : null,
    reasonAt: s.difference ? s.closedAt : null,
    cashPayments: [
      { roomCode: "A105", rentalType: "DAILY", at: at(0, 7, 10), amount: 300000 },
      { roomCode: "A106", rentalType: "DAILY", at: at(0, 8, 25), amount: 300000 },
      { roomCode: "A302", rentalType: "OVERNIGHT", at: at(0, 12, 5), amount: 200000 },
      { roomCode: "A304", rentalType: "HOURLY", at: at(0, 12, 40), amount: 120000 },
      { roomCode: "A203", rentalType: "HOURLY", at: at(0, 13, 15), amount: 100000 },
      { roomCode: "A206", rentalType: "HOURLY", at: at(0, 13, 50), amount: 260000 },
    ],
    staffHistory: { shiftsWithDifference: 2, totalShort: 80000 },
  };
};

// Staff, one-time PINs and building access (boards NhanVien*, ThemNhanVien*, PinMotLan*, PhanQuyen*).
type MockStaff = {
  id: string;
  name: string;
  phone: string | null;
  position: string;
  appAccess: string;
  username: string | null;
  status: "ACTIVE" | "LOCKED" | "REMOVED";
  lockedUntil: string | null;
  lastActivityAt: string | null;
  contract: Record<string, unknown>;
  buildingAccess: { buildingId: string; level: string }[];
};
const contract = (rate: number) => ({
  payType: "MONTHLY",
  rate,
  fixedAllowance: 600000,
  standardShifts: 26,
  startDate: "2026-01-01",
  annualLeaveDays: 12,
});
const roster: MockStaff[] = [
  {
    id: "u-fd",
    name: "Lễ tân demo",
    phone: null,
    position: "FRONT_DESK",
    appAccess: "RECEPTIONIST",
    username: "frontdesk",
    status: "ACTIVE",
    lockedUntil: null,
    lastActivityAt: at(0, 7, 58),
    contract: contract(5500000),
    buildingAccess: [
      { buildingId: "A", level: "EDIT" },
      { buildingId: "B", level: "VIEW" },
    ],
  },
  {
    id: "u-hoa",
    name: "Chị Hoa",
    phone: null,
    position: "FRONT_DESK",
    appAccess: "RECEPTIONIST",
    username: "hoa",
    status: "ACTIVE",
    lockedUntil: null,
    lastActivityAt: at(0, 13, 2),
    contract: contract(5500000),
    buildingAccess: [
      { buildingId: "A", level: "NONE" },
      { buildingId: "B", level: "EDIT" },
    ],
  },
  {
    id: "u-minh",
    name: "Anh Minh",
    phone: null,
    position: "FRONT_DESK",
    appAccess: "RECEPTIONIST",
    username: "minh",
    status: "LOCKED",
    lockedUntil: at(-1, 11, 35),
    lastActivityAt: at(1, 11, 20),
    contract: contract(5500000),
    buildingAccess: [
      { buildingId: "A", level: "EDIT" },
      { buildingId: "B", level: "EDIT" },
    ],
  },
  {
    id: "u-lan",
    name: "Chị Lan",
    phone: null,
    position: "HOUSEKEEPING",
    appAccess: "HOUSEKEEPING",
    username: "lan",
    status: "ACTIVE",
    lockedUntil: null,
    lastActivityAt: at(0, 8, 30),
    contract: contract(5000000),
    buildingAccess: [
      { buildingId: "A", level: "EDIT" },
      { buildingId: "B", level: "EDIT" },
    ],
  },
  {
    id: "u-mai",
    name: "Chị Mai",
    phone: null,
    position: "MANAGER",
    appAccess: "MANAGER",
    username: "mai",
    status: "ACTIVE",
    lockedUntil: null,
    lastActivityAt: at(0, 9, 10),
    contract: contract(9000000),
    buildingAccess: [
      { buildingId: "A", level: "EDIT" },
      { buildingId: "B", level: "EDIT" },
    ],
  },
  {
    id: "u-tung",
    name: "Anh Tùng",
    phone: "0903456456",
    position: "SECURITY",
    appAccess: "NONE",
    username: null,
    status: "ACTIVE",
    lockedUntil: null,
    lastActivityAt: null,
    contract: contract(5500000),
    buildingAccess: [],
  },
];
const pinExpiry = () => new Date(Date.now() + 24 * 3600_000).toISOString();
const ROLE: Record<string, string> = {
  RECEPTIONIST: "RECEPTIONIST",
  HOUSEKEEPING: "HOUSEKEEPING",
  MANAGER: "MANAGER",
};

export const ownerHandlers = [
  http.get("*/v1/owner/staff", () => json({ items: roster.filter((s) => s.status !== "REMOVED") })),
  http.post("*/v1/owner/staff", async ({ request }) => {
    const b = (await request.json()) as Record<string, unknown> & {
      name: string;
      username?: string | null;
      appAccess: string;
    };
    if (roster.some((s) => s.username && s.username === b.username))
      return json({ code: "CONFLICT" }, 409);
    const staff = {
      id: `u-${roster.length + 1}`,
      phone: null,
      lockedUntil: null,
      lastActivityAt: null,
      status: "ACTIVE",
      buildingAccess: [],
      username: null,
      ...b,
    } as unknown as MockStaff;
    roster.push(staff);
    return json(
      {
        staff,
        oneTimePin: b.appAccess === "NONE" ? null : { pin: "482917", expiresAt: pinExpiry() },
      },
      201,
    );
  }),
  http.patch("*/v1/owner/staff/:id", async ({ params, request }) => {
    const s = roster.find((x) => x.id === params.id);
    if (!s) return json({}, 404);
    Object.assign(s, await request.json());
    return json(s);
  }),
  http.post("*/v1/owner/staff/:id/pin-reset", () =>
    json({ pin: "739104", expiresAt: pinExpiry() }),
  ),
  http.post("*/v1/owner/staff/:id/lock", ({ params }) => {
    const s = roster.find((x) => x.id === params.id);
    if (s) Object.assign(s, { status: "LOCKED", lockedUntil: pinExpiry() });
    return json(s);
  }),
  http.post("*/v1/owner/staff/:id/unlock", ({ params }) => {
    const s = roster.find((x) => x.id === params.id);
    if (s) Object.assign(s, { status: "ACTIVE", lockedUntil: null });
    return json(s);
  }),
  http.post("*/v1/owner/staff/:id/remove", async ({ params, request }) => {
    const b = (await request.json()) as { ownerPin: string };
    if (b.ownerPin !== "482915") return json({ code: "INVALID_PIN" }, 422);
    const s = roster.find((x) => x.id === params.id);
    if (s) s.status = "REMOVED";
    return new HttpResponse(null, { status: 204 });
  }),
  http.get("*/v1/owner/staff-permissions", () =>
    json({
      items: roster
        .filter((s) => s.status !== "REMOVED" && s.appAccess !== "NONE")
        .map((s) => ({
          userId: s.id,
          name: s.name,
          role: ROLE[s.appAccess],
          access: s.buildingAccess,
        })),
    }),
  ),
  http.put("*/v1/owner/staff/:id/building-permissions/:building", async ({ params, request }) => {
    const s = roster.find((x) => x.id === params.id);
    const { level } = (await request.json()) as { level: string };
    if (s) {
      const hit = s.buildingAccess.find((a) => a.buildingId === params.building);
      if (hit) hit.level = level;
      else s.buildingAccess.push({ buildingId: String(params.building), level });
    }
    return json({ buildingId: params.building, level });
  }),
  http.get("*/v1/owner/transactions", ({ request }) => {
    const q = (new URL(request.url).searchParams.get("q") ?? "").toLowerCase();
    return json({
      items: TRANSACTIONS().filter((x) => !q || JSON.stringify(x).toLowerCase().includes(q)),
      nextCursor: null,
    });
  }),
  http.post("*/v1/owner/payment-events/:id/link", ({ params }) => {
    linked.add(String(params.id));
    return json(TRANSACTIONS().find((x) => x.paymentEventId === params.id));
  }),
  http.get("*/v1/stays", ({ request }) => {
    const p = new URL(request.url).searchParams;
    const q = (p.get("q") ?? "").toLowerCase();
    const items = STAYS.filter(
      (s) =>
        (!p.get("from") || day(s.checkInAt) >= p.get("from")!) &&
        (!p.get("to") || day(s.checkInAt) <= p.get("to")!) &&
        (!p.get("state") || s.state === p.get("state")) &&
        (!q || `${s.roomCode} ${s.guestName}`.toLowerCase().includes(q)),
    );
    return json({ items, nextCursor: null });
  }),
  http.get("*/v1/owner/stays/:id/timeline", () =>
    json({
      items: [
        {
          at: at(0, 13, 10),
          kind: "CHECKED_IN",
          actorName: "Lễ tân demo",
          details: { room: "A202", rentalType: "HOURLY" },
        },
        {
          at: at(0, 13, 12),
          kind: "CHECK_IN_EDITED",
          actorName: "Lễ tân demo",
          details: {
            oldTime: at(0, 12, 50),
            newTime: at(0, 13, 10),
            reasonCode: "WRONG_TIME",
            note: "ghi nhầm giờ",
          },
        },
        {
          at: at(0, 13, 30),
          kind: "EXTRAS_ADDED",
          actorName: "Lễ tân demo",
          details: { service: "Nước suối", quantity: "1" },
        },
        {
          at: at(0, 13, 55),
          kind: "CHECKED_OUT",
          actorName: "Hệ thống",
          details: { billCode: "PH0930A202" },
        },
        {
          at: at(0, 13, 56),
          kind: "PAYMENT_RECEIVED",
          actorName: "SePay",
          details: { method: "TRANSFER", amount: "90000" },
        },
        { at: at(0, 14, 20), kind: "CLEANED", actorName: "Chị Lan", details: {} },
      ],
    }),
  ),
  http.get("*/v1/owner/shifts", ({ request }) => {
    const only = new URL(request.url).searchParams.get("onlyDifferences") === "true";
    return json({ items: SHIFTS.filter((s) => !only || s.difference !== 0), nextCursor: null });
  }),
  http.get("*/v1/owner/shifts/:id", ({ params }) => json(review(String(params.id)))),
];
