"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { AlertTriangle, ChevronLeft, ChevronRight, Info, Pencil, Users } from "lucide-react";
import { toast } from "sonner";
import { FadeIn } from "@/components/motion";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { lp } from "@/lib/locale";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { useStaff, type Staff } from "../staff/hooks";
import { AssignSheet, dayFull, dayKey, SHIFTS } from "./AssignSheet";
import {
  type Leave,
  type Roster,
  type ShiftCode,
  useCopyWeek,
  useDecideLeave,
  usePendingLeave,
  useRoster,
} from "./hooks";
import { addDays, dm, isDate, mondayOf, today, weekDays } from "./week";

const SHIFT_TONE: Record<ShiftCode, string> = {
  MORNING: "border-info-line bg-info-bg text-info",
  AFTERNOON: "border-ok-line bg-ok-bg text-ok",
  NIGHT: "border-line bg-maint-bg text-maint",
};
const pill =
  "inline-block rounded-full border px-2.5 py-0.5 text-[12px] font-bold whitespace-nowrap";
const covers = (l: Leave, date: string, shift?: ShiftCode) =>
  l.fromDate <= date && date <= l.toDate && (!l.shift || !shift || l.shift === shift);

export function RosterView() {
  const router = useRouter();
  const params = useSearchParams();
  const week = mondayOf(isDate(params.get("week")) ? params.get("week")! : today());
  const days = weekDays(week);
  const [day, setDay] = useState(() => (days.includes(today()) ? today() : days[0]));
  const roster = useRoster(week, days[6]);
  const staffQ = useStaff();
  const pending = usePendingLeave();
  const copy = useCopyWeek();
  const decide = useDecideLeave();
  const [assign, setAssign] = useState<{ date: string; userId?: string } | null>(null);
  const [declining, setDeclining] = useState<Leave | null>(null);
  const [reason, setReason] = useState("");
  const staff: Staff[] = (staffQ.data ?? []).filter((s) => s.status !== "REMOVED");
  const r: Roster | undefined = roster.data;
  const name = (id?: string | null) => staff.find((s) => s.id === id)?.name ?? "";
  const go = (w: string) => router.replace(lp(`/owner/roster?week=${w}`));
  const activeDay = days.includes(day) ? day : days[0];

  if (roster.isError || staffQ.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([roster.refetch(), staffQ.refetch()])} />
      </AppFrame>
    );

  const leaveFor = (userId: string, date: string, shift?: ShiftCode) =>
    r?.leave.find(
      (l) =>
        l.userId === userId &&
        covers(l, date, shift) &&
        l.status !== "DECLINED" &&
        l.status !== "CANCELLED",
    );
  const worksOn = (userId: string, date: string) =>
    SHIFTS.filter((sh) =>
      r?.assignments.some((a) => a.userId === userId && a.date === date && a.shift === sh),
    );
  const leaveText = (l: Leave) =>
    l.shift
      ? tf("roster.leaveLine", {
          day: dayFull(l.fromDate),
          date: dm(l.fromDate),
          shift: t(`roster.shift.${l.shift}` as MessageKey).toLowerCase(),
        })
      : tf("roster.leaveAllDay", { day: dayFull(l.fromDate), date: dm(l.fromDate) });
  const approve = (l: Leave) =>
    decide.mutate(
      { id: l.id },
      {
        onSuccess: () => toast.success(t("roster.approved")),
        onError: () => toast.error(t("roster.actionFailed")),
      },
    );
  const sendDecline = () =>
    declining &&
    decide.mutate(
      { id: declining.id, reason: reason.trim() },
      {
        onSuccess: () => {
          toast.success(t("roster.declined"));
          setDeclining(null);
          setReason("");
        },
        onError: () => toast.error(t("roster.actionFailed")),
      },
    );
  const request = (l: Leave, compact?: boolean) => (
    <Card
      key={l.id}
      className={cn("gap-1.5 p-4 shadow-none", compact && "border-dirty-line bg-dirty-bg")}
    >
      <b className="text-[17px]">{compact ? `${l.userName} · ${leaveText(l)}` : l.userName}</b>
      {!compact && <span className="text-[14px]">{leaveText(l)}</span>}
      <p className="text-[13px] text-ink-2">
        {[l.reason, t(`roster.kind.${l.kind}` as MessageKey)].filter(Boolean).join(" · ")}
        {l.coverUserId && name(l.coverUserId)
          ? ` · ${tf("roster.cover", { name: name(l.coverUserId) })}`
          : ""}
      </p>
      <div className="mt-1 grid grid-cols-2 gap-2">
        <Button
          variant="outline"
          size="lg"
          className="bg-card font-bold"
          onClick={() => setDeclining(l)}
        >
          {t("roster.decline")}
        </Button>
        <Button
          size="lg"
          className="font-bold"
          disabled={decide.isPending}
          onClick={() => approve(l)}
        >
          {t("roster.approve")}
        </Button>
      </div>
    </Card>
  );

  const nav = (
    <div className="flex items-center gap-2">
      <Button
        variant="outline"
        size="icon-lg"
        aria-label={t("roster.prevWeek")}
        onClick={() => go(addDays(week, -7))}
      >
        <ChevronLeft aria-hidden="true" />
      </Button>
      <b className="min-w-[150px] text-center text-[16px] leading-tight">
        {tf("roster.week", { from: dm(days[0]), to: dm(days[6]), year: days[6].slice(0, 4) })}
      </b>
      <Button
        variant="outline"
        size="icon-lg"
        aria-label={t("roster.nextWeek")}
        onClick={() => go(addDays(week, 7))}
      >
        <ChevronRight aria-hidden="true" />
      </Button>
    </div>
  );
  const emptyWeek = !!r && r.assignments.length === 0;
  const dayShifts = (date: string) =>
    SHIFTS.map((sh) => ({
      sh,
      people: (r?.assignments ?? []).filter((a) => a.date === date && a.shift === sh),
    }));

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("roster.title")}
          subtitle={tf("roster.sub", { n: staff.length, m: pending.data?.length ?? 0 })}
          back="/owner/settings"
          right={
            <Button asChild variant="outline" size="lg" className="hidden font-bold lg:inline-flex">
              <Link href={lp("/owner/payroll")}>
                <Users aria-hidden="true" />
                {t("roster.payroll")}
              </Link>
            </Button>
          }
        />
        <div className="flex flex-col gap-3 px-5">
          {/* Phone */}
          <div className="flex flex-col gap-3 lg:hidden">
            {pending.data?.slice(0, 1).map((l) => request(l, true))}
            <div className="flex items-center justify-between">{nav}</div>
            <div
              className="-mx-5 flex gap-2 overflow-x-auto px-5"
              role="tablist"
              aria-label={t("roster.title")}
            >
              {days.map((d) => (
                <button
                  key={d}
                  type="button"
                  role="tab"
                  aria-selected={d === activeDay}
                  onClick={() => setDay(d)}
                  className={cn(
                    "flex h-[60px] w-[52px] shrink-0 flex-col items-center justify-center rounded-[10px] border border-border bg-card text-[12px] transition-colors duration-100",
                    d === activeDay && "border-transparent bg-primary text-primary-foreground",
                  )}
                >
                  <span>{t(`roster.days.${dayKey(d)}` as MessageKey)}</span>
                  <b className="text-[16px]">{d.slice(8)}</b>
                </button>
              ))}
            </div>
            <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
              {dayFull(activeDay)}, {dm(activeDay)}
            </h2>
            {roster.isLoading && <Skeleton className="h-40 rounded-card" aria-busy="true" />}
            {r &&
              dayShifts(activeDay).map(({ sh, people }) => (
                <Card key={sh} className="gap-2 p-4 shadow-none">
                  <div className="flex items-center justify-between">
                    <span className={cn(pill, SHIFT_TONE[sh])}>
                      {t(`roster.shift.${sh}` as MessageKey)}
                    </span>
                    <span className="text-[13px] text-ink-2">
                      {t(`roster.hours.${sh}` as MessageKey)}
                    </span>
                  </div>
                  {people.length === 0 && (
                    <p className="text-[14px] text-muted-foreground">{t("roster.empty")}</p>
                  )}
                  {people.map((a) => (
                    <p key={a.userId} className="text-[15px]">
                      {name(a.userId)}
                      {leaveFor(a.userId, activeDay, sh) && (
                        <span className="mt-1 flex items-start gap-1.5 rounded-[10px] border border-warn-line bg-warn-bg p-2 text-[13px] text-warn-ink">
                          <AlertTriangle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
                          {leaveFor(a.userId, activeDay, sh)?.status === "PENDING"
                            ? t("roster.pending")
                            : t("roster.leave")}
                        </span>
                      )}
                    </p>
                  ))}
                </Card>
              ))}
            <Button
              variant="outline"
              size="lg"
              className="font-bold"
              onClick={() => setAssign({ date: activeDay })}
            >
              <Pencil aria-hidden="true" />
              {t("roster.assign")}
            </Button>
          </div>

          {/* Desktop and tablet */}
          <div className="hidden flex-col gap-3 lg:flex">
            <div className="flex flex-wrap items-center gap-3">
              {nav}
              <Button
                variant="outline"
                size="lg"
                className="font-bold"
                disabled={!emptyWeek || copy.isPending}
                title={t("roster.copyHint")}
                onClick={() =>
                  copy.mutate(week, {
                    onSuccess: () => toast.success(t("roster.copied")),
                    onError: () => toast.error(t("roster.copyFailed")),
                  })
                }
              >
                {t("roster.copy")}
              </Button>
              <span className="flex items-center gap-1.5 text-[14px] font-bold text-primary">
                <Pencil className="size-4" aria-hidden="true" />
                {t("roster.pick")}
              </span>
            </div>
            <div className="grid grid-cols-[minmax(0,1fr)_320px] items-start gap-4">
              <FadeIn>
                <Card className="gap-0 overflow-x-auto p-0 shadow-none">
                  {roster.isLoading ? (
                    <Skeleton className="h-72" />
                  ) : (
                    <table className="w-full min-w-[720px] text-[13px]">
                      <thead>
                        <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                          <th scope="col" className="px-3 pl-5 font-bold">
                            {t("roster.colStaff")}
                          </th>
                          {days.map((d) => (
                            <th key={d} scope="col" className="px-1 text-center font-bold">
                              {t(`roster.days.${dayKey(d)}` as MessageKey)} {dm(d)}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {staff.map((s) => (
                          <tr key={s.id} className="border-t border-border">
                            <td className="px-3 py-2.5 pl-5 leading-tight">
                              <b className="block text-[14px]">{s.name}</b>
                              <span className="text-[12px] text-muted-foreground">
                                {t(`staff.position.${s.position}` as MessageKey)}
                              </span>
                            </td>
                            {days.map((d) => {
                              const shifts = worksOn(s.id, d);
                              const lv = leaveFor(s.id, d);
                              return (
                                <td key={d} className="px-1 text-center">
                                  <button
                                    type="button"
                                    onClick={() => setAssign({ date: d, userId: s.id })}
                                    aria-label={`${s.name} ${dayFull(d)} ${dm(d)}`}
                                    className="flex min-h-10 w-full flex-col items-center justify-center gap-1 rounded-[10px] px-1 py-1 transition-colors duration-100 hover:bg-sunken"
                                  >
                                    {lv && (
                                      <span
                                        className={cn(
                                          pill,
                                          lv.status === "PENDING"
                                            ? "border-dirty-line bg-dirty-bg text-dirty"
                                            : "border-warn-line bg-warn-bg text-warn-ink",
                                        )}
                                      >
                                        {lv.status === "PENDING"
                                          ? t("roster.pending")
                                          : t("roster.leave")}
                                      </span>
                                    )}
                                    {shifts.map((sh) => (
                                      <span key={sh} className={cn(pill, SHIFT_TONE[sh])}>
                                        {t(`roster.shift.${sh}` as MessageKey)}
                                      </span>
                                    ))}
                                    {!lv && shifts.length === 0 && (
                                      <span className="text-muted-foreground">
                                        {t("roster.off")}
                                      </span>
                                    )}
                                  </button>
                                </td>
                              );
                            })}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </Card>
                <p className="mt-2 flex items-start gap-2 text-[13px] text-muted-foreground">
                  <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
                  {t("roster.foot")}
                </p>
              </FadeIn>
              <div className="flex flex-col gap-3">
                <Card className="gap-2 p-5 shadow-none">
                  <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
                    {tf("roster.requests", { n: pending.data?.length ?? 0 })}
                  </h2>
                  {pending.data?.length === 0 && (
                    <p className="text-sm text-muted-foreground">{t("roster.noRequests")}</p>
                  )}
                  {pending.data?.map((l) => request(l))}
                </Card>
                <Card className="gap-2 p-5 shadow-none">
                  <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
                    {t("roster.attention")}
                  </h2>
                  {(r?.gaps ?? []).length === 0 && (
                    <p className="text-sm text-muted-foreground">{t("roster.noAttention")}</p>
                  )}
                  {(r?.gaps ?? []).map((g) => (
                    <p
                      key={`${g.date}${g.shift}`}
                      className="flex items-start gap-2 rounded-card border border-warn-line bg-warn-bg p-3 text-[13px] text-warn-ink"
                    >
                      <AlertTriangle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
                      {tf("roster.gap", {
                        date: `${dayFull(g.date)} ${dm(g.date)}`,
                        shift: t(`roster.shift.${g.shift}` as MessageKey).toLowerCase(),
                      })}
                    </p>
                  ))}
                </Card>
                <Card className="gap-1 p-5 shadow-none">
                  <h2 className="text-[13px] font-bold uppercase tracking-wide text-ink-2">
                    {t("roster.shiftsTitle")}
                  </h2>
                  {SHIFTS.map((sh) => (
                    <p key={sh} className="flex justify-between text-[14px]">
                      <span className="text-ink-2">{t(`roster.shift.${sh}` as MessageKey)}</span>
                      <b>{t(`roster.hours.${sh}` as MessageKey)}</b>
                    </p>
                  ))}
                </Card>
              </div>
            </div>
          </div>
        </div>
      </main>
      {assign && r && (
        <AssignSheet
          key={`${assign.date}${assign.userId}`}
          date={assign.date}
          only={assign.userId}
          staff={staff}
          roster={r}
          onClose={() => setAssign(null)}
        />
      )}
      {declining && (
        <ResponsiveDialog
          open
          onOpenChange={(o) => !o && setDeclining(null)}
          title={t("roster.declineTitle")}
        >
          <label className="flex flex-col gap-1 text-[13px] font-bold">
            {t("roster.declineReason")}
            <Textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              className="min-h-24 rounded-[10px] bg-card text-[15px] font-normal"
            />
          </label>
          {reason.trim().length > 0 && reason.trim().length < 2 && (
            <p className="text-[12px] text-destructive">{t("roster.reasonShort")}</p>
          )}
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button variant="outline" size="lg" onClick={() => setDeclining(null)}>
              {t("roster.cancel")}
            </Button>
            <Button
              size="lg"
              disabled={reason.trim().length < 2 || decide.isPending}
              onClick={sendDecline}
            >
              {t("roster.declineSend")}
            </Button>
          </div>
        </ResponsiveDialog>
      )}
    </AppFrame>
  );
}
