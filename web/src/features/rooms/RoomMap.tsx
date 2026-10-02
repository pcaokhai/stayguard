"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { FadeIn, RollingNumber, SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { QueryError } from "@/components/StateView";
import { Badge } from "@/components/ui/badge";
import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { t, type MessageKey } from "../../lib/t";
import { useMe } from "../session/useMe";
import { useBuildings, useRooms } from "./hooks";
import { RoomTile } from "./RoomTile";
import { COUNTER_ORDER, STATUS } from "./status";
import { lp } from "../../lib/locale";

const clock = () => {
  const d = new Date();
  const p2 = (n: number) => String(n).padStart(2, "0");
  return `${p2(d.getHours())}:${p2(d.getMinutes())}, ${p2(d.getDate())}/${p2(d.getMonth() + 1)}`;
};

export function RoomMap() {
  const router = useRouter();
  const b = useSearchParams().get("b");
  const me = useMe();
  const buildings = useBuildings();
  const current = buildings.data?.find((x) => x.id === b) ?? buildings.data?.[0];
  const rooms = useRooms(current?.id);
  const readOnly = current?.level === "VIEW";

  if (buildings.isError || rooms.isError)
    return (
      <AppFrame>
        <QueryError onRetry={() => void Promise.all([buildings.refetch(), rooms.refetch()])} />
      </AppFrame>
    );

  return (
    <AppFrame>
      <main className="mx-auto flex max-w-[1280px] flex-col pb-8">
        <header className="flex flex-col gap-1 px-5 pb-3 pt-5 md:flex-row md:items-baseline md:gap-4">
          <h1 className="text-xl font-bold">{me.data?.tenant.name}</h1>
          <p className="text-[13px] text-muted-foreground">
            {me.data && t(`rooms.role${me.data.user.role}` as MessageKey)} · {clock()}
          </p>
        </header>
        <ScrollArea className="px-5 pb-3 md:max-w-md">
          <ToggleGroup
            type="single"
            value={current?.id ?? ""}
            onValueChange={(id) => id && router.replace(lp(`/rooms?b=${id}`))}
            aria-label={t("rooms.buildings")}
            className="flex w-full gap-2 pb-2"
          >
            {buildings.data?.map((x) => (
              <ToggleGroupItem
                key={x.id}
                value={x.id}
                className="relative h-11 min-w-28 flex-1 rounded-[10px] border border-border bg-card px-3 text-[15px] font-semibold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
              >
                {x.id === current?.id && (
                  <SlidingPill
                    id="building-pill"
                    className="absolute inset-0 -z-0 rounded-[10px] bg-primary"
                  />
                )}
                <span className="relative whitespace-nowrap">
                  {x.name} ({Object.values(x.counts).reduce((a, n) => a + n, 0)})
                </span>
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <ScrollBar orientation="horizontal" />
        </ScrollArea>
        {current && (
          <ul className="flex flex-wrap gap-1.5 px-5 pb-3.5">
            {COUNTER_ORDER.map(([status, key]) => (
              <li key={status}>
                <Badge variant={STATUS[status].variant} className="px-2.5 py-1.5 font-semibold">
                  {t(STATUS[status].label)} <RollingNumber value={current.counts[key]} />
                </Badge>
              </li>
            ))}
          </ul>
        )}
        {readOnly && (
          <p className="mx-5 mb-3 rounded-[10px] bg-sunken p-3 text-[13px] text-ink-2">
            {t("rooms.viewOnlyNote")}
          </p>
        )}
        <ul className="grid grid-cols-3 gap-2 px-5 md:grid-cols-5 lg:grid-cols-6">
          {rooms.data?.map((r, i) => (
            <li key={r.id}>
              <FadeIn delay={Math.min(i, 10) * 0.03}>
                <RoomTile room={r} readOnly={readOnly} />
              </FadeIn>
            </li>
          ))}
        </ul>
        <p className="mx-5 mt-4 text-[13px] text-muted-foreground">{t("rooms.hint")}</p>
      </main>
    </AppFrame>
  );
}
