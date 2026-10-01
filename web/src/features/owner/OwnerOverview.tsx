"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ScreenHeader } from "../../components/ScreenHeader";
import { api } from "../../lib/api";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";

const row = "flex justify-between";

function useOverview() {
  return useQuery({
    queryKey: ["owner-overview"],
    refetchInterval: 30_000,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/owner/overview");
      if (error || !data) throw new Error("getOwnerOverview failed");
      return data;
    },
  });
}

export function OwnerOverview() {
  const q = useOverview();
  if (q.isError)
    return (
      <p role="alert" className="p-5">
        {t("owner.failed")}
      </p>
    );
  const o = q.data;
  if (!o) return null;
  const [, month, day] = o.date.split("-");

  return (
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col">
      <ScreenHeader
        title={`${t("owner.today")}, ${day}/${month}`}
        subtitle={t("owner.updated")}
        back="/vi"
      />
      <div className="flex flex-1 flex-col gap-3 px-5 pb-8">
        <section className="flex flex-col gap-3 rounded-card border border-line-soft bg-surface p-5">
          <p className="text-sm text-muted">{t("owner.revenue")}</p>
          <p className="text-[40px] font-bold leading-none">{formatVnd(o.revenueTotal)}</p>
          <div className="grid grid-cols-2 gap-2">
            <p className="rounded-card bg-ok-bg p-2.5 text-[13px] text-ok">
              {t("owner.transfers")}
              <b className="block text-xl">{formatVnd(o.transfersReceived)}</b>
            </p>
            <p className="rounded-card bg-sunken p-2.5 text-[13px] text-ink-2">
              {t("owner.cash")}
              <b className="block text-xl text-ink">{formatVnd(o.cashExpected)}</b>
            </p>
          </div>
          {o.byBuilding.map((b) => (
            <p key={b.buildingId} className={`${row} text-ink-2`}>
              <span>{b.name}</span>
              <span>{formatVnd(b.revenue)}</span>
            </p>
          ))}
          <p className={`${row} text-ink-2`}>
            <span>{t("owner.occupancy")}</span>
            <span>
              {tf("owner.occupancyValue", {
                o: o.occupancy.occupiedRooms,
                t: o.occupancy.totalRooms,
                x: o.occupancy.overdueRooms,
              })}
            </span>
          </p>
        </section>
        {o.alerts.length > 0 && (
          <section className="flex flex-col gap-2">
            <h2 className="font-bold">{tf("owner.alerts", { n: o.alerts.length })}</h2>
            {o.alerts.map((a) => (
              <p
                key={a.id}
                className="rounded-card border border-warn-line bg-warn-bg p-3.5 font-bold text-warn"
              >
                {a.kind}
                {a.roomCode ? ` · ${a.roomCode}` : ""}
                {a.amount != null ? ` · ${formatVnd(a.amount)}` : ""}
              </p>
            ))}
          </section>
        )}
        <section className="flex flex-col gap-2 rounded-card border border-line-soft bg-surface p-5">
          <h2 className="font-bold">{t("owner.latest")}</h2>
          {o.latestPayments.map((p) => (
            <p key={p.paymentId} className={row}>
              <span>
                {p.roomCode} · {p.method === "TRANSFER" ? t("owner.qr") : t("owner.cashShort")} ·{" "}
                {formatClock(p.at)}
              </span>
              <span>{formatVnd(p.amount)}</span>
            </p>
          ))}
        </section>
        <Link
          href="/vi/rooms"
          className="mt-auto flex h-14 items-center justify-center whitespace-nowrap rounded-card bg-brand text-lg font-bold text-white"
        >
          {t("owner.toRooms")}
        </Link>
      </div>
    </main>
  );
}
