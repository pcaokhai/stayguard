"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { formatClock, minutesBetween } from "../../lib/time";
import { useBuildings } from "../rooms/hooks";
import { STATUS } from "../rooms/status";
import { ExtrasSheet } from "./ExtrasSheet";
import { useRoom, useStay } from "./hooks";
import { rentalLabel } from "./labels";
import { localized, lp } from "../../lib/locale";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

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
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-col">
        <TopBar
          title={`${t("stay.roomTitle")} ${s.roomCode}`}
          subtitle={[
            building?.name,
            room.data && localized(room.data.unitType.name),
            rentalLabel(s.rentalType),
          ]
            .filter(Boolean)
            .join(" · ")}
          back="/rooms"
          right={
            <Badge variant={status.variant} className="h-8 px-3 text-sm font-bold">
              {t(status.label)}
            </Badge>
          }
        />
        <div className="flex flex-1 flex-col gap-3 px-5 pb-8">
          <Card className="p-5 shadow-none">
            <p className="text-sm text-muted-foreground">{t("stay.staying")}</p>
            <p className="text-[32px] font-bold leading-tight">
              {tf("stay.hoursMinutes", { h: Math.floor(mins / 60), m: mins % 60 })}
            </p>
            <p className="text-sm text-muted-foreground">
              {t("stay.since")} {formatClock(s.checkInAt)} · {s.guestName} · {s.guestPhone}
            </p>
          </Card>
          <Card className="gap-2 p-5 shadow-none">
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
            {q.capped && <p className="text-[13px] text-muted-foreground">{t("stay.capped")}</p>}
          </Card>
          <Card className="gap-2 p-5 shadow-none">
            <h2 className="font-bold">{t("stay.services")}</h2>
            {s.extras.map((x) => (
              <p key={x.serviceCode} className={row}>
                <span>
                  {localized(x.name)} × {x.quantity}
                </span>
                <span>{formatVnd(x.amount)}</span>
              </p>
            ))}
            <Button type="button" variant="dashed" onClick={() => setSheet(true)}>
              {t("stay.addService")}
            </Button>
          </Card>
          <Button asChild size="lg" className="mt-auto">
            <Link href={lp(`/checkout?stay=${s.id}`)}>{t("stay.checkout")}</Link>
          </Button>
        </div>
        {sheet && (
          <ExtrasSheet stayId={s.id} roomCode={s.roomCode} onClose={() => setSheet(false)} />
        )}
      </main>
    </AppFrame>
  );
}
