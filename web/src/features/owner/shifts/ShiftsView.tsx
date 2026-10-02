"use client";

import Link from "next/link";
import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { SlidingPill, StaggerList } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { lp } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { cn } from "@/lib/utils";
import { clockOf, formatDayMonth } from "../format";
import { useClosedShifts } from "./hooks";
import { dayLabel, differenceText, differenceTone, shiftName, timeRange } from "./labels";
import { ShiftReview } from "./ShiftReview";

const ALL = "__all";
const DIFF = "__diff";

// List of closed shifts (DanhSachCa); with a shift selected, the review (DoiSoatCa). On desktop both show side by side.
export function ShiftsView({ selectedId }: { selectedId?: string }) {
  const [filter, setFilter] = useState(ALL);
  const shifts = useClosedShifts(filter === DIFF);
  const all = useClosedShifts(false);
  const items = (shifts.data ?? []).filter(
    (s) => filter === ALL || filter === DIFF || s.userName === filter,
  );
  const names = [...new Set((all.data ?? []).map((s) => s.userName))];
  const off = (all.data ?? []).filter((s) => s.difference !== 0);
  const short = off.filter((s) => s.difference < 0).reduce((a, s) => a - s.difference, 0);
  // Desktop opens the first shift when none is chosen.
  const current = selectedId ?? items[0]?.id;
  const sel = (all.data ?? []).find((s) => s.id === current);

  if (shifts.isError || all.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([shifts.refetch(), all.refetch()])} />
      </AppFrame>
    );

  const chips = [
    { value: ALL, label: t("shifts.all") },
    { value: DIFF, label: t("shifts.diff") },
    ...names.map((n) => ({ value: n, label: n })),
  ];
  const list = (
    <div className="flex flex-col gap-3">
      <ToggleGroup
        type="single"
        value={filter}
        onValueChange={(v) => v && setFilter(v)}
        aria-label={t("shifts.title")}
        className="flex flex-wrap justify-start gap-2 lg:hidden"
      >
        {chips.map((c) => (
          <ToggleGroupItem
            key={c.value}
            value={c.value}
            className="relative h-11 shrink-0 rounded-full! border border-border bg-card px-4 text-[15px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
          >
            {filter === c.value && (
              <SlidingPill id="shift-pill" className="absolute inset-0 rounded-full bg-primary" />
            )}
            <span className="relative whitespace-nowrap">{c.label}</span>
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <h2 className="hidden text-[13px] font-bold uppercase tracking-wide text-ink-2 lg:block">
        {t("shifts.recent")}
      </h2>
      {shifts.isLoading && <Skeleton className="h-48 rounded-card" aria-busy="true" />}
      {!shifts.isLoading && items.length === 0 && (
        <p className="py-8 text-center text-muted-foreground">{t("shifts.empty")}</p>
      )}
      <StaggerList className="flex flex-col gap-2.5">
        {items.map((s) => (
          <Link
            key={s.id}
            href={lp(`/owner/shift?id=${encodeURIComponent(s.id)}`)}
            aria-current={s.id === current ? "true" : undefined}
            className={cn(
              "flex min-h-[64px] items-center gap-2 rounded-card border border-border bg-card p-3.5 transition-colors duration-100",
              s.id === current && "lg:border-info-line lg:bg-info-bg",
            )}
          >
            <span className="min-w-0 flex-1 leading-snug">
              <b className="block text-[16px]">
                {shiftName(s)} · {dayLabel(s.closedAt)}
              </b>
              <span className="text-[13px] text-ink-2">
                {timeRange(s)} · {s.userName}
              </span>
            </span>
            <span
              className={cn(
                "rounded-full border px-3 py-1 text-[13px] font-bold whitespace-nowrap",
                differenceTone(s.difference),
              )}
            >
              {differenceText(s.difference)}
            </span>
            <ChevronRight className="size-4 text-muted-foreground lg:hidden" aria-hidden="true" />
          </Link>
        ))}
      </StaggerList>
    </div>
  );

  const phoneDetail = !!selectedId;
  const title =
    phoneDetail && sel
      ? tf("shifts.detailTitle", { shift: shiftName(sel), date: formatDayMonth(sel.closedAt) })
      : t("shifts.title");
  const sub =
    phoneDetail && sel
      ? tf("shifts.detailSub", { user: sel.userName, time: clockOf(sel.closedAt) })
      : off.length
        ? tf("shifts.month", { n: off.length, amount: formatVnd(short) })
        : t("shifts.monthNone");

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar title={title} subtitle={sub} back={phoneDetail ? "/owner/shifts" : "/owner"} />
        <div className="grid gap-4 px-5 lg:grid-cols-[300px_1fr] lg:items-start">
          <div className={cn(phoneDetail && "max-lg:hidden")}>{list}</div>
          <div className={cn(!phoneDetail && "max-lg:hidden")}>
            {current ? (
              <ShiftReview id={current} />
            ) : (
              <p className="py-8 text-muted-foreground">{t("shifts.pickShift")}</p>
            )}
          </div>
        </div>
      </main>
    </AppFrame>
  );
}
