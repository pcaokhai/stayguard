"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { ScreenHeader } from "../../components/ScreenHeader";
import { newIdempotencyKey } from "../../lib/api";
import { formatVnd } from "../../lib/money";
import { t } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { useCheckout, useCreatePayment, useStay } from "./hooks";
import { billLineLabel } from "./labels";

const row = "flex justify-between text-base";
const option =
  "flex min-h-16 flex-col justify-center rounded-card bg-surface px-4 py-3 text-left disabled:opacity-60";

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
  if (!invoice) return <p className="p-5 text-muted">{t("checkout.preparing")}</p>;
  const q = invoice.quote;

  const choose = (method: "CASH" | "TRANSFER") =>
    pay.mutate(
      { method, key: (payKeys.current[method] ??= newIdempotencyKey()) },
      {
        onSuccess: (p) =>
          router.push(`/vi/${method === "TRANSFER" ? "pay" : "paid"}?payment=${p.id}`),
      },
    );

  return (
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col">
      <ScreenHeader
        title={`${t("checkout.title")} ${invoice.roomCode}`}
        subtitle={
          stay.data
            ? `${t("stay.since")} ${formatClock(stay.data.checkInAt)}${stay.data.checkOutAt ? ` · ${t("checkout.out")} ${formatClock(stay.data.checkOutAt)}` : ""}`
            : undefined
        }
        back={`/vi/stay?id=${stayId}`}
      />
      <div className="flex flex-col gap-3 px-5 pb-8">
        <section className="flex flex-col gap-2 rounded-card border border-line-soft bg-surface p-5">
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
                {x.name.vi} × {x.quantity}
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
        </section>
        <h2 className="mt-2 font-semibold">{t("checkout.payWith")}</h2>
        <button
          type="button"
          disabled={pay.isPending}
          onClick={() => choose("TRANSFER")}
          className={`${option} border-2 border-brand`}
        >
          <span className="text-lg font-bold">{t("checkout.qr")}</span>
          <span className="text-sm text-muted">{t("checkout.qrSub")}</span>
        </button>
        <button
          type="button"
          disabled={pay.isPending}
          onClick={() => choose("CASH")}
          className={`${option} border border-line`}
        >
          <span className="text-lg font-bold">{t("checkout.cash")}</span>
          <span className="text-sm text-muted">{t("checkout.cashSub")}</span>
        </button>
        {pay.isError && (
          <p role="alert" className="text-sm text-warn">
            {t("checkout.failed")}
          </p>
        )}
      </div>
    </main>
  );
}
