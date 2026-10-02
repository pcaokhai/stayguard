"use client";

import { useRef, useState } from "react";
import { toast } from "sonner";
import type { components } from "@/api/generated/schema";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Skeleton } from "@/components/ui/skeleton";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { newIdempotencyKey } from "@/lib/api";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { clockOf } from "../format";
import { useLinkTransfer, useUnpaidBills } from "./hooks";

type Transaction = components["schemas"]["Transaction"];

// Bottom sheet on phones, dialog from 640 px (boards GanPhieu, GanPhieuPC).
export function LinkSheet({ tx, onClose }: { tx: Transaction | null; onClose: () => void }) {
  const wide = useMediaQuery("(min-width: 640px)");
  const bills = useUnpaidBills(!!tx);
  const link = useLinkTransfer();
  const [picked, setPicked] = useState<string | null>(null);
  const keys = useRef(new Map<string, string>());
  if (!tx) return null;

  const list = bills.data ?? [];
  const chosen = list.find((b) => b.invoiceId && b.invoiceId === (picked ?? firstPick(list)));
  const submit = () => {
    if (!chosen?.invoiceId || !tx.paymentEventId) return;
    const action = `${tx.paymentEventId}:${chosen.invoiceId}`;
    const key = keys.current.get(action) ?? newIdempotencyKey();
    keys.current.set(action, key);
    link.mutate(
      { eventId: tx.paymentEventId, invoiceId: chosen.invoiceId, key },
      {
        onSuccess: () => {
          toast.success(tf("money.linked", { bill: chosen.billCode ?? "" }));
          onClose();
        },
        onError: (e) => {
          const status = (e as { status?: number }).status;
          toast.error(
            status === 409
              ? t("money.alreadyLinked")
              : status === 422
                ? t("money.amountMismatch")
                : t("money.linkFailed"),
          );
        },
      },
    );
  };

  return (
    <ResponsiveDialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={
        wide ? t("money.linkTitle") : tf("money.linkTitlePhone", { amount: formatVnd(tx.amount) })
      }
      description={tf("money.received", { time: clockOf(tx.at), note: tx.transferNote ?? "" })}
    >
      <dl className="grid gap-1 rounded-card bg-sunken p-3.5 text-[14px] max-sm:hidden">
        <Row k={t("money.receivedAmount")} v={<b>{formatVnd(tx.amount)}</b>} />
        <Row k={t("money.receivedAt")} v={`${clockOf(tx.at)} · SePay`} />
        <Row k={t("money.receivedNote")} v={`“${tx.transferNote ?? ""}”`} />
      </dl>
      <p className="text-[13px] font-bold uppercase tracking-wide text-ink-2 max-sm:hidden">
        {t("money.pick")}
      </p>
      {bills.isLoading && <Skeleton className="h-16 rounded-card" />}
      {!bills.isLoading && list.length === 0 && (
        <p className="py-2 text-sm text-muted-foreground">{t("money.noCandidates")}</p>
      )}
      <RadioGroup
        value={chosen?.invoiceId ?? ""}
        onValueChange={setPicked}
        aria-label={t("money.pick")}
        className="gap-2.5"
      >
        {list.map((b, i) => {
          const same = b.total === tx.amount;
          return (
            <label
              key={b.invoiceId ?? `${b.roomCode}-${i}`}
              className="flex min-h-14 cursor-pointer items-center gap-3 rounded-card border border-border p-3 has-[:checked]:border-2 has-[:checked]:border-primary has-disabled:opacity-50"
            >
              <RadioGroupItem value={b.invoiceId ?? ""} disabled={!b.invoiceId} />
              <span className="min-w-0 flex-1">
                <b className="block font-mono text-[13px]">{b.billCode ?? "—"}</b>
                <span className="text-[13px] text-muted-foreground">
                  {tf("money.outAt", {
                    room: b.roomCode,
                    time: b.checkOutAt ? clockOf(b.checkOutAt) : "",
                  })}
                </span>
              </span>
              {b.total != null && <b className="text-[16px]">{formatVnd(b.total)}</b>}
              <Badge variant={same ? "ok" : "warn"} className="max-sm:hidden">
                {same ? t("money.sameAmount") : t("money.otherAmount")}
              </Badge>
            </label>
          );
        })}
      </RadioGroup>
      <p className="rounded-card border border-info-line bg-info-bg p-3 text-[13px] text-info">
        {t("money.linkWarn")}
      </p>
      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="outline" size="lg" onClick={onClose}>
          {t("money.cancel")}
        </Button>
        <Button size="lg" disabled={!chosen || link.isPending} onClick={submit}>
          {chosen ? tf("money.confirm", { bill: chosen.billCode ?? "" }) : t("money.link")}
        </Button>
      </div>
    </ResponsiveDialog>
  );
}

const firstPick = (list: { invoiceId?: string }[]) => list.find((b) => b.invoiceId)?.invoiceId;

function Row({ k, v }: { k: string; v: React.ReactNode }) {
  return (
    <div className="flex justify-between gap-3">
      <dt className="text-muted-foreground">{k}</dt>
      <dd>{v}</dd>
    </div>
  );
}
