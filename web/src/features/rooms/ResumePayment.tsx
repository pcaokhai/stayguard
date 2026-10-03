"use client";

import { Banknote, QrCode } from "lucide-react";
import { useRouter } from "next/navigation";
import { useMutation } from "@tanstack/react-query";
import type { components } from "../../api/generated/schema";
import { Button } from "@/components/ui/button";
import { api, idempotencyHeader, newIdempotencyKey } from "../../lib/api";
import { lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";

type Pending = components["schemas"]["PendingPayment"];
const row = "flex justify-between gap-3 text-sm";

// Reopens the payment of a checked-out stay. With a transfer payment already there the pay screen
// opens on it; otherwise the invoice (checkout is idempotent) gets a payment through createPayment.
// A fresh Idempotency-Key per user action; amounts are the API's.
function useResume(stayId: string, roomId: string, pending: Pending) {
  const router = useRouter();
  return useMutation({
    mutationFn: async (method: "TRANSFER" | "CASH") => {
      if (method === "TRANSFER" && pending.paymentId) return { id: pending.paymentId, method };
      const co = await api.POST("/v1/stays/{stayId}/checkout", {
        params: { path: { stayId }, header: idempotencyHeader(newIdempotencyKey()) },
      });
      if (!co.data) throw new Error("checkoutStay failed");
      const pay = await api.POST("/v1/invoices/{invoiceId}/payments", {
        params: { path: { invoiceId: co.data.id }, header: idempotencyHeader(newIdempotencyKey()) },
        body: { method },
      });
      if (!pay.data) throw new Error("createPayment failed");
      return { id: pay.data.id, method };
    },
    onSuccess: ({ id, method }) =>
      router.push(lp(`/${method === "TRANSFER" ? "pay" : "paid"}?payment=${id}&room=${roomId}`)),
  });
}

export function ResumePayment({
  stayId,
  roomId,
  pending,
  readOnly,
}: {
  stayId: string;
  roomId: string;
  pending: Pending;
  readOnly: boolean;
}) {
  const resume = useResume(stayId, roomId, pending);
  const refund = pending.remaining === 0 && pending.refundDue > 0;
  const check = pending.remaining === 0 && pending.refundDue === 0;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-2 rounded-xl border border-warn-line bg-warn-bg p-3.5 text-warn-ink">
        <p className={row}>
          <span>{t("panel.pendingTotal")}</span>
          <span>{formatVnd(pending.total)}</span>
        </p>
        <p className={row}>
          <span>{t("pay.receivedSoFar")}</span>
          <span>{formatVnd(pending.received)}</span>
        </p>
        {!check && (
          <p className="flex items-baseline justify-between gap-3 font-bold">
            <span>{refund ? t("panel.refundDue") : t("pay.remaining")}</span>
            <span className="text-[26px] leading-none">
              {formatVnd(refund ? pending.refundDue : pending.remaining)}
            </span>
          </p>
        )}
        {check && <p className="font-semibold">{t("rooms.checkBill")}</p>}
      </div>
      {check && <p className="text-sm text-muted-foreground">{t("panel.checkBillNote")}</p>}
      {!readOnly && !check && (
        <>
          {refund ? (
            <Button size="lg" loading={resume.isPending} onClick={() => resume.mutate("CASH")}>
              <Banknote aria-hidden="true" />
              {tf("checkout.refund", { amount: formatVnd(pending.refundDue) })}
            </Button>
          ) : (
            <>
              <Button
                size="lg"
                loading={resume.isPending && resume.variables === "TRANSFER"}
                onClick={() => resume.mutate("TRANSFER")}
              >
                <QrCode aria-hidden="true" />
                {t("panel.resume")}
              </Button>
              <Button
                variant="outline"
                loading={resume.isPending && resume.variables === "CASH"}
                onClick={() => resume.mutate("CASH")}
              >
                <Banknote aria-hidden="true" />
                {t("panel.resumeCash")}
              </Button>
            </>
          )}
          {resume.isError && (
            <p role="alert" className="text-sm text-warn">
              {t("panel.resumeFailed")}
            </p>
          )}
        </>
      )}
    </div>
  );
}
