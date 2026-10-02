"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { newIdempotencyKey } from "../../lib/api";
import { formatVnd } from "../../lib/money";
import { t } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { useCheckout, useCreatePayment, useStay } from "./hooks";
import { billLineLabel } from "./labels";
import { localized, lp } from "../../lib/locale";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

const row = "flex justify-between text-base";

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
      <p role="alert" className="p-5">
        {t("stay.loadFailed")}
      </p>
    );
  if (!invoice) return <p className="p-5 text-muted-foreground">{t("checkout.preparing")}</p>;
  const q = invoice.quote;

  const choose = (method: "CASH" | "TRANSFER") =>
    pay.mutate(
      { method, key: (payKeys.current[method] ??= newIdempotencyKey()) },
      {
        onSuccess: (p) =>
          router.push(lp(`/${method === "TRANSFER" ? "pay" : "paid"}?payment=${p.id}`)),
      },
    );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-col">
        <TopBar
          title={`${t("checkout.title")} ${invoice.roomCode}`}
          subtitle={
            stay.data
              ? `${t("stay.since")} ${formatClock(stay.data.checkInAt)}${stay.data.checkOutAt ? ` · ${t("checkout.out")} ${formatClock(stay.data.checkOutAt)}` : ""}`
              : undefined
          }
          back={`/stay?id=${stayId}`}
        />
        <div className="flex flex-col gap-3 px-5 pb-8">
          <Card className="gap-2 p-5 shadow-none">
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
            {stay.data?.extras.map((x) => (
              <p key={x.serviceCode} className={row}>
                <span>
                  {localized(x.name)} × {x.quantity}
                </span>
                <span>{formatVnd(x.amount)}</span>
              </p>
            ))}
            <p className={`${row} border-t border-line-soft pt-2`}>
              <span>{t("checkout.total")}</span>
              <span>{formatVnd(q.total)}</span>
            </p>
            <p className={`${row} text-ink-2`}>
              <span>{t("checkout.deposit")}</span>
              <span>−{formatVnd(q.depositPaid)}</span>
            </p>
            <p className="flex items-baseline justify-between font-bold">
              <span>{t("checkout.due")}</span>
              <span className="text-[32px]">{formatVnd(q.balanceDue)}</span>
            </p>
          </Card>
          <h2 className="mt-2 font-semibold">{t("checkout.payWith")}</h2>
          <Button
            type="button"
            variant="outline"
            loading={pay.isPending}
            onClick={() => choose("TRANSFER")}
            className="h-auto min-h-16 flex-col items-start justify-center gap-0 whitespace-normal rounded-xl bg-card px-4 py-3 text-left border-2 border-primary"
          >
            <span className="text-lg font-bold">{t("checkout.qr")}</span>
            <span className="text-sm text-muted-foreground">{t("checkout.qrSub")}</span>
          </Button>
          <Button
            type="button"
            variant="outline"
            loading={pay.isPending}
            onClick={() => choose("CASH")}
            className="h-auto min-h-16 flex-col items-start justify-center gap-0 whitespace-normal rounded-xl bg-card px-4 py-3 text-left border-2 border-primary"
          >
            <span className="text-lg font-bold">{t("checkout.cash")}</span>
            <span className="text-sm text-muted-foreground">{t("checkout.cashSub")}</span>
          </Button>
          {pay.isError && (
            <p role="alert" className="text-sm text-warn">
              {t("checkout.failed")}
            </p>
          )}
        </div>
      </main>
    </AppFrame>
  );
}
