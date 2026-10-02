"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { Check, ChevronLeft, ChevronRight, Download, Info } from "lucide-react";
import { toast } from "sonner";
import { FadeIn, SlidingPill } from "@/components/motion";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { newIdempotencyKey } from "@/lib/api";
import { lp } from "@/lib/locale";
import { formatVnd, parseVnd, vndNumber } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { type PayrollLine, useMarkPaid, usePayroll, useUpdatePayrollLine } from "./hooks";
import { addMonths, isMonth, monthLabel, thisMonth } from "./month";

const FILTERS = ["ALL", "FRONT_DESK", "HOUSEKEEPING", "SECURITY", "MANAGER"] as const;
type Filter = (typeof FILTERS)[number];

const contract = (l: PayrollLine) =>
  l.payType === "MONTHLY"
    ? t("payroll.monthly")
    : tf(l.payType === "PER_SHIFT" ? "payroll.perShift" : "payroll.hourly", {
        rate: formatVnd(l.rate),
      });
const shifts = (l: PayrollLine) =>
  l.standardShifts
    ? tf("payroll.shiftsOf", { n: l.shiftsWorked, std: l.standardShifts })
    : String(l.shiftsWorked);

// Bonus or deduction typed by the owner; saved when the field loses focus (docs/15 rule 15).
function Adjust({
  line,
  field,
  month,
}: {
  line: PayrollLine;
  field: "bonus" | "deduction";
  month: string;
}) {
  const update = useUpdatePayrollLine(month);
  const [text, setText] = useState<string | null>(null);
  const value = text ?? vndNumber(line[field]);
  const commit = () => {
    if (text === null) return;
    const n = parseVnd(text);
    setText(null);
    if (n === line[field]) return;
    update.mutate(
      { userId: line.userId, [field]: n },
      { onError: () => toast.error(t("payroll.saveFailed")) },
    );
  };
  return (
    <Input
      value={value}
      inputMode="numeric"
      disabled={line.status === "PAID"}
      aria-label={`${t(field === "bonus" ? "payroll.colBonus" : "payroll.colDeduction")} ${line.name}`}
      onChange={(e) => setText(e.target.value.replace(/[^\d.,]/g, ""))}
      onBlur={commit}
      className="h-10 w-[120px] rounded-[10px] bg-card text-right text-[14px]"
    />
  );
}

