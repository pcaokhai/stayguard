"use client";

import { AlertTriangle, Banknote, Lock, QrCode, TimerOff } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import QRCode from "qrcode";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import type { components } from "../../api/generated/schema";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { buzz } from "../../lib/haptics";
import { newIdempotencyKey } from "../../lib/api";
import { lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { isDemo } from "../../lib/session";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { cn } from "@/lib/utils";
import { FlowSplit } from "../rooms/FlowSplit";
import { useCreatePayment, useRoom } from "../stay/hooks";
import { usePayment, useSimulatePayment } from "./hooks";

type Payment = components["schemas"]["Payment"];

const noopSubscribe = () => () => {};
const row = "flex justify-between gap-4 text-[15px]";
const PAID_EXIT_MS = 450;

export function PayView() {
  const params = useSearchParams();
  const id = params.get("payment") ?? "";
  const roomId = params.get("room");
  const router = useRouter();
  const payment = usePayment(id);
  const room = useRoom(roomId);
  const p = payment.data;
  const paid = p?.status === "PAID";

  // On PAID the QR shrinks and fades first, then the Paid screen takes over (docs/16 §5).
  useEffect(() => {
    if (!paid) return;
    buzz();
    const timer = setTimeout(
      () => router.replace(lp(`/paid?payment=${id}&room=${roomId ?? ""}`)),
      PAID_EXIT_MS,
    );
    return () => clearTimeout(timer);
  }, [paid, id, roomId, router]);

  // Offline, TanStack pauses the 3 s poll and keeps the last data instead of failing: show the offline state with Retry.
  // Coming back resumes the same GET; nothing here writes, so no second payment is possible.
  if (payment.isError || payment.fetchStatus === "paused")
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void payment.refetch()} />
      </AppFrame>
    );

  return (
    <AppFrame tabs={false}>
      <FlowSplit roomId={roomId}>
        <TopBar
          title={tf("pay.titleRoom", { room: room.data?.code ?? "" })}
          subtitle={p?.qr ? `${t("pay.slip")} ${p.qr.transferNote}` : undefined}
          back="/rooms"
        />
        {!p ? <Skeleton className="mx-5 h-96" /> : <PaymentState p={p} paid={paid} />}
      </FlowSplit>
    </AppFrame>
  );
}

// One screen per payment state. A pending transfer that came back without a QR is treated as expired: never a blank screen.
export function PaymentState({ p, paid }: { p: Payment; paid: boolean }) {
  if (p.status === "EXPIRED" || (p.status === "PENDING" && !p.qr && p.method === "TRANSFER"))
    return <Expired p={p} />;
  if (p.status === "MISMATCH") return <Mismatch p={p} />;
  return <Waiting p={p} paid={paid} />;
}

