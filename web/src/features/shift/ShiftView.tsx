"use client";

import { Minus, Plus } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { AppFrame } from "@/components/shell/AppFrame";
import { homeFor } from "../auth/home";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { newIdempotencyKey } from "../../lib/api";
import { lp } from "../../lib/locale";
import { formatVnd, parseVnd, vndNumber } from "../../lib/money";
import { useMe } from "../session/useMe";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { useCloseShift, useCurrentShift } from "./hooks";
import { ShiftLine } from "./ShiftLine";

const DENOMINATIONS = [500000, 200000, 100000, 50000, 20000, 10000] as const;
const row = "flex justify-between gap-3 text-[15px]";

// Boards 11 and PC "Kết ca": the app adds up the counted notes (a sum of what the person counted);
// the expected cash and the final difference are the server's (shift ledger).
export function ShiftView() {
  const router = useRouter();
  const shift = useCurrentShift();
  const close = useCloseShift();
  const [key] = useState(newIdempotencyKey);
  const [qty, setQty] = useState<Record<number, number>>({});
  const [reason, setReason] = useState("");
  const [floatLeft, setFloatLeft] = useState<string>();
  const s = shift.data;
  const user = useMe().data?.user;
  const by = (...kinds: string[]) => (s?.movements ?? []).filter((m) => kinds.includes(m.kind));

  if (shift.isError)
    return (
      <AppFrame>
        <QueryError onRetry={() => void shift.refetch()} />
      </AppFrame>
    );
  if (s === null)
    return (
      <AppFrame>
        <EmptyState title={t("shift.noShift")} body="" />
      </AppFrame>
    );

  const counted = DENOMINATIONS.reduce((sum, d) => sum + d * (qty[d] ?? 0), 0);
  const diff = s ? counted - s.expectedCash : 0;
  const unpaid = s?.unpaidInvoices ?? [];
  const needsReason = diff !== 0 || unpaid.length > 0; // the server asks for a reason while invoices are not fully paid
  const floatValue = floatLeft === undefined ? (s?.openingFloat ?? 0) : parseVnd(floatLeft);
  const ready = !!s && (!needsReason || reason.trim().length >= 3);
  const step = (d: number, by: number) =>
    setQty((q) => ({ ...q, [d]: Math.max(0, (q[d] ?? 0) + by) }));

  const submit = () =>
    s &&
    close.mutate(
      {
        key,
        body: {
          counts: DENOMINATIONS.map((denomination) => ({
            denomination,
            quantity: qty[denomination] ?? 0,
          })),
          floatLeft: floatValue,
          reason: needsReason ? reason.trim() : null,
        },
      },
      {
        onSuccess: () => {
          toast.success(t("shift.closed"));
          router.replace(lp(homeFor(user?.role ?? "RECEPTIONIST")));
        },
      },
    );

  return (
    <AppFrame>
      <main className="mx-auto flex w-full max-w-[1000px] flex-1 flex-col">
        <TopBar
          title={t("shift.title")}
          subtitle={
            s ? tf("shift.sub", { name: s.userName, time: formatClock(s.openedAt) }) : undefined
          }
        />
        {!s ? (
          <Skeleton className="mx-5 h-80" />
        ) : (
          <div className="flex flex-1 flex-col gap-3 px-5 pb-8 lg:grid lg:grid-cols-2 lg:grid-rows-[auto_1fr] lg:items-start lg:gap-5">
            <Card className="gap-2.5 p-5 shadow-none lg:col-start-1 lg:row-start-1">
              <h2 className="font-bold">{t("shift.expects")}</h2>
              <ShiftLine
                label={t("shift.float")}
                value={formatVnd(s.openingFloat)}
                movements={by("OPENING_FLOAT")}
              />
              <ShiftLine
                label={t("shift.cashIn")}
                value={formatVnd(s.cashIn)}
                movements={by("DEPOSIT", "PAYMENT")}
              />
              <ShiftLine
                label={t("shift.cashOut")}
                value={`−${formatVnd(s.cashOut)}`}
                movements={by("REFUND", "PAYOUT")}
              />
              <Link
                href={lp("/shift/payout")}
                className="w-fit py-1 text-sm font-bold text-primary underline"
              >
                + {t("shift.recordPayout")}
              </Link>
              <p className="flex items-baseline justify-between border-t border-border pt-3 font-bold">
                <span>{t("shift.expected")}</span>
                <span className="text-[30px] leading-none">{formatVnd(s.expectedCash)}</span>
              </p>
              <p className="text-[13px] text-muted-foreground">
                {tf("shift.transfers", { amount: formatVnd(s.transfersReceived) })}
              </p>
            </Card>
            <Card className="gap-1 p-5 shadow-none lg:col-start-2 lg:row-span-2 lg:row-start-1">
              <h2 className="font-bold">{t("shift.counted")}</h2>
              <p className="pb-2 text-[13px] text-muted-foreground">{t("shift.countHint")}</p>
              <ul className="flex flex-col gap-2.5">
                {DENOMINATIONS.map((d) => (
                  <li
                    key={d}
                    className="grid grid-cols-[1fr_auto_auto_auto_1fr] items-center gap-2"
                  >
                    <b className="text-[15px]">{formatVnd(d)}</b>
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      aria-label={`${t("shift.less")} ${formatVnd(d)}`}
                      onClick={() => step(d, -1)}
                    >
                      <Minus />
                    </Button>
                    <span className="w-6 text-center text-lg font-bold">{qty[d] ?? 0}</span>
                    <Button
                      type="button"
                      variant="secondary"
                      size="icon"
                      aria-label={`${t("shift.more")} ${formatVnd(d)}`}
                      onClick={() => step(d, 1)}
                    >
                      <Plus />
                    </Button>
                    <span className="text-right text-sm text-muted-foreground">
                      {formatVnd(d * (qty[d] ?? 0))}
                    </span>
                  </li>
                ))}
              </ul>
              <p className="mt-2 flex items-baseline justify-between border-t border-border pt-3 font-bold">
                <span>{t("shift.countedTotal")}</span>
                <span className="text-[26px] leading-none">{formatVnd(counted)}</span>
              </p>
            </Card>
            {/* On desktop the difference sits under the expected-cash card, beside the count: no empty gap. */}
            <div className="flex flex-col gap-3 lg:col-start-1 lg:row-start-2">
              <div
                role="status"
                className={`rounded-xl border p-3.5 ${
                  diff === 0
                    ? "border-ok-line bg-ok-bg text-ok"
                    : "border-warn-line bg-warn-bg text-warn-ink"
                }`}
              >
                <p className="text-xs font-bold uppercase tracking-wide">{t("shift.difference")}</p>
                <p className="text-2xl font-bold">
                  {diff === 0
                    ? t("shift.match")
                    : tf(diff < 0 ? "shift.short" : "shift.over", {
                        amount: formatVnd(Math.abs(diff)),
                      })}
                </p>
              </div>
              {unpaid.length > 0 && (
                <div
                  role="status"
                  className="rounded-xl border border-warn-line bg-warn-bg p-3.5 text-warn-ink"
                >
                  <p className="text-xs font-bold uppercase tracking-wide">
                    {t("shift.unpaidTitle")}
                  </p>
                  <ul className="mt-1 flex flex-col gap-0.5 text-sm font-semibold">
                    {unpaid.map((i) => (
                      <li key={i.invoiceId}>
                        {i.balance === 0 && (i.refundDue ?? 0) > 0
                          ? tf("shift.refundPendingRow", {
                              room: i.roomCode,
                              bill: i.billCode,
                              amount: formatVnd(i.refundDue ?? 0),
                            })
                          : tf("shift.unpaidRow", {
                              room: i.roomCode,
                              bill: i.billCode,
                              amount: formatVnd(i.balance),
                            })}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
            <div className="flex flex-col gap-3 lg:col-span-2 lg:mx-auto lg:w-full lg:max-w-[560px]">
              <label className="flex flex-col gap-1.5 text-sm font-bold">
                {t("shift.reason")}
                <Textarea
                  rows={3}
                  maxLength={500}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  aria-invalid={needsReason && reason.trim().length < 3}
                  className="rounded-[10px] bg-card px-4 text-base font-normal"
                />
              </label>
              <label className="flex flex-col gap-1.5 text-sm font-bold">
                {t("shift.floatLeft")}
                <Input
                  inputMode="numeric"
                  value={floatLeft ?? vndNumber(s.openingFloat)}
                  onChange={(e) => setFloatLeft(e.target.value)}
                  className="h-12 rounded-[10px] bg-card px-4 text-base font-normal"
                />
              </label>
              <p className="text-[13px] text-muted-foreground">{t("shift.lockNote")}</p>
              {needsReason && !ready && (
                <p role="alert" className="text-sm text-warn">
                  {t("shift.reasonRequired")}
                </p>
              )}
              {close.isError && (
                <p role="alert" className="text-sm font-semibold text-warn">
                  {t("shift.failed")}
                </p>
              )}
              <Button size="lg" disabled={!ready} loading={close.isPending} onClick={submit}>
                {t("shift.submit")}
              </Button>
            </div>
          </div>
        )}
      </main>
    </AppFrame>
  );
}