export function PayrollView() {
  const router = useRouter();
  const params = useSearchParams();
  const month = isMonth(params.get("month")) ? params.get("month")! : thisMonth();
  const q = usePayroll(month);
  const mark = useMarkPaid(month);
  const [filter, setFilter] = useState<Filter>("ALL");
  const [confirm, setConfirm] = useState(false);
  const [key] = useState(newIdempotencyKey);
  const go = (m: string) => router.replace(lp(`/owner/payroll?month=${m}`));
  const all = q.data?.lines ?? [];
  const rows = all.filter((l) => filter === "ALL" || l.position === filter);
  const unpaid = all.filter((l) => l.status === "UNPAID").length;
  const sum = (f: (l: PayrollLine) => number) => rows.reduce((n, l) => n + f(l), 0);

  if (q.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void q.refetch()} />
      </AppFrame>
    );

  const markAll = () =>
    mark.mutate(
      { key },
      {
        onSuccess: () => {
          toast.success(t("payroll.marked"));
          setConfirm(false);
        },
        onError: () => toast.error(t("payroll.markFailed")),
      },
    );
  const monthBar = (
    <div className="flex items-center gap-2">
      <Button
        variant="outline"
        size="icon-lg"
        aria-label={t("expense.prev")}
        onClick={() => go(addMonths(month, -1))}
      >
        <ChevronLeft aria-hidden="true" />
      </Button>
      <b className="min-w-[120px] text-center text-[17px]">
        {tf("expense.sub", { m: monthLabel(month) })}
      </b>
      <Button
        variant="outline"
        size="icon-lg"
        aria-label={t("expense.next")}
        onClick={() => go(addMonths(month, 1))}
      >
        <ChevronRight aria-hidden="true" />
      </Button>
    </div>
  );
  const th = "px-3 font-bold";

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("payroll.title")}
          subtitle={
            q.data
              ? tf("payroll.sub", {
                  n: all.length,
                  total: formatVnd(q.data.totalNet),
                  state: unpaid ? t("payroll.unpaidAll") : t("payroll.paidAll"),
                })
              : undefined
          }
          back="/owner/settings"
          right={
            <Button
              variant="outline"
              size="lg"
              disabled={!rows.length}
              onClick={() => downloadCsv(month, rows)}
            >
              <Download aria-hidden="true" />
              <span className="hidden md:inline">{t("payroll.csv")}</span>
              <span className="sr-only md:hidden">{t("payroll.csv")}</span>
            </Button>
          }
        />
        <div className="flex flex-col gap-3 px-5">
          <div className="flex flex-wrap items-center justify-between gap-3">
            {monthBar}
            <ToggleGroup
              type="single"
              value={filter}
              onValueChange={(v) => v && setFilter(v as Filter)}
              aria-label={t("payroll.title")}
              className="hidden flex-wrap gap-2 md:flex"
            >
              {FILTERS.map((f) => (
                <ToggleGroupItem
                  key={f}
                  value={f}
                  className="relative h-11 rounded-full! border border-border bg-card px-4 text-[15px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
                >
                  {filter === f && (
                    <SlidingPill
                      id="pay-pill"
                      className="absolute inset-0 rounded-full bg-primary"
                    />
                  )}
                  <span className="relative whitespace-nowrap">
                    {f === "ALL" ? t("payroll.all") : t(`staff.position.${f}` as MessageKey)}
                  </span>
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          {q.isLoading && <Skeleton className="h-64 rounded-card" aria-busy="true" />}
          {q.data && rows.length === 0 && (
            <EmptyState title={t("payroll.title")} body={t("payroll.empty")} />
          )}

          <FadeIn className="flex flex-col gap-3 md:hidden">
            {rows.map((l) => (
              <Card key={l.userId} className="gap-1.5 p-4 shadow-none">
                <div className="flex items-start justify-between gap-2">
                  <span className="leading-tight">
                    <b className="block text-[17px]">{l.name}</b>
                    <span className="text-[13px] text-ink-2">
                      {t(`staff.position.${l.position}` as MessageKey)}
                    </span>
                  </span>
                  <span
                    className={cn(
                      "rounded-full border px-2.5 py-0.5 text-[12px] font-bold",
                      l.status === "PAID"
                        ? "border-ok-line bg-ok-bg text-ok"
                        : "border-warn-line bg-warn-bg text-warn-ink",
                    )}
                  >
                    {t(l.status === "PAID" ? "payroll.paid" : "payroll.unpaid")}
                  </span>
                </div>
                <dl className="grid grid-cols-2 gap-x-3 gap-y-1 text-[13px]">
                  {[
                    [t("payroll.colContract"), contract(l)],
                    [t("payroll.colShifts"), shifts(l)],
                    [t("payroll.colLeave"), String(l.leaveDays)],
                    [t("payroll.colEarned"), formatVnd(l.earnedPay)],
                    [t("payroll.colAllowance"), formatVnd(l.allowance)],
                    [t("payroll.colNet"), formatVnd(l.net)],
                  ].map(([k, v]) => (
                    <div key={k} className="flex justify-between gap-2">
                      <dt className="text-ink-2">{k}</dt>
                      <dd className="font-bold">{v}</dd>
                    </div>
                  ))}
                </dl>
                <div className="mt-1 flex items-center gap-2">
                  <span className="text-[12px] font-bold text-ink-2">{t("payroll.adjust")}</span>
                  <Adjust line={l} field="bonus" month={month} />
                  <Adjust line={l} field="deduction" month={month} />
                </div>
              </Card>
            ))}
          </FadeIn>

          {rows.length > 0 && (
            <FadeIn className="hidden md:block">
              <Card className="gap-0 overflow-x-auto p-0 shadow-none">
                <table className="w-full min-w-[1180px] text-[14px] [&_td]:whitespace-nowrap">
                  <thead>
                    <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                      <th scope="col" className="sticky left-0 z-10 bg-card px-3 pl-5 font-bold">
                        {t("payroll.colStaff")}
                      </th>
                      {(
                        [
                          "colContract",
                          "colShifts",
                          "colLeave",
                          "colEarned",
                          "colAllowance",
                          "colBonus",
                          "colDeduction",
                          "colNet",
                          "colStatus",
                        ] as const
                      ).map((k) => (
                        <th key={k} scope="col" className={th}>
                          {t(`payroll.${k}`)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((l) => (
                      <tr
                        key={l.userId}
                        className="border-t border-border transition-colors duration-100 hover:bg-sunken/50"
                      >
                        <td className="sticky left-0 z-10 bg-card px-3 py-3 pl-5 leading-tight">
                          <b className="block text-[15px]">{l.name}</b>
                          <span className="text-[12px] text-muted-foreground">
                            {t(`staff.position.${l.position}` as MessageKey)}
                          </span>
                        </td>
                        <td className="px-3">{contract(l)}</td>
                        <td className="px-3">{shifts(l)}</td>
                        <td className="px-3">{l.leaveDays}</td>
                        <td className="px-3">{formatVnd(l.earnedPay)}</td>
                        <td className="px-3">{formatVnd(l.allowance)}</td>
                        <td className="px-3">
                          <Adjust line={l} field="bonus" month={month} />
                        </td>
                        <td className="px-3">
                          <Adjust line={l} field="deduction" month={month} />
                        </td>
                        <td className="px-3 font-bold">{formatVnd(l.net)}</td>
                        <td className="px-3">
                          <span
                            className={cn(
                              "rounded-full border px-2.5 py-0.5 text-[12px] font-bold",
                              l.status === "PAID"
                                ? "border-ok-line bg-ok-bg text-ok"
                                : "border-warn-line bg-warn-bg text-warn-ink",
                            )}
                          >
                            {t(l.status === "PAID" ? "payroll.paid" : "payroll.unpaid")}
                          </span>
                        </td>
                      </tr>
                    ))}
                    <tr className="border-t border-border bg-sunken font-bold">
                      <td className="sticky left-0 z-10 bg-sunken px-3 py-3 pl-5">
                        {tf("payroll.totalRow", { n: rows.length })}
                      </td>
                      <td colSpan={3} />
                      <td className="px-3">{formatVnd(sum((l) => l.earnedPay))}</td>
                      <td className="px-3">{formatVnd(sum((l) => l.allowance))}</td>
                      <td className="px-3">{formatVnd(sum((l) => l.bonus))}</td>
                      <td className="px-3">{formatVnd(sum((l) => l.deduction))}</td>
                      <td className="px-3">{formatVnd(sum((l) => l.net))}</td>
                      <td />
                    </tr>
                  </tbody>
                </table>
              </Card>
              <p className="mt-2 text-[12px] text-muted-foreground">{t("payroll.scroll")}</p>
            </FadeIn>
          )}
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="flex max-w-[640px] items-start gap-2 text-[13px] text-muted-foreground">
              <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
              {t("payroll.foot")}
            </p>
            <Button
              size="lg"
              className="font-bold"
              disabled={!unpaid || mark.isPending}
              onClick={() => setConfirm(true)}
            >
              <Check aria-hidden="true" />
              {t("payroll.markAll")}
            </Button>
          </div>
        </div>
      </main>
      {confirm && (
        <ResponsiveDialog
          open
          onOpenChange={(o) => !o && setConfirm(false)}
          title={t("payroll.markAll")}
          description={tf("payroll.sub", {
            n: unpaid,
            total: formatVnd(q.data?.totalNet ?? 0),
            state: t("payroll.unpaidAll"),
          })}
        >
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button variant="outline" size="lg" onClick={() => setConfirm(false)}>
              {t("expense.cancel")}
            </Button>
            <Button size="lg" disabled={mark.isPending} onClick={markAll}>
              {t("payroll.markAll")}
            </Button>
          </div>
        </ResponsiveDialog>
      )}
    </AppFrame>
  );
}

function downloadCsv(month: string, rows: PayrollLine[]) {
  const cell = (v: string | number) => `"${String(v).replace(/"/g, '""')}"`;
  const lines = rows.map((l) =>
    [
      l.name,
      l.position,
      l.payType,
      l.shiftsWorked,
      l.leaveDays,
      l.earnedPay,
      l.allowance,
      l.bonus,
      l.deduction,
      l.net,
      l.status,
    ]
      .map(cell)
      .join(","),
  );
  const blob = new Blob(["﻿" + lines.join("\n")], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  Object.assign(document.createElement("a"), {
    href: url,
    download: `payroll-${month}.csv`,
  }).click();
  URL.revokeObjectURL(url);
}
