"use client";

import { Receipt } from "lucide-react";
import { useRouter } from "next/navigation";
import { useMutation } from "@tanstack/react-query";
import type { components } from "../../api/generated/schema";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { api, idempotencyHeader, newIdempotencyKey } from "../../lib/api";
import { localized, lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { ResumePayment } from "../rooms/ResumePayment";
import { billLineLabel } from "./labels";

type Stay = components["schemas"]["Stay"];
const row = "flex justify-between gap-3 text-[15px]";

// A stay that is already checked out (opened from an alert, history or a bookmark): a read-only summary.
// Nothing here acts on the stay (no check-out, extras, time edit or move) and nothing is "running": the bill
// is the frozen one and every amount is the API's. Unpaid or refund pending offers the resume action instead.
export function CheckedOutSummary({ stay }: { stay: Stay }) {
  const router = useRouter();
  const q = stay.quote;
  const p = stay.pendingPayment;
  // The invoice id is not on the stay: repeating the (idempotent) check-out returns the same invoice.
  const receipt = useMutation({
    mutationFn: async () => {
      const { data } = await api.POST("/v1/stays/{stayId}/checkout", {
        params: { path: { stayId: stay.id }, header: idempotencyHeader(newIdempotencyKey()) },
      });
      if (!data) throw new Error("checkoutStay failed");
      return data.id;
    },
    onSuccess: (id) => router.push(lp(`/receipt?invoice=${id}`)),
  });

  return (
    <div className="flex flex-col gap-3">
      <Card className="gap-2 p-5 shadow-none">
        <div className="flex items-center justify-between gap-3">
          <p className="text-lg font-bold">
            {stay.checkOutAt
              ? tf("stay.checkedOutAt", { time: formatClock(stay.checkOutAt) })
              : t("stay.checkedOut")}
          </p>
          {!p && (
            <Badge variant="ok" className="px-3 py-1 font-bold">
              {t("pay.paid")}
            </Badge>
          )}
        </div>
        <p className="text-sm text-muted-foreground">
          {stay.guestName} · {stay.guestPhone}
        </p>
      </Card>
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
        {stay.extras.map((x) => (
          <p key={x.serviceCode} className={row}>
            <span>
              {localized(x.name)} × {x.quantity}
            </span>
            <span>{formatVnd(x.amount)}</span>
          </p>
        ))}
        <p className={`${row} border-t border-border pt-2.5 font-bold`}>
          <span>{t("checkout.total")}</span>
          <span>{formatVnd(q.total)}</span>
        </p>
        <p className={`${row} text-ink-2`}>
          <span>{t("checkout.deposit")}</span>
          <span>−{formatVnd(stay.deposit)}</span>
        </p>
      </Card>
      {p ? (
        <ResumePayment stayId={stay.id} roomId={stay.roomId} pending={p} readOnly={false} />
      ) : (
        <Button
          variant="outline"
          size="lg"
          loading={receipt.isPending}
          onClick={() => receipt.mutate()}
        >
          <Receipt aria-hidden="true" />
          {t("stay.viewReceipt")}
        </Button>
      )}
      {receipt.isError && (
        <p role="alert" className="text-sm text-warn">
          {t("receipt.failed")}
        </p>
      )}
    </div>
  );
}
