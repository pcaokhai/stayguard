"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { ScreenHeader } from "../../components/ScreenHeader";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { formatClock, minutesBetween } from "../../lib/time";
import { useBuildings } from "../rooms/hooks";
import { STATUS } from "../rooms/status";
import { ExtrasSheet } from "./ExtrasSheet";
import { useRoom, useStay } from "./hooks";
import { rentalLabel } from "./labels";

const row = "flex justify-between text-base";

export function StayDetail() {
  const id = useSearchParams().get("id");
  const stay = useStay(id);
  const room = useRoom(stay.data?.roomId ?? null);
  const building = useBuildings().data?.find((b) => b.id === room.data?.buildingId);
  const [sheet, setSheet] = useState(false);

  if (stay.isError)
    return (
      <p role="alert" className="p-5">
        {t("stay.loadFailed")}
      </p>
    );
  const s = stay.data;
  if (!s) return null;
  const q = s.quote;
  const mins = minutesBetween(s.checkInAt, q.asOf);
  const status = STATUS[room.data?.status ?? "OCCUPIED"];

  return (
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col">
      <ScreenHeader
        title={`${t("stay.roomTitle")} ${s.roomCode}`}
        subtitle={[building?.name, room.data?.unitType.name.vi, rentalLabel(s.rentalType)]
          .filter(Boolean)
          .join(" · ")}
        right={
          <span className={`rounded-full border px-3 py-1.5 text-sm font-bold ${status.pill}`}>
            {t(status.label)}
          </span>
        }
      />
      <div className="flex flex-1 flex-col gap-3 px-5 pb-8">
        <section className="rounded-card border border-line-soft bg-surface p-5">
          <p className="text-sm text-muted">{t("stay.staying")}</p>
          <p className="text-[32px] font-bold leading-tight">
            {tf("stay.hoursMinutes", { h: Math.floor(mins / 60), m: mins % 60 })}
          </p>
          <p className="text-sm text-muted">
            {t("stay.since")} {formatClock(s.checkInAt)} · {s.guestName} · {s.guestPhone}
          </p>
        </section>
        <section className="flex flex-col gap-2 rounded-card border border-line-soft bg-surface p-5">
          <div className="flex items-baseline justify-between">
            <h2 className="font-bold">{t("stay.runningTotal")}</h2>
            <p className="text-[32px] font-bold">{formatVnd(q.total)}</p>
          </div>
          <p className={`${row} text-ink-2`}>
            <span>{t("stay.roomCharge")}</span>
            <span>{formatVnd(q.stayAmount)}</span>
          </p>
          <p className={`${row} text-ink-2`}>
            <span>{t("stay.services")}</span>
            <span>{formatVnd(q.extrasAmount)}</span>
          </p>
          <p className={`${row} text-ink-2`}>
            <span>{t("stay.depositPaid")}</span>
            <span>−{formatVnd(q.depositPaid)}</span>
          </p>
          <p className={`${row} border-t border-line-soft pt-2 font-bold`}>
            <span>{q.refundDue > 0 ? t("stay.refundDue") : t("stay.balanceDue")}</span>
            <span>{formatVnd(q.refundDue > 0 ? q.refundDue : q.balanceDue)}</span>
          </p>
          {q.capped && <p className="text-[13px] text-muted">{t("stay.capped")}</p>}
        </section>
        <section className="flex flex-col gap-2 rounded-card border border-line-soft bg-surface p-5">
          <h2 className="font-bold">{t("stay.services")}</h2>
          {s.extras.map((x) => (
            <p key={x.serviceCode} className={row}>
              <span>
                {x.name.vi} × {x.quantity}
              </span>
              <span>{formatVnd(x.amount)}</span>
            </p>
          ))}
          <button
            type="button"
            onClick={() => setSheet(true)}
            className="h-11 rounded-[10px] border border-dashed border-line font-semibold"
          >
            {t("stay.addService")}
          </button>
        </section>
        <Link
          href={`/vi/checkout?stay=${s.id}`}
          className="mt-auto flex h-14 items-center justify-center whitespace-nowrap rounded-card bg-brand text-lg font-bold text-white"
        >
          {t("stay.checkout")}
        </Link>
      </div>
      {sheet && <ExtrasSheet stayId={s.id} roomCode={s.roomCode} onClose={() => setSheet(false)} />}
    </main>
  );
}
