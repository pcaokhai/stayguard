"use client";

import QRCode from "qrcode";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState, useSyncExternalStore } from "react";
import { ScreenHeader } from "../../components/ScreenHeader";
import { formatVnd } from "../../lib/money";
import { isDemo } from "../../lib/session";
import { t } from "../../lib/t";
import { usePayment, useSimulatePayment } from "./hooks";

const noopSubscribe = () => () => {};
const row = "flex justify-between gap-4";

export function PayView() {
  const id = useSearchParams().get("payment") ?? "";
  const router = useRouter();
  const payment = usePayment(id);
  const simulate = useSimulatePayment(id);
  const [qrImg, setQrImg] = useState("");
  const demo = useSyncExternalStore(noopSubscribe, isDemo, () => false);
  const p = payment.data;
  const payload = p?.qr?.payload;

  useEffect(() => {
    if (payload) QRCode.toDataURL(payload, { margin: 1, width: 280 }).then(setQrImg);
  }, [payload]);
  useEffect(() => {
    if (p?.status === "PAID") router.replace(`/vi/paid?payment=${id}`);
  }, [p?.status, id, router]);

  if (payment.isError)
    return (
      <p role="alert" className="p-5">
        {t("pay.failed")}
      </p>
    );
  if (!p) return null;
  const qr = p.qr;

  return (
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col">
      <ScreenHeader
        title={t("pay.title")}
        subtitle={qr ? `${t("pay.slip")} ${qr.transferNote}` : undefined}
        back="/vi/rooms"
      />
      <div className="flex flex-1 flex-col items-center gap-4 px-5 pb-8">
        <p className="text-[44px] font-bold leading-none">{formatVnd(p.amount)}</p>
        {qrImg && (
          // eslint-disable-next-line @next/next/no-img-element -- data URL, nothing to optimise
          <img
            src={qrImg}
            width={280}
            height={280}
            alt={t("pay.qrAlt")}
            className="rounded-3xl border border-line-soft bg-surface p-4"
          />
        )}
        {qr && (
          <dl className="flex w-full flex-col gap-1.5 rounded-card border border-line-soft bg-surface p-4 text-muted">
            <div className={row}>
              <dt>{t("pay.account")}</dt>
              <dd className="text-ink">{qr.accountNoMasked}</dd>
            </div>
            <div className={row}>
              <dt>{t("pay.holder")}</dt>
              <dd className="text-ink">{qr.accountName}</dd>
            </div>
            <div className={row}>
              <dt>{t("pay.note")}</dt>
              <dd className="font-bold text-ink">{qr.transferNote}</dd>
            </div>
          </dl>
        )}
        <p role="status" className="w-full rounded-[10px] bg-warn-bg p-3.5 font-bold text-warn">
          {p.status === "PENDING" && t("pay.waiting")}
          {p.status === "EXPIRED" && t("pay.expired")}
          {p.status === "MISMATCH" && t("pay.mismatch")}
        </p>
        <p className="text-center text-[13px] text-muted">{t("pay.autoSwitch")}</p>
        <div className="mt-auto flex w-full flex-col gap-3">
          {demo && p.status === "PENDING" && (
            <button
              type="button"
              disabled={simulate.isPending}
              onClick={() => simulate.mutate()}
              className="h-14 whitespace-nowrap rounded-card border-2 border-dashed border-brand text-lg font-bold text-brand disabled:opacity-60"
            >
              {t("pay.simulate")}
            </button>
          )}
          <button type="button" onClick={() => router.back()} className="h-11 font-bold text-ink-2">
            {t("pay.change")}
          </button>
        </div>
      </div>
    </main>
  );
}
