"use client";

import { AppFrame } from "@/components/shell/AppFrame";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { formatVnd } from "../../lib/money";
import { t, type MessageKey } from "../../lib/t";
import { useMe } from "../session/useMe";
import { usePayment } from "./hooks";
import { lp } from "../../lib/locale";
import { Button } from "@/components/ui/button";

const row = "flex justify-between gap-4";

export function PaidView() {
  const id = useSearchParams().get("payment");
  const p = usePayment(id).data;
  const me = useMe().data;
  if (!p) return null;
  const paidAt = p.paidAt
    ? new Date(p.paidAt).toLocaleTimeString("vi-VN", {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : "";

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-col items-center gap-3 px-5 pb-8 pt-24">
        <div className="flex size-24 items-center justify-center rounded-full border-[3px] border-ok bg-ok-bg text-ok">
          <svg
            width="44"
            height="44"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="3"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="M5 12l5 5 9-10" />
          </svg>
        </div>
        <h1 className="text-xl font-bold text-ok">{t("pay.paid")}</h1>
        <p className="text-[44px] font-bold leading-none">{formatVnd(p.amount)}</p>
        <p className="text-ink-2">
          {p.method === "CASH" ? t("pay.cashAt") : t("pay.receivedAt")} {paidAt}
        </p>
        <dl className="mt-3 flex w-full flex-col gap-1.5 rounded-card border border-line-soft bg-surface p-4 text-muted-foreground">
          <div className={row}>
            <dt>{t("pay.method")}</dt>
            <dd className="text-ink">{t(`pay.method${p.method}` as MessageKey)}</dd>
          </div>
          {p.qr && (
            <div className={row}>
              <dt>{t("pay.slip")}</dt>
              <dd className="text-ink">{p.qr.transferNote}</dd>
            </div>
          )}
          {p.transactionId && (
            <div className={row}>
              <dt>{t("pay.txn")}</dt>
              <dd className="min-w-0 break-all text-right text-ink">{p.transactionId}</dd>
            </div>
          )}
          {me && (
            <div className={row}>
              <dt>{t("pay.staff")}</dt>
              <dd className="text-ink">{me.user.name}</dd>
            </div>
          )}
        </dl>
        <p className="w-full rounded-[10px] bg-dirty-bg p-3.5 text-dirty">
          {t("pay.roomToClean")} <b>{t("pay.toClean")}</b>.
        </p>
        <div className="mt-auto flex w-full flex-col gap-3 pt-8 print:hidden">
          <Button asChild size="lg">
            <Link href={lp("/rooms")}>{t("pay.toRooms")}</Link>
          </Button>
          <Button type="button" variant="outline" size="lg" onClick={() => window.print()}>
            {t("pay.print")}
          </Button>
        </div>
      </main>
    </AppFrame>
  );
}