function Waiting({ p, paid }: { p: Payment; paid: boolean }) {
  const router = useRouter();
  const simulate = useSimulatePayment(p.id);
  const demo = useSyncExternalStore(noopSubscribe, isDemo, () => false);
  const [qrImg, setQrImg] = useState("");
  const payload = p.qr?.payload;
  useEffect(() => {
    if (payload) void QRCode.toDataURL(payload, { margin: 1, width: 280 }).then(setQrImg);
  }, [payload]);
  const qr = p.qr;

  return (
    <div className="flex flex-1 flex-col items-center gap-4 px-5 pb-8 lg:pb-0">
      {/* Amounts on the QR screen never animate (docs/16 §4 rule 4). */}
      <p className="text-[44px] font-bold leading-none">{formatVnd(p.qr?.amount ?? p.amount)}</p>
      <AnimatePresence>
        {qrImg && !paid && (
          <motion.div exit={{ scale: 0.9, opacity: 0 }} transition={{ duration: 0.3 }}>
            {/* eslint-disable-next-line @next/next/no-img-element -- data URL, nothing to optimise */}
            <img
              src={qrImg}
              width={232}
              height={232}
              alt={t("pay.qrAlt")}
              className="rounded-3xl border bg-card p-3"
            />
          </motion.div>
        )}
      </AnimatePresence>
      {(p.receivedAmount ?? 0) > 0 && (
        <Card className="w-full gap-1.5 p-4 shadow-none">
          <p className={row}>
            <span className="text-muted-foreground">{t("pay.receivedSoFar")}</span>
            <span>{formatVnd(p.receivedAmount ?? 0)}</span>
          </p>
          <p className={row}>
            <span className="text-muted-foreground">{t("pay.remaining")}</span>
            <b className="text-warn-ink">{formatVnd(p.remaining)}</b>
          </p>
        </Card>
      )}
      {qr && (
        <Card className="w-full gap-1.5 p-4 text-muted-foreground shadow-none">
          <p className={row}>
            <span>{t("pay.account")}</span>
            <span className="text-foreground">{qr.accountNoMasked}</span>
          </p>
          <p className={row}>
            <span>{t("pay.holder")}</span>
            <span className="text-foreground">{qr.accountName}</span>
          </p>
          <p className={row}>
            <span>{t("pay.note")}</span>
            <b className="text-foreground">{qr.transferNote}</b>
          </p>
        </Card>
      )}
      <p
        role="status"
        className="flex w-full items-center gap-2.5 rounded-[10px] bg-warn-bg p-3.5 font-bold text-warn-ink"
      >
        {/* The indicator breathes (opacity 0.6 <-> 1, 1.6 s). */}
        <motion.span
          aria-hidden="true"
          className="size-2.5 shrink-0 rounded-full bg-warn-ink"
          animate={{ opacity: [0.6, 1, 0.6] }}
          transition={{ duration: 1.6, repeat: Infinity }}
        />
        {t("pay.waiting")}
      </p>
      <p className="text-center text-[13px] text-muted-foreground">{t("pay.autoSwitch")}</p>
      <div className="mt-auto flex w-full flex-col gap-3 pt-4">
        {demo && p.status === "PENDING" && (
          <Button
            type="button"
            variant="dashed"
            size="lg"
            loading={simulate.isPending}
            onClick={() => simulate.mutate()}
            className="border-2 border-primary text-primary"
          >
            {t("pay.simulate")}
          </Button>
        )}
        <Button type="button" variant="ghost" onClick={() => router.back()}>
          {t("pay.change")}
        </Button>
      </div>
    </div>
  );
}

// Starts another payment for the same invoice: one Idempotency-Key per method and attempt.
function useNewPayment(invoiceId: string) {
  const router = useRouter();
  const roomId = useSearchParams().get("room") ?? "";
  const create = useCreatePayment();
  const keys = useRef<Record<string, string>>({});
  const start = (method: "CASH" | "TRANSFER", attempt: string) =>
    create.mutate(
      { invoiceId, method, key: (keys.current[`${method}${attempt}`] ??= newIdempotencyKey()) },
      {
        onSuccess: (n) =>
          router.replace(
            lp(`/${method === "TRANSFER" ? "pay" : "paid"}?payment=${n.id}&room=${roomId}`),
          ),
      },
    );
  return { start, create };
}

function Headline({
  icon: Icon,
  tone,
  title,
  body,
}: {
  icon: typeof Lock;
  tone: string;
  title: string;
  body: string;
}) {
  return (
    <div className="flex flex-col items-center gap-2 text-center">
      <span className={cn("flex size-[72px] items-center justify-center rounded-[20px]", tone)}>
        <Icon className="size-7" aria-hidden="true" />
      </span>
      <h2 className="max-w-[320px] text-[24px] font-bold leading-tight">{title}</h2>
      <p className="max-w-[320px] text-sm text-muted-foreground">{body}</p>
    </div>
  );
}

