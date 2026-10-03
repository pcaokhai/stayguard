"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import type { components } from "../../api/generated/schema";
import { Pulse } from "@/components/motion";
import { localized, lp } from "../../lib/locale";
import { t, type MessageKey } from "../../lib/t";
import { cn } from "@/lib/utils";
import { formatElapsed, STATUS } from "./status";

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
  return localized(room.unitType.name);
}

// Phone and tablet portrait open a page; with a side panel (desktop) the tile selects the room.
export function hrefFor(room: Room): string | null {
  if (room.status === "VACANT") return lp(`/checkin?room=${room.id}`);
  if (room.activeStay?.pendingPayment) return lp(`/checkout?stay=${room.activeStay.id}`); // checked out, not settled: resume
  if ((room.status === "OCCUPIED" || room.status === "OVERDUE") && room.activeStay)
    return lp(`/stay?id=${room.activeStay.id}`);
  if (room.status === "TO_CLEAN") return lp(`/clean?room=${room.id}`);
  return null;
}

export function RoomTile({
  room,
  readOnly,
  selected,
  onSelect,
}: {
  room: Room;
  readOnly: boolean;
  selected?: boolean;
  onSelect?: (room: Room) => void;
}) {
  const s = STATUS[room.status];
  const href = readOnly ? null : hrefFor(room);
  // A status change cross-fades the colour (CSS) and pulses the tile once.
  const prev = useRef(room.status);
  const [pulse, setPulse] = useState(0);
  useEffect(() => {
    if (prev.current !== room.status) {
      prev.current = room.status;
      setPulse((n) => n + 1);
    }
  }, [room.status]);

  const cls = cn(
    "flex h-[76px] flex-col justify-between rounded-[10px] border p-2.5 text-left transition-colors duration-200",
    s.box,
    readOnly && "opacity-60",
    selected && "ring-2 ring-primary ring-offset-1 ring-offset-background",
  );
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
  return (
    <Pulse trigger={pulse}>
      {onSelect ? (
        <button
          type="button"
          onClick={() => onSelect(room)}
          aria-pressed={selected}
          className={cn(cls, "w-full")}
        >
          {body}
        </button>
      ) : href ? (
        <Link href={href} className={cls}>
          {body}
        </Link>
      ) : (
        <div className={cls}>{body}</div>
      )}
    </Pulse>
  );
}
