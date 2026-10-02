"use client";

import { ChevronRight } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useState, type ReactNode } from "react";
import type { components } from "../../api/generated/schema";
import { RollingNumber, SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { duration, ease } from "@/lib/motion";
import { cn } from "@/lib/utils";
import { lp } from "../../lib/locale";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { useNow } from "@/hooks/useNow";
import { useBuildings } from "../rooms/hooks";
import { useMe } from "../session/useMe";
import { useTasks } from "./hooks";

type Task = components["schemas"]["HousekeepingTask"];
const LONG_WAIT_MINUTES = 30;

// "Cần dọn ({n})" with the number rolling when it changes.
const withNumber = (key: "hk.toCleanTab" | "hk.doneTab", n: number): ReactNode => {
  const [a, b] = t(key).split("{n}");
  return (
    <>
      {a}
      <RollingNumber value={n} />
      {b}
    </>
  );
};

export const waitText = (minutes: number) =>
  minutes >= 60
    ? tf("stay.hoursMinutes", { h: Math.floor(minutes / 60), m: minutes % 60 })
    : tf("hk.waitMin", { m: minutes });

export const waitedMinutes = (x: Task, now: number) =>
  x.waitingMinutes ?? Math.max(0, Math.round((now - new Date(x.createdAt).getTime()) / 60_000));

export function HousekeepingList() {
  const tasks = useTasks();
  const buildings = useBuildings().data;
  const me = useMe().data;
  const now = useNow();
  const [tab, setTab] = useState<"open" | "done">("open");
  const [building, setBuilding] = useState("all");

  if (tasks.isError)
    return (
      <AppFrame>
        <QueryError onRetry={() => void tasks.refetch()} />
      </AppFrame>
    );
  const all = tasks.data ?? [];
  const inBuilding = (x: Task) => building === "all" || x.buildingId === building;
  const open = all
    .filter((x) => x.status === "OPEN" && inBuilding(x))
    .sort((a, b) => waitedMinutes(b, now) - waitedMinutes(a, now));
  const done = all.filter((x) => x.status === "DONE" && inBuilding(x));
  const openAll = all.filter((x) => x.status === "OPEN").length;
  const doneAll = all.filter((x) => x.status === "DONE").length;
  const shown = tab === "open" ? open : done;
  const nameOf = (id: string) => buildings?.find((b) => b.id === id)?.name ?? "";

  return (
    <AppFrame>
      <main className="mx-auto flex w-full max-w-[640px] flex-col gap-3 pb-8">
        <TopBar
          title={t("hk.title")}
          subtitle={tf("hk.sub", {
            name: me?.user.name ?? "",
            buildings: buildings?.map((b) => b.name).join(", ") ?? "",
          })}
        />
        <div className="flex flex-col gap-3 px-5">
          <div role="tablist" className="grid grid-cols-2 gap-1 rounded-xl bg-secondary p-1">
            {(["open", "done"] as const).map((k) => (
              <button
                key={k}
                type="button"
                role="tab"
                aria-selected={tab === k}
                onClick={() => setTab(k)}
                className={cn(
                  "relative h-10 rounded-[9px] text-sm font-bold",
                  tab === k ? "" : "text-muted-foreground",
                )}
              >
                {tab === k && (
                  <SlidingPill
                    id="hk-tab"
                    className="absolute inset-0 rounded-[9px] bg-card shadow-sm"
                  />
                )}
                <span className="relative">
                  {withNumber(
                    k === "open" ? "hk.toCleanTab" : "hk.doneTab",
                    k === "open" ? openAll : doneAll,
                  )}
                </span>
              </button>
            ))}
          </div>
          <ScrollArea>
            <ToggleGroup
              type="single"
              value={building}
              onValueChange={(v) => v && setBuilding(v)}
              aria-label={t("rooms.buildings")}
              className="flex w-max flex-wrap justify-start gap-2 pb-2"
            >
              {[
                ["all", t("hk.allBuildings")] as const,
                ...(buildings ?? []).map((b) => [b.id, b.name] as const),
              ].map(([id, label]) => (
                <ToggleGroupItem
                  key={id}
                  value={id}
                  className="h-10 rounded-full! border bg-card px-4 font-bold data-[state=on]:border-transparent data-[state=on]:bg-primary data-[state=on]:text-primary-foreground"
                >
                  {label}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <ScrollBar orientation="horizontal" />
          </ScrollArea>
          {tab === "open" && <p className="text-[13px] text-muted-foreground">{t("hk.longest")}</p>}
          {!tasks.data ? (
            <Skeleton className="h-24" />
          ) : !shown.length ? (
            <p className="py-6 text-center text-muted-foreground">
              {tab === "open" ? t("hk.empty") : t("hk.emptyDone")}
            </p>
          ) : (
            <ul className="flex flex-col gap-3">
              <AnimatePresence initial={false}>
                {shown.map((x) => (
                  <motion.li
                    key={x.id}
                    layout
                    exit={{ x: 40, opacity: 0 }}
                    transition={{ duration: duration.base, ease: ease.standard }}
                  >
                    <Link
                      href={lp(`/clean?room=${x.roomId}`)}
                      className="flex items-center gap-3 rounded-card border border-dirty-line bg-dirty-bg p-4"
                    >
                      <span className="min-w-0 flex-1 leading-tight">
                        <b className="block text-xl">{x.roomCode}</b>
                        <span className="text-[13px] text-ink-2">
                          {tf(tab === "open" ? "hk.checkedOutAt" : "hk.cleanedAt", {
                            building: nameOf(x.buildingId),
                            time: formatClock(
                              tab === "open" ? x.createdAt : (x.completedAt ?? x.createdAt),
                            ),
                          })}
                        </span>
                      </span>
                      {tab === "open" && (
                        <Badge
                          variant={
                            waitedMinutes(x, now) >= LONG_WAIT_MINUTES ? "overdue" : "maintenance"
                          }
                          className="px-3 py-1 font-bold"
                        >
                          {tf("hk.waiting", { time: waitText(waitedMinutes(x, now)) })}
                        </Badge>
                      )}
                      <ChevronRight className="size-4 text-muted-foreground" aria-hidden="true" />
                    </Link>
                  </motion.li>
                ))}
              </AnimatePresence>
            </ul>
          )}
          <Card className="mt-3 gap-2 p-4 shadow-none">
            <p className="text-sm font-bold">{t("hk.usedHint")}</p>
            <Button
              asChild
              variant="outline"
              className="border-destructive/30 text-destructive hover:text-destructive"
            >
              <Link href={lp("/report-used")}>{t("hk.reportOwner")}</Link>
            </Button>
          </Card>
        </div>
      </main>
    </AppFrame>
  );
}