function Expired({ p }: { p: Payment }) {
  const { start, create } = useNewPayment(p.invoiceId);
  const received = p.receivedAmount ?? 0;
  return (
    <div className="flex flex-1 flex-col gap-4 px-5 pb-8 lg:pb-0">
      <Headline
        icon={TimerOff}
        tone="bg-warn-bg text-warn-ink"
        title={t("pay.expiredTitle")}
        body={
          received > 0
            ? tf("pay.expiredBodyPartial", { amount: formatVnd(received) })
            : t("pay.expiredBody")
        }
      />
      {/* An expired payment carries no remaining amount: with bank money already in, show only what arrived; the new QR asks for the rest. */}
      <Card className="gap-2 p-4 shadow-none">
        <p className={row}>
          <span className="text-muted-foreground">
            {received > 0 ? t("pay.receivedSoFar") : t("pay.due")}
          </span>
          <b>{formatVnd(received > 0 ? received : p.amount)}</b>
        </p>
      </Card>
      <div className="mt-auto flex flex-col gap-2.5">
        <Button size="lg" loading={create.isPending} onClick={() => start("TRANSFER", p.id)}>
          <QrCode aria-hidden="true" />
          {t("pay.newQr")}
        </Button>
        <Button
          size="lg"
          variant="outline"
          loading={create.isPending}
          onClick={() => start("CASH", p.id)}
        >
          <Banknote aria-hidden="true" />
          {t("pay.takeCash")}
        </Button>
        {create.isError && (
          <p role="alert" className="text-sm text-warn">
            {t("pay.actionFailed")}
          </p>
        )}
        <p className="flex gap-2 text-[13px] text-muted-foreground">
          <Lock className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          {t("pay.expiredNote")}
        </p>
      </div>
    </div>
  );
}

// The bank reported less than the bill. Nobody can mark it paid by hand (CLAUDE.md §4 rule 4).
function Mismatch({ p }: { p: Payment }) {
  const { start, create } = useNewPayment(p.invoiceId);
  // received and remaining both come from the API; the browser never subtracts amounts.
  const received = p.receivedAmount ?? 0;
  return (
    <div className="flex flex-1 flex-col gap-4 px-5 pb-8 lg:pb-0">
      <Headline
        icon={AlertTriangle}
        tone="bg-warn-bg text-warn-ink"
        title={t("pay.mismatchTitle")}
        body={t("pay.mismatchBody")}
      />
      <Card className="gap-2.5 p-4 shadow-none">
        <p className={row}>
          <span className="text-muted-foreground">{t("pay.due")}</span>
          <span>{formatVnd(p.amount)}</span>
        </p>
        <p className={row}>
          <span className="text-muted-foreground">
            {p.paidAt
              ? tf("pay.receivedAtTime", { time: formatClock(p.paidAt) })
              : t("pay.receivedAt")}
          </span>
          <span>{formatVnd(received)}</span>
        </p>
        <p className={`${row} border-t border-border pt-2.5`}>
          <span className="text-muted-foreground">{t("pay.remaining")}</span>
          <b className="text-warn-ink">{formatVnd(p.remaining)}</b>
        </p>
        {p.transactionId && (
          <p className={row}>
            <span className="text-muted-foreground">{t("pay.txnId")}</span>
            <span>{p.transactionId}</span>
          </p>
        )}
      </Card>
      <Button size="lg" loading={create.isPending} onClick={() => start("TRANSFER", p.id)}>
        <QrCode aria-hidden="true" />
        {tf("pay.qrRemaining", { amount: formatVnd(p.remaining) })}
      </Button>
      <Button
        size="lg"
        variant="outline"
        loading={create.isPending}
        onClick={() => start("CASH", p.id)}
      >
        <Banknote aria-hidden="true" />
        {tf("pay.cashRemaining", { amount: formatVnd(p.remaining) })}
      </Button>
      {create.isError && (
        <p role="alert" className="text-sm text-warn">
          {t("pay.actionFailed")}
        </p>
      )}
      <p className="mt-auto flex items-start gap-2 rounded-xl border border-info-line bg-info-bg p-3.5 text-[13px] text-info">
        <Lock className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
        {t("pay.mismatchNote")}
      </p>
    </div>
  );
}
