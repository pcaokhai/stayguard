"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import type { ReactNode } from "react";
import { FadeIn } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { lp } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { localized } from "@/lib/locale";
import { t, tf, type MessageKey } from "@/lib/t";
import { GuestIdPanel } from "../../guestid/GuestIdPanel";
import { useStay } from "../../stay/hooks";
import { billLineLabel } from "../../stay/labels";
import { clockOf, formatDayMonth } from "../format";
import { useTimeline } from "./hooks";
import { dotTone, eventText } from "./timeline";

const heading = "text-[13px] font-bold uppercase tracking-wide text-ink-2";
const RENTAL: Record<string, MessageKey> = {
  HOURLY: "ownerStays.rentalHourly",
  OVERNIGHT: "ownerStays.rentalOvernight",
  DAILY: "ownerStays.rentalDaily",
};

// "0909123321" -> "0909 ••• 321": the owner sees the guest, not the whole number on screen.
const maskPhone = (p: string) => (p.length > 6 ? `${p.slice(0, 4)} ••• ${p.slice(-3)}` : p);

const Row = ({ k, v, strong }: { k: string; v: ReactNode; strong?: boolean }) => (
  <div className="flex justify-between gap-3 py-0.5">
    <dt className="text-ink-2">{k}</dt>
    <dd className={strong ? "font-bold" : undefined}>{v}</dd>
  </div>
);

export function StayView() {
  const id = useSearchParams().get("id");
  const stay = useStay(id);
  const timeline = useTimeline(id);

  if (stay.isError || timeline.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([stay.refetch(), timeline.refetch()])} />
      </AppFrame>
    );
  const s = stay.data;
  const events = timeline.data ?? [];
  if (!s)
    return (
      <AppFrame tabs={false}>
        <Skeleton className="m-5 h-64 rounded-card" aria-busy="true" />
      </AppFrame>
    );
  const edits = events.filter((e) => e.kind === "CHECK_IN_EDITED").length;
  const rental = t(RENTAL[s.rentalType]);
  const q = s.quote;

  const timelineCard = (
    <Card className="gap-1 p-5 shadow-none">
      <h2 className={heading}>{t("ownerStays.timeline")}</h2>
      <ol>
        {events.map((e, i) => (
          <li
            key={`${e.kind}-${i}`}
            className="grid grid-cols-[48px_12px_1fr] items-start gap-x-3 border-t border-border py-2.5 first:border-t-0"
          >
            <b className="text-[15px]">{clockOf(e.at)}</b>
            <span
              className={`mt-1.5 size-2.5 rounded-full ${dotTone(e.kind)}`}
              aria-hidden="true"
            />
            <div className="leading-snug">
              <p className="text-[15px]">{eventText(e)}</p>
              <p className="text-[13px] text-muted-foreground">{e.actorName}</p>
            </div>
          </li>
        ))}
      </ol>
      <p className="mt-1 text-[13px] text-muted-foreground">{t("ownerStays.timelineNote")}</p>
    </Card>
  );
  const guestCard = (
    <Card className="gap-1 p-5 shadow-none">
      <h2 className={heading}>{t("ownerStays.guest")}</h2>
      <dl className="text-[15px]">
        <Row k={t("ownerStays.name")} v={s.guestName} />
        <Row k={t("ownerStays.phone")} v={maskPhone(s.guestPhone)} />
      </dl>
    </Card>
  );
  const billCard = (
    <Card className="gap-1 p-5 shadow-none">
      <h2 className={heading}>{t("ownerStays.bill")}</h2>
      <dl className="text-[15px]">
        {q.lines.map((l) => (
          <Row key={l.code} k={billLineLabel(l.code)} v={formatVnd(l.amount)} />
        ))}
        {s.extras.map((x) => (
          <Row
            key={x.serviceCode}
            k={tf("ownerStays.extraQty", { name: localized(x.name), qty: x.quantity })}
            v={formatVnd(x.amount)}
          />
        ))}
        <Row k={t("ownerStays.total")} v={formatVnd(q.total)} strong />
        <Row k={t("ownerStays.deposit")} v={`-${formatVnd(q.depositPaid)}`} />
        {q.refundDue > 0 && <Row k={t("ownerStays.refund")} v={formatVnd(q.refundDue)} />}
        {q.balanceDue > 0 && <Row k={t("ownerStays.balance")} v={formatVnd(q.balanceDue)} />}
      </dl>
    </Card>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={tf("ownerStays.detailTitle", {
            room: s.roomCode,
            date: formatDayMonth(s.checkInAt),
          })}
          subtitle={
            edits
              ? tf("ownerStays.detailSub", { rental, n: edits })
              : tf("ownerStays.detailSubNone", { rental })
          }
          back="/owner/stays"
          right={
            <Button asChild variant="outline" size="lg" className="hidden lg:inline-flex">
              <Link href={lp("/owner/stays")}>{t("ownerStays.backHistory")}</Link>
            </Button>
          }
        />
        <FadeIn className="grid gap-3 px-5 lg:grid-cols-[1.4fr_1fr] lg:items-start lg:gap-4">
          <div className="flex flex-col gap-3 max-lg:contents">
            <div className="max-lg:order-2">{timelineCard}</div>
          </div>
          <div className="flex flex-col gap-3 max-lg:contents">
            <div className="flex flex-col gap-3 max-lg:order-1">
              {guestCard}
              <GuestIdPanel stayId={s.id} guestName={s.guestName} />
            </div>
            <div className="max-lg:order-3">{billCard}</div>
          </div>
        </FadeIn>
      </main>
    </AppFrame>
  );
}
