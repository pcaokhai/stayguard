"use client";

import { Lock } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { FadeIn, RollingNumber, SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Badge } from "@/components/ui/badge";
import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { lp } from "../../lib/locale";
import { t, type MessageKey } from "../../lib/t";
import { useMe } from "../session/useMe";
import { useBuildings, useRooms } from "./hooks";
import { RoomPanel } from "./RoomPanel";
import { RoomTile } from "./RoomTile";
import { COUNTER_ORDER, STATUS } from "./status";

const p2 = (n: number) => String(n).padStart(2, "0");
const clock = () => {
  const d = new Date();
  return `${p2(d.getHours())}:${p2(d.getMinutes())}, ${p2(d.getDate())}/${p2(d.getMonth() + 1)}`;
};

// Front desk map (/rooms) and the owner's (/owner/rooms): same page, the owner has no shift.
export function RoomMap({ owner = false }: { owner?: boolean }) {
  const router = useRouter();
  const params = useSearchParams();
  const b = params.get("b");
  const selectedId = params.get("room");
  const base = owner ? "/owner/rooms" : "/rooms";
  const wide = useMediaQuery("(min-width: 1024px)");
  const me = useMe();
  const buildings = useBuildings();
  const current = buildings.data?.find((x) => x.id === b) ?? buildings.data?.[0];
  const rooms = useRooms(current?.id);
  const readOnly = current?.level === "VIEW";
  // With a side panel the first occupied room is shown until another is picked.
  const selected =
    rooms.data?.find((r) => r.id === selectedId) ??
    (wide ? rooms.data?.find((r) => r.activeStay) : undefined);
  const go = (next: Record<string, string>) => {
    const q = new URLSearchParams(params);
    for (const [k, v] of Object.entries(next)) q.set(k, v);
    router.replace(lp(`${base}?${q}`));
  };

  if (buildings.isError || rooms.isError)
    return (
      <AppFrame>
        <QueryError onRetry={() => void Promise.all([buildings.refetch(), rooms.refetch()])} />
      </AppFrame>
    );

  return (
    <AppFrame>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col pb-8">
        {owner ? (
          <TopBar title={t("rooms.title")} subtitle={t("rooms.ownerSub")} back="/owner" />
        ) : (
          <header className="flex flex-col gap-1 px-5 pb-3 pt-5 md:flex-row md:items-baseline md:gap-4">
            <h1 className="text-xl font-bold">{me.data?.tenant.name}</h1>
            <p className="text-[13px] text-muted-foreground">
              {me.data && t(`rooms.role${me.data.user.role}` as MessageKey)} · {clock()}
            </p>
          </header>
        )}
        <div className="grid gap-x-6 lg:grid-cols-[1fr_300px] xl:grid-cols-[1fr_360px]">
          <div className="min-w-0">
            <ScrollArea className="px-5 pb-3">
              <ToggleGroup
                type="single"
                value={current?.id ?? ""}
                onValueChange={(id) => id && go({ b: id })}
                aria-label={t("rooms.buildings")}
                className="flex w-max gap-2 pb-2"
              >
                {buildings.data?.map((x) => (
                  <ToggleGroupItem
                    key={x.id}
                    value={x.id}
                    className="relative h-11 shrink-0 rounded-full! border border-border bg-card px-4 text-[15px] font-semibold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
                  >
                    {x.id === current?.id && (
                      <SlidingPill
                        id="building-pill"
                        className="absolute inset-0 rounded-full bg-primary"
                      />
                    )}
                    <span className="relative flex items-center gap-1.5 whitespace-nowrap">
                      {x.level === "VIEW" && (
                        <Lock className="size-3.5" aria-label={t("rooms.viewOnly")} />
                      )}
                      {x.name} · {Object.values(x.counts).reduce((a, n) => a + n, 0)}
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
              <p className="mx-5 mb-3 rounded-[10px] bg-secondary p-3 text-[13px] text-ink-2">
                {t("rooms.viewOnlyNote")}
              </p>
            )}
            <ul className="grid grid-cols-3 gap-2 px-5 md:grid-cols-5 xl:grid-cols-6">
              {rooms.data?.map((r, i) => (
                <li key={r.id}>
                  <FadeIn delay={Math.min(i, 10) * 0.03}>
                    <RoomTile
                      room={r}
                      readOnly={readOnly && !wide}
                      selected={wide && r.id === selected?.id}
                      onSelect={wide ? (room) => go({ room: room.id }) : undefined}
                    />
                  </FadeIn>
                </li>
              ))}
            </ul>
            <p className="mx-5 mt-4 text-[13px] text-muted-foreground">
              {owner ? t("rooms.ownerNote") : t("rooms.hint")}
            </p>
          </div>
          <aside
            className="hidden lg:block"
            aria-label={t("panel.room").replace("{code}", selected?.code ?? "")}
          >
            <RoomPanel room={selected} readOnly={readOnly} />
          </aside>
        </div>
      </main>
    </AppFrame>
  );
}
