"use client";

import { ArrowLeftRight, Pencil } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { localized, lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { formatClock, minutesBetween } from "../../lib/time";
import { FlowSplit } from "../rooms/FlowSplit";
import { useBuildings } from "../rooms/hooks";
import { STATUS } from "../rooms/status";
import { ExtrasSheet } from "./ExtrasSheet";
import { useRoom, useStay } from "./hooks";
import { IdChips } from "./IdChips";
import { rentalLabel } from "./labels";

const row = "flex justify-between text-[15px]";

export function StayDetail() {
  const id = useSearchParams().get("id");
  const stay = useStay(id);
  const room = useRoom(stay.data?.roomId ?? null);
  const building = useBuildings().data?.find((b) => b.id === room.data?.buildingId);
  const [sheet, setSheet] = useState(false);

  if (stay.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void stay.refetch()} />
      </AppFrame>
    );
  const s = stay.data;
  const q = s?.quote;
  const mins = s && q ? minutesBetween(s.checkInAt, q.asOf) : 0;
  const status = STATUS[room.data?.status ?? "OCCUPIED"];

  return (
    <AppFrame tabs={false}>
      <FlowSplit roomId={s?.roomId}>
        <TopBar
          title={s ? `${t("stay.roomTitle")} ${s.roomCode}` : t("stay.roomTitle")}
          subtitle={
            s
              ? [
                  building?.name,
                  room.data && localized(room.data.unitType.name),
                  rentalLabel(s.rentalType),
                ]
                  .filter(Boolean)
                  .join(" · ")
              : undefined
          }
          back="/rooms"
          right={
            <Badge variant={status.variant} className="mr-3 h-8 px-3 text-sm font-bold">
              {t(status.label)}
            </Badge>
          }
        />
        {!s || !q ? (
          <Skeleton className="mx-5 h-64" />
        ) : (
          <div className="flex flex-col gap-3 px-5 pb-8 lg:pb-0">
            <Card className="gap-2 p-5 shadow-none">
              <p className="text-sm text-muted-foreground">{t("stay.staying")}</p>
              <p className="text-[34px] font-bold leading-none">
                {tf("stay.hoursMinutes", { h: Math.floor(mins / 60), m: mins % 60 })}
              </p>
              <p className="text-sm text-muted-foreground">
                {t("stay.since")} {formatClock(s.checkInAt)} · {s.guestName} · {s.guestPhone}
              </p>
              {s.guestId && <IdChips ids={s.guestId} />}
            </Card>
            <Card className="gap-2 p-5 shadow-none">
              <div className="flex items-baseline justify-between">
                <h2 className="font-bold">{t("stay.runningTotal")}</h2>
                <p className="text-[32px] font-bold leading-none">{formatVnd(q.total)}</p>
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
              <p className={`${row} border-t border-border pt-2 font-bold`}>
                <span>{q.refundDue > 0 ? t("stay.refundDue") : t("stay.balanceDue")}</span>
                <span>{formatVnd(q.refundDue > 0 ? q.refundDue : q.balanceDue)}</span>
              </p>
              {q.capped && <p className="text-[13px] text-muted-foreground">{t("stay.capped")}</p>}
            </Card>
            <Card className="gap-2.5 p-5 shadow-none">
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
              <div className="grid grid-cols-2 gap-2">
                <Button asChild variant="outline">
                  <Link href={lp(`/stay/edit-time?id=${s.id}`)}>
                    <Pencil aria-hidden="true" />
                    {t("stay.editTime")}
                  </Link>
                </Button>
                <Button asChild variant="outline">
                  <Link href={lp(`/stay/move?id=${s.id}`)}>
                    <ArrowLeftRight aria-hidden="true" />
                    {t("stay.moveRoom")}
                  </Link>
                </Button>
              </div>
            </Card>
            <Button asChild size="lg" className="mt-6">
              <Link href={lp(`/checkout?stay=${s.id}`)}>{t("stay.checkout")}</Link>
            </Button>
          </div>
        )}
        {sheet && s && (
          <ExtrasSheet
            stayId={s.id}
            roomCode={s.roomCode}
            extras={s.extras}
            onClose={() => setSheet(false)}
          />
        )}
      </FlowSplit>
    </AppFrame>
  );
}
