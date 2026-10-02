"use client";

import { ChevronLeft, ChevronRight, Users } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { FadeIn } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { lp } from "../../lib/locale";
import { t, tf, type MessageKey } from "../../lib/t";
import { useMe } from "../session/useMe";
import { fromIso, shiftDay, today, toIso } from "../history/dates";
import { dayMonth, shiftName, shortWeekday } from "./format";
import { LeaveForm } from "./LeaveForm";
import { LeaveList } from "./LeaveList";
import { SHIFT_HOURS, useMyLeave, useMyRoster } from "./hooks";

// Monday of the week containing `iso`.
const weekStart = (iso: string) => shiftDay(iso, -((fromIso(iso).getDay() + 6) % 7));
const monthBounds = (iso: string) => {
  const d = fromIso(iso);
  return [
    toIso(new Date(d.getFullYear(), d.getMonth(), 1)),
    toIso(new Date(d.getFullYear(), d.getMonth() + 1, 0)),
  ] as const;
};

// Boards P51 and the PC "Lịch và nghỉ". One month is loaded; the week shown is derived from it.
export function ScheduleView() {
  const router = useRouter();
  const me = useMe().data;
  const [start, setStart] = useState(() => weekStart(today()));
  const end = shiftDay(start, 6);
  const [monthFrom, monthTo] = monthBounds(start);
  // The week can straddle two months, so the range is the month plus the whole week.
  const roster = useMyRoster(monthFrom < start ? monthFrom : start, monthTo > end ? monthTo : end);
  const leave = useMyLeave();

  if (roster.isError || leave.isError)
    return (
      <AppFrame>
        <QueryError onRetry={() => void Promise.all([roster.refetch(), leave.refetch()])} />
      </AppFrame>
    );
  const mine = roster.data?.assignments.filter((a) => a.userId === me?.user.id) ?? [];
  const shiftOn = (iso: string) => mine.find((a) => a.date === iso)?.shift;
  const pendingOn = (iso: string) =>
    roster.data?.leave.some(
      (l) =>
        l.userId === me?.user.id && l.status === "PENDING" && l.fromDate <= iso && iso <= l.toDate,
    );
  const inMonth = mine.filter((a) => a.date >= monthFrom && a.date <= monthTo);
  const done = inMonth.filter((a) => a.date < today()).length;
  const week = Array.from({ length: 7 }, (_, i) => shiftDay(start, i));
  const position = me ? t(`rooms.role${me.user.role}` as MessageKey).toLowerCase() : "";
  const weekLabel = tf("schedule.week", { from: dayMonth(start), to: dayMonth(end) });
  const balance = leave.data?.balance;

  const Cell = ({ iso, wide }: { iso: string; wide?: boolean }) => {
    const shift = shiftOn(iso);
    const pending = pendingOn(iso);
    return (
      <div
        className={
          wide ? "flex flex-col items-center gap-1 text-center" : "flex items-center gap-3"
        }
      >
        {wide ? null : (
          <b className="w-[58px] text-[15px] leading-tight">
            {shortWeekday(iso)} <span className="block">{dayMonth(iso)}</span>
          </b>
        )}
        {pending ? (
          <Badge variant="warn" className="px-3 py-1 font-bold">
            {t("leave.PENDING")}
          </Badge>
        ) : shift ? (
          <Badge variant="ok" className="px-3 py-1 font-bold">
            {shiftName(shift)}
          </Badge>
        ) : (
          <span className="text-sm text-muted-foreground">{t("schedule.off")}</span>
        )}
        {shift && !pending && !wide && (
          <span className="ml-auto text-sm text-ink-2">{SHIFT_HOURS[shift]}</span>
        )}
      </div>
    );
  };

  return (
    <AppFrame>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-8">
        <div className="lg:flex lg:items-end lg:justify-between lg:gap-6 lg:px-5 lg:pt-5">
          <div className="lg:hidden">
            <TopBar
              title={t("schedule.title")}
              subtitle={me && tf("schedule.sub", { name: me.user.name, position })}
            />
          </div>
          <div className="hidden lg:block">
            <h1 className="text-[28px] font-bold leading-tight">{t("schedule.titlePc")}</h1>
            <p className="text-sm text-muted-foreground">
              {me &&
                tf("schedule.subWeek", {
                  name: me.user.name,
                  position,
                  from: dayMonth(start),
                  to: dayMonth(end),
                })}
            </p>
          </div>
          <div className="grid grid-cols-2 gap-3 px-5 lg:grid-cols-3 lg:px-0">
            {balance ? (
              <>
                <Card className="gap-1 p-3.5 shadow-none lg:hidden">
                  <p className="text-[13px] text-muted-foreground">{t("schedule.leaveLeft")}</p>
                  <b className="text-2xl">{balance.left}</b>
                </Card>
                <Card className="gap-1 p-3.5 shadow-none lg:hidden">
                  <p className="text-[13px] text-muted-foreground">{t("schedule.shiftsMonth")}</p>
                  <b className="text-2xl">
                    {done} / {inMonth.length}
                  </b>
                </Card>
                {(
                  [
                    ["annual", balance.annual],
                    ["used", balance.used],
                    ["left", balance.left],
                  ] as const
                ).map(([k, v]) => (
                  <Card key={k} className="hidden gap-1 p-3.5 shadow-none lg:flex">
                    <p className="text-[13px] text-muted-foreground">{t(`schedule.${k}`)}</p>
                    <b className="text-2xl">{v}</b>
                  </Card>
                ))}
              </>
            ) : (
              <Skeleton className="col-span-2 h-16" />
            )}
          </div>
        </div>

        <div className="flex items-center justify-between gap-2 px-5">
          <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">{weekLabel}</h2>
          <div className="flex gap-1.5">
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              aria-label={t("schedule.prevWeek")}
              onClick={() => setStart((s) => shiftDay(s, -7))}
            >
              <ChevronLeft />
            </Button>
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              aria-label={t("schedule.nextWeek")}
              onClick={() => setStart((s) => shiftDay(s, 7))}
            >
              <ChevronRight />
            </Button>
          </div>
        </div>
        {!roster.data ? (
          <Skeleton className="mx-5 h-72" />
        ) : (
          <FadeIn key={start} className="px-5">
            <Card className="gap-0 p-0 shadow-none lg:hidden">
              <ul>
                {week.map((iso) => (
                  <li key={iso} className="border-b border-border px-4 py-3 last:border-0">
                    <Cell iso={iso} />
                  </li>
                ))}
              </ul>
            </Card>
            <ul className="hidden grid-cols-7 gap-3 lg:grid">
              {week.map((iso) => (
                <li key={iso}>
                  <Card className="h-full items-center gap-1 p-3.5 text-center shadow-none">
                    <span className="text-[13px] text-muted-foreground">{shortWeekday(iso)}</span>
                    <b className="text-lg">{dayMonth(iso)}</b>
                    <Cell iso={iso} wide />
                  </Card>
                </li>
              ))}
            </ul>
          </FadeIn>
        )}
        <p className="flex items-start gap-2 px-5 text-[13px] text-muted-foreground lg:hidden">
          <Users className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          {t("schedule.swapNote")}
        </p>
        <div className="mt-auto grid grid-cols-2 gap-3 px-5 lg:hidden">
          <Button asChild variant="outline" size="lg">
            <Link href={lp("/me/leave")}>{t("schedule.myRequests")}</Link>
          </Button>
          <Button asChild size="lg">
            <Link href={lp("/me/leave/new")}>+ {t("schedule.requestLeave")}</Link>
          </Button>
        </div>

        <div className="hidden gap-5 px-5 lg:grid lg:grid-cols-[1fr_340px] lg:items-start">
          <section aria-label={t("schedule.requests")} className="flex flex-col gap-3">
            <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
              {t("schedule.requests")}
            </h2>
            <LeaveList items={leave.data?.items ?? []} />
          </section>
          <Card className="gap-3 p-5 shadow-none">
            <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
              {t("schedule.requestLeave")}
            </h2>
            <LeaveForm shiftFor={shiftOn} onDone={() => router.refresh()} />
          </Card>
        </div>
      </main>
    </AppFrame>
  );
}
