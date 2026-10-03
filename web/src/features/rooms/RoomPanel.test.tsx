import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, test, vi } from "vitest";
import vi_ from "../../../messages/vi.json";
import en from "../../../messages/en.json";
import type { components } from "../../api/generated/schema";
import { RoomPanel } from "./RoomPanel";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: () => {} }) }));

type Room = components["schemas"]["Room"];
const room = (status: Room["status"], extra: Partial<Room> = {}): Room => ({
  id: "r1",
  code: "A103",
  buildingId: "b1",
  floor: 1,
  unitType: { code: "STD", name: { vi: "Phòng thường", en: "Standard" } },
  status,
  ...extra,
});
const render = (r: Room) =>
  renderToStaticMarkup(
    <QueryClientProvider client={new QueryClient()}>
      <RoomPanel room={r} readOnly={false} />
    </QueryClientProvider>,
  );

test("occupied_room_without_active_stay_never_shows_maintenance_text_FU", () => {
  for (const status of ["OCCUPIED", "OVERDUE"] as const) {
    const html = render(room(status));
    for (const text of [vi_.rooms.maintenanceSub, en.rooms.maintenanceSub])
      expect(html).not.toContain(text);
    expect(html).toContain(vi_.rooms.panelLoadFailed); // the test runs in the default (vi) locale
  }
});

test("maintenance_text_only_for_maintenance_rooms_FU", () => {
  expect(render(room("MAINTENANCE"))).toContain(vi_.rooms.maintenanceSub);
  expect(render(room("VACANT"))).toContain(vi_.rooms.vacantSub);
  // To clean reuses the /clean body: checklist and the three actions
  const clean = render(room("TO_CLEAN"));
  for (const k of [vi_.clean.checklist, vi_.clean.done, vi_.clean.maintenance, vi_.clean.damage])
    expect(clean).toContain(k);
});
