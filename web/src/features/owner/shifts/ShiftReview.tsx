"use client";

import type { ReactNode } from "react";
import { FadeIn } from "@/components/motion";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Download } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { clockOf } from "../format";
import { useShiftReview } from "./hooks";
import { floatLine, handoverLine, differenceTone, shiftCsv } from "./labels";

const heading = "text-[13px] font-bold uppercase tracking-wide text-ink-2";
const Row = ({ k, v, strong }: { k: string; v: ReactNode; strong?: boolean }) => (
  <div
    className={cn(
      "flex justify-between gap-3 py-1",
      strong && "border-t border-border pt-2 font-bold",
    )}
  >
    <dt className={strong ? undefined : "text-ink-2"}>{k}</dt>
    <dd>{v}</dd>
  </div>
);

// Reconciliation of one closed shift (boards DoiSoatCa, DoiSoatCaPC). Figures are the closed shift's, never recomputed here.
export function ShiftReview({ id }: { id: string }) {
  const q = useShiftReview(id);
  if (q.isError) return <QueryError onRetry={() => void q.refetch()} />;
  const r = q.data;
  if (!r) return <Skeleton className="h-72 rounded-card" aria-busy="true" />;
  const d = r.difference;
  const hint = d < 0 ? "shortHint" : d > 0 ? "overHint" : "okHint";
  const signed = (n: number) => (n < 0 ? `−${formatVnd(-n)}` : formatVnd(n));

  return (
    <FadeIn key={id} className="flex flex-col gap-3">
      <Card className={cn("gap-1 border p-5 shadow-none lg:hidden", differenceTone(d))}>
        <p className="text-[13px] font-bold">{t("shifts.result")}</p>
        <p className="text-[28px] font-bold leading-tight">
          {d < 0
            ? tf("shifts.short", { amount: formatVnd(-d) })
            : d > 0
              ? tf("shifts.over", { amount: formatVnd(d) })
              : t("shifts.ok")}
        </p>
        <p className="text-[13px]">{t(`shifts.${hint}` as MessageKey)}</p>
      </Card>

      <dl className="hidden gap-3 lg:grid lg:grid-cols-3">
        {[
          ["shifts.expected", formatVnd(r.shift.expectedCash), ""],
          ["shifts.counted", formatVnd(r.countedCash), ""],
          ["shifts.difference", signed(d), d < 0 ? "text-destructive" : ""],
        ].map(([k, v, tone]) => (
          <Card key={k} className="gap-1 p-5 shadow-none">
            <dt className={heading}>{t(k as MessageKey)}</dt>
            <dd className={cn("text-[28px] font-bold leading-tight", tone)}>{v}</dd>
          </Card>
        ))}
      </dl>
      <Card className="gap-1 p-5 shadow-none lg:hidden">
        <dl className="text-[15px]">
          <Row k={t("shifts.expected")} v={formatVnd(r.shift.expectedCash)} />
          <Row k={t("shifts.counted")} v={formatVnd(r.countedCash)} />
          <Row k={t("shifts.difference")} v={signed(d)} strong />
        </dl>
      </Card>

      <Card className="gap-2 p-5 shadow-none">
        <p className="text-[15px] font-bold">{handoverLine(r)}</p>
        <p className="text-[15px]">{floatLine(r)}</p>
        <Button
          variant="outline"
          size="lg"
          className="self-start"
          onClick={() => downloadShiftCsv(r)}
        >
          <Download aria-hidden="true" />
          {t("shifts.csv")}
        </Button>
      </Card>

      {r.reason && (
        <Card className="gap-2 p-5 shadow-none">
          <h2 className="text-[17px] font-bold lg:text-[13px] lg:uppercase lg:tracking-wide lg:text-ink-2">
            {t("shifts.reason")}
          </h2>
          <p className="text-[15px]">“{r.reason}”</p>
          {r.reasonRecordedAt && (
            <p className="text-[13px] text-muted-foreground">
              {r.shift.userName} · {tf("shifts.reasonAt", { time: clockOf(r.reasonRecordedAt) })}
            </p>
          )}
        </Card>
      )}

      <Card className="gap-1 p-5 shadow-none">
        <h2 className="text-[17px] font-bold lg:text-[13px] lg:uppercase lg:tracking-wide lg:text-ink-2">
          {t("shifts.cashBills")}
        </h2>
        <table className="w-full text-[14px]">
          <thead className="hidden lg:table-header-group">
            <tr className="h-9 text-left text-[12px] uppercase tracking-wide text-ink-2">
              {(["room", "type", "timeCol", "cash"] as const).map((k) => (
                <th key={k} scope="col" className={cn("font-bold", k === "cash" && "text-right")}>
                  {t(`shifts.${k}`)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {r.cashPayments.map((p, i) => (
              <tr
                key={`${p.roomCode}-${i}`}
                className="h-9 border-t border-border/60 first:border-t-0 lg:h-10"
              >
                <td className="font-medium">
                  {p.roomCode}
                  <span className="text-ink-2 lg:hidden">
                    {" "}
                    · {t(`shifts.rental.${p.rentalType}` as MessageKey)} · {clockOf(p.at)}
                  </span>
                </td>
                <td className="hidden lg:table-cell">
                  {t(`shifts.rental.${p.rentalType}` as MessageKey)}
                </td>
                <td className="hidden lg:table-cell">{clockOf(p.at)}</td>
                <td className="text-right">{formatVnd(p.amount)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="text-[13px] text-muted-foreground">
          {tf("shifts.cashTotal", { amount: formatVnd(r.shift.cashIn) })}
        </p>
      </Card>

      <Card className="gap-1 bg-sunken p-4 shadow-none lg:hidden">
        <h2 className="text-[14px] font-bold">{t("shifts.history")}</h2>
        <p className="text-[14px]">
          {tf("shifts.historyText", {
            n: r.staffHistory.shiftsWithDifference,
            amount: formatVnd(r.staffHistory.totalShort),
          })}
        </p>
      </Card>
    </FadeIn>
  );
}

function downloadShiftCsv(r: NonNullable<ReturnType<typeof useShiftReview>["data"]>) {
  const blob = new Blob(["\uFEFF" + shiftCsv(r)], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  Object.assign(document.createElement("a"), {
    href: url,
    download: `shift-${r.shift.id}.csv`,
  }).click();
  URL.revokeObjectURL(url);
}
