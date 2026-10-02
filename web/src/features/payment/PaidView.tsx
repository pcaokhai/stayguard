"use client";

import { motion } from "motion/react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useEffect } from "react";
import { SuccessTick } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { localized, lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf, type MessageKey } from "../../lib/t";
import { clockLocale } from "../../lib/time";
import { useBuildings } from "../rooms/hooks";
import { useRoom } from "../stay/hooks";
import { useMe } from "../session/useMe";
import { usePayment } from "./hooks";

const row = "flex justify-between gap-4 text-[15px]";

// The one celebratory moment: the tick draws, the phone buzzes once. The amount never animates.
export function PaidView() {
  const params = useSearchParams();
  const id = params.get("payment");
  const roomId = params.get("room");
  const p = usePayment(id).data;
  const me = useMe().data;
  const room = useRoom(roomId).data;
  const building = useBuildings().data?.find((b) => b.id === room?.buildingId);
  useEffect(() => {
    if (p?.method === "CASH") navigator.vibrate?.(15); // transfers already buzzed on the QR screen
  }, [p?.method]);
  if (!p) return null;
  const paidAt = p.paidAt
    ? new Date(p.paidAt).toLocaleTimeString(clockLocale(), {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : "";
  const roomCode = room?.code;

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-1 flex-col items-center gap-3 px-5 pb-8 pt-20">
        <motion.div
          initial={{ scale: 0.8, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          transition={{ type: "spring", stiffness: 300, damping: 20 }}
          className="flex size-[92px] items-center justify-center rounded-full border-[3px] border-ok bg-ok-bg text-ok"
        >
          <SuccessTick className="size-12" />
        </motion.div>
        <h1 className="text-base font-bold text-ok">{t("pay.paid")}</h1>
        <p className="text-[44px] font-bold leading-none">{formatVnd(p.amount)}</p>
        <p className="text-center text-sm text-ink-2">
          {p.method === "CASH" ? t("pay.cashAt") : t("pay.receivedAt")} {paidAt}
        </p>
        <Card className="mt-2 w-full gap-1.5 p-4 text-muted-foreground shadow-none">
          {p.qr && (
            <p className={row}>
              <span>{t("pay.slip")}</span>
              <span className="text-foreground">{p.qr.transferNote}</span>
            </p>
          )}
          {roomCode && (
            <p className={row}>
              <span>{t("pay.roomLine")}</span>
              <span className="text-foreground">
                {[roomCode, building?.name].filter(Boolean).join(" · ")}
              </span>
            </p>
          )}
          <p className={row}>
            <span>{t("pay.method")}</span>
            <span className="text-foreground">{t(`pay.method${p.method}` as MessageKey)}</span>
          </p>
          {p.transactionId && (
            <p className={row}>
              <span>{t("pay.txn")}</span>
              <span className="min-w-0 break-all text-right text-foreground">
                {p.transactionId}
              </span>
            </p>
          )}
          {me && (
            <p className={row}>
              <span>{t("pay.staff")}</span>
              <span className="text-foreground">{me.user.name}</span>
            </p>
          )}
        </Card>
        {roomCode && (
          <p className="w-full rounded-[10px] bg-dirty-bg p-3.5 text-sm text-dirty">
            {tf("pay.roomToClean", { room: roomCode })} <b>{t("pay.toClean")}</b>.
          </p>
        )}
        <div className="mt-auto flex w-full flex-col gap-3 pt-8 print:hidden">
          <Button asChild size="lg">
            <Link href={lp("/rooms")}>{t("pay.toRooms")}</Link>
          </Button>
          <Button asChild variant="outline" size="lg">
            <Link href={lp(`/receipt?invoice=${p.invoiceId}`)}>{t("pay.print")}</Link>
          </Button>
        </div>
      </main>
    </AppFrame>
  );
}
