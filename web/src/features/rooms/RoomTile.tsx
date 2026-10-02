import Link from "next/link";
import type { components } from "../../api/generated/schema";
import { t, type MessageKey } from "../../lib/t";
import { formatElapsed, STATUS } from "./status";
import { lp } from "../../lib/locale";

type Room = components["schemas"]["Room"];

const RENTAL: Record<string, MessageKey> = {
  HOURLY: "rooms.hourly",
  OVERNIGHT: "rooms.overnight",
  DAILY: "rooms.daily",
};

function subline(room: Room): string {
  const stay = room.activeStay;
  if (stay)
    return `${t(RENTAL[stay.rentalType])} · ${formatElapsed(stay.elapsedMinutes, t("rooms.hourUnit"))}`;
  if (room.status === "TO_CLEAN") return t("rooms.justLeft");
  if (room.status === "MAINTENANCE") return room.note ?? "";
  return room.unitType.name.vi;
}

function hrefFor(room: Room): string | null {
  if (room.status === "VACANT") return lp(`/checkin?room=${room.id}`);
  if ((room.status === "OCCUPIED" || room.status === "OVERDUE") && room.activeStay)
    return lp(`/stay?id=${room.activeStay.id}`);
  return null;
}

export function RoomTile({ room, readOnly }: { room: Room; readOnly: boolean }) {
  const s = STATUS[room.status];
  const href = readOnly ? null : hrefFor(room);
  const cls = `flex h-[76px] flex-col justify-between rounded-[10px] border p-2.5 ${s.box} ${readOnly ? "opacity-60" : ""}`;
  const body = (
    <>
      <span className="flex items-center justify-between text-base font-bold">
        {room.code}
        {readOnly && <span className="text-xs font-semibold">{t("rooms.viewOnly")}</span>}
      </span>
      <span className="text-xs font-semibold">{t(s.label)}</span>
      <span className="truncate text-xs">{subline(room)}</span>
    </>
  );
  return href ? (
    <Link href={href} className={cls}>
      {body}
    </Link>
  ) : (
    <div className={cls}>{body}</div>
  );
}
