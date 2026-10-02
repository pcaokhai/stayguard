"use client";

import { Banknote, QrCode } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { newIdempotencyKey } from "../../lib/api";
import { localized, lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { formatClock, minutesBetween } from "../../lib/time";
import { FlowSplit } from "../rooms/FlowSplit";
import { useCheckout, useCreatePayment, useStay } from "./hooks";
import { billLineLabel } from "./labels";

const row = "flex justify-between gap-3 text-[15px]";
const option =
  "h-auto min-h-16 flex-col items-start justify-center gap-0 whitespace-normal rounded-xl bg-card px-4 py-3 text-left";

export function CheckoutView() {
  const stayId = useSearchParams().get("stay") ?? "";
  const router = useRouter();
  const stay = useStay(stayId);
  const checkout = useCheckout(stayId);
  const [checkoutKey] = useState(newIdempotencyKey);
  // Idempotency-Key must be a UUID, and the same key with another body is a 409: one key per method.
  const payKeys = useRef<Partial<Record<"CASH" | "TRANSFER", string>>>({});
  const started = useRef(false);
  const invoice = checkout.data;
  const pay = useCreatePayment(invoice?.id ?? "");

  // The server ends the stay and freezes the bill; repeating the request returns the same invoice.
  useEffect(() => {
    if (stayId && !started.current) {
      started.current = true;
      checkout.mutate(checkoutKey);
    }
  }, [stayId, checkout, checkoutKey]);

  if (checkout.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => checkout.mutate(checkoutKey)} />
      </AppFrame>
    );
  const q = invoice?.quote;
  const s = stay.data;
  const mins = s?.checkOutAt ? minutesBetween(s.checkInAt, s.checkOutAt) : undefined;

  const choose = (method: "CASH" | "TRANSFER") =>
    pay.mutate(
      { method, key: (payKeys.current[method] ??= newIdempotencyKey()) },
      {
        onSuccess: (p) =>
          router.push(
            lp(
              `/${method === "TRANSFER" ? "pay" : "paid"}?payment=${p.id}&room=${s?.roomId ?? ""}`,
            ),
          ),
      },
    );

  return (
    <AppFrame tabs={false}>
      <FlowSplit roomId={s?.roomId}>
        <TopBar
          title={`${t("checkout.title")} ${invoice?.roomCode ?? s?.roomCode ?? ""}`}
          subtitle={
            s
              ? [
                  `${t("checkout.in")} ${formatClock(s.checkInAt)}`,
                  s.checkOutAt && `${t("checkout.out")} ${formatClock(s.checkOutAt)}`,
                  mins !== undefined &&
                    tf("stay.hoursMinutes", { h: Math.floor(mins / 60), m: mins % 60 }),
                ]
                  .filter(Boolean)
                  .join(" · ")
              : undefined
          }
          back={`/stay?id=${stayId}`}
        />
        {!invoice || !q ? (
          <div className="px-5">
            <Skeleton className="h-72 w-full" />
            <p className="pt-3 text-sm text-muted-foreground">{t("checkout.preparing")}</p>
          </div>
        ) : (
          <div className="flex flex-col gap-3 px-5 pb-8 lg:pb-0">
            <Card className="gap-2.5 p-5 shadow-none">
              <h2 className="font-bold">{t("checkout.details")}</h2>
              {q.lines.map((l) => (
                <p key={l.code} className={row}>
                  <span>
                    {billLineLabel(l.code)}
                    {l.quantity > 1 ? ` × ${l.quantity}` : ""}
                  </span>
                  <span>{formatVnd(l.amount)}</span>
                </p>
              ))}
              {s?.extras.map((x) => (
                <p key={x.serviceCode} className={row}>
                  <span>
                    {localized(x.name)} × {x.quantity}
                  </span>
                  <span>{formatVnd(x.amount)}</span>
                </p>
              ))}
              <p className={`${row} border-t border-border pt-2.5`}>
                <span>{t("checkout.total")}</span>
                <span>{formatVnd(q.total)}</span>
              </p>
              <p className={`${row} text-ink-2`}>
                <span>{t("checkout.deposit")}</span>
                <span>−{formatVnd(q.depositPaid)}</span>
              </p>
              <p className="flex items-baseline justify-between font-bold">
                <span>{t("checkout.due")}</span>
                <span className="text-[34px] leading-none">{formatVnd(q.balanceDue)}</span>
              </p>
            </Card>
            <h2 className="mt-2 font-semibold">{t("checkout.payWith")}</h2>
            <div className="grid gap-3 lg:grid-cols-2">
              {q.refundDue === 0 && (
                <Button
                  type="button"
                  variant="outline"
                  loading={pay.isPending}
                  disabled={q.balanceDue === 0}
                  onClick={() => choose("TRANSFER")}
                  className={`${option} border-2 border-primary`}
                >
                  <span className="flex items-center gap-2 text-base font-bold">
                    <QrCode aria-hidden="true" />
                    {t("checkout.qr")}
                  </span>
                  <span className="text-[13px] font-normal text-muted-foreground">
                    {t("checkout.qrSub")}
                  </span>
                </Button>
              )}
              <Button
                type="button"
                variant="outline"
                loading={pay.isPending}
                onClick={() => choose("CASH")}
                className={option}
              >
                <span className="flex items-center gap-2 text-base font-bold">
                  <Banknote aria-hidden="true" />
                  {q.refundDue > 0
                    ? tf("checkout.refund", { amount: formatVnd(q.refundDue) })
                    : t("checkout.cash")}
                </span>
                <span className="text-[13px] font-normal text-muted-foreground">
                  {t(q.refundDue > 0 ? "checkout.refundSub" : "checkout.cashSub")}
                </span>
              </Button>
            </div>
            {pay.isError && (
              <p role="alert" className="text-sm text-warn">
                {t("checkout.failed")}
              </p>
            )}
          </div>
        )}
      </FlowSplit>
    </AppFrame>
  );
}
