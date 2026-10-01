"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { t, type MessageKey } from "../../lib/t";
import { useMe } from "../session/useMe";
import { useBuildings, useRooms } from "./hooks";
import { RoomTile } from "./RoomTile";
import { COUNTER_ORDER, STATUS } from "./status";

const clock = () =>
  new Date().toLocaleString("vi-VN", {
    hour: "2-digit",
    minute: "2-digit",
    day: "2-digit",
    month: "2-digit",
  });

export function RoomMap() {
  const b = useSearchParams().get("b");
  const me = useMe();
  const buildings = useBuildings();
  const current = buildings.data?.find((x) => x.id === b) ?? buildings.data?.[0];
  const rooms = useRooms(current?.id);
  const readOnly = current?.level === "VIEW";

  if (buildings.isError || rooms.isError)
    return (
      <p role="alert" className="p-5">
        {t("rooms.loadFailed")}
      </p>
    );

  return (
    <main className="mx-auto flex max-w-[1280px] flex-col pb-8">
      <header className="flex flex-col gap-1 px-5 pb-3 pt-5 md:flex-row md:items-baseline md:gap-4">
        <h1 className="text-xl font-bold">{me.data?.tenant.name}</h1>
        <p className="text-[13px] text-muted">
          {me.data && t(`rooms.role${me.data.user.role}` as MessageKey)} · {clock()}
        </p>
      </header>
      <nav aria-label="Buildings" className="flex gap-2 px-5 pb-3 md:max-w-md">
        {buildings.data?.map((x) => (
          <Link
            key={x.id}
            href={`/vi/rooms?b=${x.id}`}
            aria-current={x.id === current?.id}
            className={`flex h-11 flex-1 items-center justify-center whitespace-nowrap rounded-[10px] text-[15px] font-semibold ${
              x.id === current?.id ? "bg-brand text-white" : "border border-line bg-surface"
            }`}
          >
            {x.name} ({Object.values(x.counts).reduce((a, n) => a + n, 0)})
          </Link>
        ))}
      </nav>
      {current && (
        <ul className="flex flex-wrap gap-1.5 px-5 pb-3.5">
          {COUNTER_ORDER.map(([status, key]) => (
            <li
              key={status}
              className={`rounded-full border px-2.5 py-1.5 text-xs font-semibold ${STATUS[status].pill}`}
            >
              {t(STATUS[status].label)} {current.counts[key]}
            </li>
          ))}
        </ul>
      )}
      {readOnly && (
        <p className="mx-5 mb-3 rounded-[10px] bg-sunken p-3 text-[13px] text-ink-2">
          {t("rooms.viewOnlyNote")}
        </p>
      )}
      <ul className="grid grid-cols-3 gap-2 px-5 md:grid-cols-6">
        {rooms.data?.map((r) => (
          <li key={r.id}>
            <RoomTile room={r} readOnly={readOnly} />
          </li>
        ))}
      </ul>
      <p className="mx-5 mt-4 text-[13px] text-muted">{t("rooms.hint")}</p>
    </main>
  );
}
