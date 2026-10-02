"use client";

import dynamic from "next/dynamic";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { motion } from "motion/react";
import { Download } from "lucide-react";
import { FadeIn, SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { duration, ease } from "@/lib/motion";
import { lp } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { useReport, type IncomeCostReport } from "./hooks";
import { addMonths, isMonth, monthLabel, thisMonth } from "./month";

const MonthBars = dynamic(() => import("./MonthBars"), {
  ssr: false,
  loading: () => <Skeleton className="h-60 rounded-card" />,
});
type Share = IncomeCostReport["expensesByCategory"][number];

function ranges() {
  const now = thisMonth();
  const year = now.slice(0, 4);
  const q = Math.floor((Number(now.slice(5)) - 1) / 3) * 3 + 1;
  return {
    thisMonth: [now, now],
    lastMonth: [addMonths(now, -1), addMonths(now, -1)],
    quarter: [
      `${year}-${String(q).padStart(2, "0")}`,
      addMonths(`${year}-${String(q).padStart(2, "0")}`, 2),
    ],
    sixMonths: [addMonths(now, -5), now],
    thisYear: [`${year}-01`, now],
  } as const;
}
type Preset = keyof ReturnType<typeof ranges>;
const PRESETS: Preset[] = ["thisMonth", "lastMonth", "quarter", "sixMonths", "thisYear"];

function Kpi({ label, value, tone }: { label: string; value: string; tone?: "ok" }) {
  return (
    <Card
      className={cn("gap-0.5 p-4 shadow-none", tone === "ok" && "border-ok-line bg-ok-bg text-ok")}
    >
      <p className="text-[13px]">{label}</p>
      <p
        className={cn(
          "text-[22px] font-bold leading-tight lg:text-[19px] xl:text-[24px]",
          !tone && "text-ink",
        )}
      >
        {value}
      </p>
    </Card>
  );
}

function Bars({ rows, label }: { rows: Share[]; label: (k: string) => string }) {
  const max = Math.max(1, ...rows.map((r) => r.amount));
  return (
    <ul className="flex flex-col gap-2.5">
      {rows.map((r) => (
        <li key={r.key}>
          <p className="flex justify-between text-[14px]">
            <span>{label(r.key)}</span>
            <b>{formatVnd(r.amount)}</b>
          </p>
          <span className="mt-1 block h-1.5 overflow-hidden rounded-full bg-sunken">
            <motion.span
              className="block h-full origin-left rounded-full bg-primary"
              style={{ width: `${(r.amount / max) * 100}%` }}
              initial={{ scaleX: 0 }}
              animate={{ scaleX: 1 }}
              transition={{ duration: duration.slow, ease: ease.standard }}
            />
          </span>
        </li>
      ))}
    </ul>
  );
}

const heading = "text-[13px] font-bold uppercase tracking-wide text-ink-2";

export function ReportView() {
  const router = useRouter();
  const params = useSearchParams();
  const def = ranges().quarter;
  const from = isMonth(params.get("from")) ? params.get("from")! : def[0];
  const to = isMonth(params.get("to")) ? params.get("to")! : def[1];
  const ok = from <= to;
  const q = useReport(ok ? from : to, to);
  const go = (a: string, b: string) => router.replace(lp(`/owner/reports?from=${a}&to=${b}`));
  const active = (Object.entries(ranges()) as [Preset, readonly string[]][]).find(
    ([, [a, b]]) => a === from && b === to,
  )?.[0];
  const options = Array.from({ length: 24 }, (_, i) => addMonths(thisMonth(), -i));
  const d = q.data;

  if (q.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void q.refetch()} />
      </AppFrame>
    );

  const sorted = [...(d?.expensesByCategory ?? [])].sort((a, b) => b.amount - a.amount);
  const cat = (k: string) => t(`expense.cat.${k}` as MessageKey);
  const monthSelect = (value: string, onChange: (m: string) => void, label: string) => (
    <label className="flex flex-col gap-1 text-[12px] font-bold text-ink-2">
      {label}
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger className="h-12 w-full bg-card text-[15px] font-bold" aria-label={label}>
          <SelectValue>{monthLabel(value).padStart(7, "0")}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {options.map((m) => (
            <SelectItem key={m} value={m}>
              {monthLabel(m).padStart(7, "0")}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </label>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("report.title")}
          subtitle={tf("report.sub", { from: monthLabel(from), to: monthLabel(to) })}
          back="/owner"
          right={
            <Button variant="outline" size="lg" disabled={!d} onClick={() => d && downloadCsv(d)}>
              <Download aria-hidden="true" />
              <span className="hidden lg:inline">{t("report.csv")}</span>
              <span className="sr-only lg:hidden">{t("report.csv")}</span>
            </Button>
          }
        />
        <div className="flex flex-col gap-3 px-5">
          <div className="flex flex-wrap items-end justify-between gap-3">
            <ToggleGroup
              type="single"
              value={active ?? ""}
              onValueChange={(k) => k && go(ranges()[k as Preset][0], ranges()[k as Preset][1])}
              aria-label={t("report.title")}
              className="-mx-5 flex w-auto flex-nowrap justify-start gap-2 overflow-x-auto px-5 lg:mx-0 lg:flex-wrap lg:px-0"
            >
              {PRESETS.map((p) => (
                <ToggleGroupItem
                  key={p}
                  value={p}
                  className="relative h-11 shrink-0 rounded-full! border border-border bg-card px-4 text-[15px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
                >
                  {active === p && (
                    <SlidingPill
                      id="rep-pill"
                      className="absolute inset-0 rounded-full bg-primary"
                    />
                  )}
                  <span className="relative whitespace-nowrap">{t(`report.${p}`)}</span>
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <div className="grid w-full grid-cols-2 gap-3 lg:w-[300px]">
              {monthSelect(from, (m) => go(m, to), t("report.from"))}
              {monthSelect(to, (m) => go(from, m), t("report.to"))}
            </div>
          </div>
          {!ok && (
            <p role="alert" className="text-[13px] font-bold text-destructive">
              {t("report.invalid")}
            </p>
          )}
          {q.isLoading && <Skeleton className="h-72 rounded-card" aria-busy="true" />}
          {d && (
            <FadeIn className="flex flex-col gap-3">
              <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
                <Kpi label={t("report.revenue")} value={formatVnd(d.revenue)} />
                <Kpi label={t("report.expenses")} value={formatVnd(d.expenses)} />
                <Kpi tone="ok" label={t("report.profit")} value={formatVnd(d.profit)} />
                <Kpi label={t("report.margin")} value={`${Math.round(d.marginPct)}%`} />
                <div className="max-lg:hidden">
                  <Kpi label={t("report.occupancy")} value={`${Math.round(d.occupancyPct)}%`} />
                </div>
              </div>
              <div className="grid gap-3 lg:grid-cols-[1.4fr_1fr] lg:items-start lg:gap-4">
                <Card className="gap-1 p-5 shadow-none">
                  <h2 className={heading}>
                    <span className="lg:hidden">{t("report.byMonth")}</span>
                    <span className="max-lg:hidden">{t("report.byMonthPc")}</span>
                  </h2>
                  <ul className="flex gap-4 text-[13px] text-ink-2">
                    <li className="flex items-center gap-1.5">
                      <span className="size-2.5 rounded-[3px] bg-chart-1" aria-hidden="true" />
                      {t("report.revenue")}
                    </li>
                    <li className="flex items-center gap-1.5">
                      <span className="size-2.5 rounded-[3px] bg-chart-2" aria-hidden="true" />
                      {t("report.expenses")}
                    </li>
                  </ul>
                  {d.months.length ? (
                    <MonthBars months={d.months} height={250} />
                  ) : (
                    <p className="py-8 text-center text-muted-foreground">{t("report.empty")}</p>
                  )}
                </Card>
                <Card className="gap-2 p-5 shadow-none">
                  <h2 className={heading}>
                    <span className="lg:hidden">{t("report.biggest")}</span>
                    <span className="max-lg:hidden">{t("report.byCategory")}</span>
                  </h2>
                  <ul className="flex flex-col gap-1.5 lg:hidden">
                    {sorted.slice(0, 5).map((r) => (
                      <li key={r.key} className="flex justify-between text-[15px]">
                        <span>{cat(r.key)}</span>
                        <b>{formatVnd(r.amount)}</b>
                      </li>
                    ))}
                  </ul>
                  <div className="max-lg:hidden">
                    <Bars rows={sorted.slice(0, 8)} label={cat} />
                    {sorted.length > 8 && (
                      <Link
                        href={lp(`/owner/expenses?month=${to}`)}
                        className="mt-3 inline-block text-sm font-bold text-primary underline underline-offset-2"
                      >
                        {tf("report.seeAll", { n: sorted.length })}
                      </Link>
                    )}
                  </div>
                </Card>
              </div>
              <div className="grid gap-3 max-lg:hidden lg:grid-cols-3 lg:items-start lg:gap-4">
                <Card className="gap-2 p-5 shadow-none">
                  <h2 className={heading}>{t("report.byRental")}</h2>
                  <Bars
                    rows={d.revenueByRentalType}
                    label={(k) => t(`report.rental.${k}` as MessageKey)}
                  />
                </Card>
                <Card className="gap-2 p-5 shadow-none">
                  <h2 className={heading}>{t("report.byBuilding")}</h2>
                  <Bars
                    rows={d.revenueByBuilding}
                    label={(k) => tf("report.building", { code: k })}
                  />
                </Card>
                <Card className="gap-2 p-5 shadow-none">
                  <h2 className={heading}>{t("report.byMethod")}</h2>
                  <Bars
                    rows={d.revenueByMethod}
                    label={(k) => t(`report.method.${k}` as MessageKey)}
                  />
                </Card>
              </div>
              <Button asChild variant="outline" size="lg" className="font-bold lg:hidden">
                <Link href={lp(`/owner/expenses?month=${to}`)}>{t("report.manage")}</Link>
              </Button>
            </FadeIn>
          )}
        </div>
      </main>
    </AppFrame>
  );
}

function downloadCsv(d: IncomeCostReport) {
  const lines = [
    "month,revenue,expenses",
    ...d.months.map((m) => `${m.month},${m.revenue},${m.expenses}`),
  ];
  const blob = new Blob(["﻿" + lines.join("\n")], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  Object.assign(document.createElement("a"), {
    href: url,
    download: `income-costs-${d.from}_${d.to}.csv`,
  }).click();
  URL.revokeObjectURL(url);
}
