"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { lp } from "../../lib/locale";
import { shiftDay, today } from "../history/dates";
import { t, tf } from "../../lib/t";
import { useMe } from "../session/useMe";
import { LeaveForm } from "./LeaveForm";
import { LeaveList } from "./LeaveList";
import { useMyLeave, useMyRoster } from "./hooks";

// Boards P57 (list) and P58 (cancel sheet, inside LeaveList).
export function LeaveListView() {
  const me = useMe().data;
  const leave = useMyLeave();
  const [filter, setFilter] = useState<"all" | "upcoming" | "past">("all");
  if (leave.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void leave.refetch()} />
      </AppFrame>
    );
  const b = leave.data?.balance;
  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[640px] flex-col gap-3 pb-8">
        <TopBar
          title={t("leave.listTitle")}
          subtitle={me && b ? tf("leave.listSub", { name: me.user.name, year: b.year }) : undefined}
          back="/me/schedule"
        />
        <div className="flex flex-col gap-3 px-5">
          <div className="grid grid-cols-3 gap-2.5">
            {b ? (
              (
                [
                  ["annual", b.annual],
                  ["used", b.used],
                  ["left", b.left],
                ] as const
              ).map(([k, v]) => (
                <Card key={k} className="gap-1 p-3 shadow-none">
                  <p className="text-xs text-muted-foreground">{t(`schedule.${k}`)}</p>
                  <b className="text-2xl">{v}</b>
                </Card>
              ))
            ) : (
              <Skeleton className="col-span-3 h-16" />
            )}
          </div>
          <Button asChild size="lg">
            <Link href={lp("/me/leave/new")}>+ {t("schedule.requestLeave")}</Link>
          </Button>
          <ToggleGroup
            type="single"
            value={filter}
            onValueChange={(v) => v && setFilter(v as typeof filter)}
            className="flex justify-start gap-2"
          >
            {(["all", "upcoming", "past"] as const).map((f) => (
              <ToggleGroupItem
                key={f}
                value={f}
                className="h-10 rounded-full! border bg-card px-4 font-bold data-[state=on]:border-transparent data-[state=on]:bg-primary data-[state=on]:text-primary-foreground"
              >
                {t(`leave.${f}`)}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          {!leave.data ? (
            <Skeleton className="h-28" />
          ) : (
            <LeaveList items={leave.data.items} filter={filter} />
          )}
          <p className="text-[13px] text-muted-foreground">{t("leave.note")}</p>
        </div>
      </main>
    </AppFrame>
  );
}

// Board P52 on its own page (phone and tablet); desktop has the form beside the schedule.
export function LeaveNewView() {
  const router = useRouter();
  const roster = useMyRoster(today(), shiftDay(today(), 90));
  const me = useMe().data;
  const shiftFor = (iso: string) =>
    roster.data?.assignments.find((a) => a.userId === me?.user.id && a.date === iso)?.shift;
  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-1 flex-col">
        <TopBar title={t("leave.formTitle")} back="/me/schedule" />
        <div className="flex flex-1 flex-col px-5 pb-8">
          <LeaveForm shiftFor={shiftFor} onDone={() => router.replace(lp("/me/leave"))} />
        </div>
      </main>
    </AppFrame>
  );
}
