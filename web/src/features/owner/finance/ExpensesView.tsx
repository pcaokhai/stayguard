"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { motion } from "motion/react";
import { BarChart3, ChevronLeft, ChevronRight, Info, Plus } from "lucide-react";
import { FadeIn } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, QueryError } from "@/components/StateView";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { duration, ease } from "@/lib/motion";
import { lp } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { AUTO, ExpenseSheet } from "./ExpenseSheet";
import { type Expense, type Source, useExpenseMonth } from "./hooks";
import { addMonths, isMonth, monthLabel, thisMonth } from "./month";

const SourcePill = ({ s }: { s: Source }) => (
  <span
    className={cn(
      "inline-block rounded-full border px-2.5 py-0.5 text-[12px] font-bold whitespace-nowrap",
      AUTO.has(s) ? "border-info-line bg-info-bg text-info" : "border-line bg-maint-bg text-maint",
    )}
  >
    {t(`expense.src.${s}` as MessageKey)}
  </span>
);
const catName = (c: string) => t(`expense.cat.${c}` as MessageKey);
const pct = (n: number, total: number) => (total > 0 ? Math.round((n / total) * 1000) / 10 : 0);

export function ExpensesView() {
  const router = useRouter();
  const params = useSearchParams();
  const month = isMonth(params.get("month")) ? params.get("month")! : thisMonth();
  const q = useExpenseMonth(month);
  const [sheet, setSheet] = useState<{ expense?: Expense } | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const go = (m: string) => router.replace(lp(`/owner/expenses?month=${m}`));
  const d = q.data;

  if (q.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void q.refetch()} />
      </AppFrame>
    );

  const profit = d ? d.revenue - d.total : 0;
  const cats = [...(d?.categories ?? [])].sort((a, b) => b.amount - a.amount);
  const max = cats[0]?.amount ?? 1;
  const items = (d?.items ?? []).filter((x) => x.category === open);
  const add = (
    <Button size="lg" className="w-full md:w-auto" onClick={() => setSheet({})}>
      <Plus aria-hidden="true" />
      {t("expense.add")}
    </Button>
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

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("expense.title")}
          subtitle={tf("expense.sub", { m: monthLabel(month) })}
          back="/owner/settings"
          right={
            <span className="hidden gap-2 md:flex">
              <Button asChild variant="outline" size="lg" className="font-bold">
                <Link href={lp("/owner/reports")}>
                  <BarChart3 aria-hidden="true" />
                  {t("expense.report")}
                </Link>
              </Button>
              {add}
            </span>
          }
        />
        <div className="flex flex-col gap-3 px-5">
          <div className="flex flex-wrap items-center justify-between gap-3 max-md:hidden">
            {monthBar}
            {d && (
              <p className="text-[14px]">
                {tf("expense.totalLine", {
                  total: formatVnd(d.total),
                  revenue: formatVnd(d.revenue),
                })}
              </p>
            )}
          </div>
          <div className="flex items-center justify-between md:hidden">{monthBar}</div>
          {q.isLoading && <Skeleton className="h-64 rounded-card" aria-busy="true" />}
          {d && (
            <>
              <Card className="gap-1 p-4 shadow-none md:hidden">
                <dl className="text-[15px]">
                  <div className="flex justify-between py-0.5">
                    <dt className="text-ink-2">{t("expense.total")}</dt>
                    <dd className="font-bold">{formatVnd(d.total)}</dd>
                  </div>
                  <div className="flex justify-between py-0.5">
                    <dt className="text-ink-2">{t("expense.revenue")}</dt>
                    <dd>{formatVnd(d.revenue)}</dd>
                  </div>
                  <div className="flex justify-between py-0.5">
                    <dt className="text-ink-2">{t("expense.profit")}</dt>
                    <dd className={cn("font-bold", profit >= 0 ? "text-ok" : "text-destructive")}>
                      {formatVnd(profit)}
                    </dd>
                  </div>
                </dl>
              </Card>
              {cats.length === 0 && (
                <EmptyState title={t("expense.title")} body={t("expense.empty")} />
              )}
              <FadeIn>
                <Card className="gap-0 overflow-x-auto p-0 shadow-none">
                  <table className="w-full text-[14px]">
                    <thead className="max-md:hidden">
                      <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                        {(["colCat", "colSource", "colShare", "colAmount", "colPct"] as const).map(
                          (k) => (
                            <th key={k} scope="col" className="px-3 font-bold first:pl-5">
                              {t(`expense.${k}`)}
                            </th>
                          ),
                        )}
                      </tr>
                    </thead>
                    <tbody>
                      {cats.map((c) => (
                        <tr
                          key={c.category}
                          onClick={() => setOpen(c.category)}
                          className="cursor-pointer border-t border-border transition-colors duration-100 first:border-t-0 hover:bg-sunken/50 max-md:flex max-md:items-center max-md:gap-2 max-md:px-4 max-md:py-3"
                        >
                          <td className="px-3 py-3 pl-5 text-[15px] font-bold max-md:flex-1 max-md:p-0">
                            <button
                              type="button"
                              className="text-left"
                              onClick={() => setOpen(c.category)}
                            >
                              {catName(c.category)}
                            </button>
                            <span className="mt-1 block md:hidden">
                              <SourcePill s={c.source} />
                            </span>
                          </td>
                          <td className="px-3 max-md:hidden">
                            <SourcePill s={c.source} />
                          </td>
                          <td className="w-[34%] px-3 max-md:hidden">
                            <span className="block h-2 overflow-hidden rounded-full bg-sunken">
                              <motion.span
                                className="block h-full origin-left rounded-full bg-primary"
                                style={{ width: `${(c.amount / max) * 100}%` }}
                                initial={{ scaleX: 0 }}
                                animate={{ scaleX: 1 }}
                                transition={{ duration: duration.slow, ease: ease.standard }}
                              />
                            </span>
                          </td>
                          <td className="px-3 text-right font-bold max-md:p-0 md:text-left">
                            {formatVnd(c.amount)}
                          </td>
                          <td className="px-3 pr-5 text-ink-2 max-md:hidden">
                            {pct(c.amount, d.total)}%
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </Card>
              </FadeIn>
              <p className="hidden items-start gap-2 text-[13px] text-muted-foreground md:flex">
                <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
                {t("expense.foot")}
              </p>
              <div className="md:hidden">{add}</div>
            </>
          )}
        </div>
      </main>
      {open && (
        <ResponsiveDialog open onOpenChange={(o) => !o && setOpen(null)} title={catName(open)}>
          <ul className="flex flex-col">
            {items.map((x) => (
              <li
                key={x.id}
                className="flex min-h-12 items-center gap-2 border-t border-border py-2 first:border-t-0"
              >
                <span className="min-w-0 flex-1 leading-snug">
                  <b className="block">{formatVnd(x.amount)}</b>
                  <span className="text-[12px] text-muted-foreground">{x.note ?? ""}</span>
                </span>
                <SourcePill s={x.source} />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setOpen(null);
                    setSheet({ expense: x });
                  }}
                >
                  {t("maint.handle")}
                </Button>
              </li>
            ))}
          </ul>
        </ResponsiveDialog>
      )}
      {sheet && (
        <ExpenseSheet
          key={sheet.expense?.id ?? "new"}
          month={month}
          expense={sheet.expense}
          onClose={() => setSheet(null)}
        />
      )}
    </AppFrame>
  );
}
