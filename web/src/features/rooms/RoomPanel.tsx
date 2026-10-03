"use client";

import { ArrowLeftRight, Pencil } from "lucide-react";
import Link from "next/link";
import { useRef, useState } from "react";
import type { components } from "../../api/generated/schema";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { newIdempotencyKey } from "../../lib/api";
import { localized, lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { formatClock, minutesBetween } from "../../lib/time";
import { useCompleteTask, useTasks } from "../housekeeping/hooks";
import { ExtrasSheet } from "../stay/ExtrasSheet";
import { useStay } from "../stay/hooks";
import { rentalLabel } from "../stay/labels";
import { IdChips } from "../stay/IdChips";
import { isOwnerRole } from "@/components/shell/nav";
import { useMe } from "../session/useMe";
import { tileStatus } from "./status";
import { ResumePayment } from "./ResumePayment";
import { CleanRoom } from "../housekeeping/CleanRoom";

type Room = components["schemas"]["Room"];

const row = "flex justify-between gap-3 text-sm";

// Desktop side panel (docs/15 §1): details and the next action for the selected room.
export function RoomPanel({ room, readOnly }: { room?: Room; readOnly: boolean }) {
  if (!room)
    return (
      <Card className="p-5 text-sm text-muted-foreground shadow-none">{t("rooms.pickRoom")}</Card>
    );
  const s = tileStatus(room);
  return (
    <Card className="gap-3 p-5 shadow-none">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-[22px] font-bold">{tf("panel.room", { code: room.code })}</h2>
        <Badge variant={s.variant} className="px-3 py-1 text-sm font-bold">
          {t(s.label)}
        </Badge>
      </div>
      {room.activeStay ? (
        <StayBody room={room} stayId={room.activeStay.id} readOnly={readOnly} />
      ) : (
        <IdleBody room={room} readOnly={readOnly} />
      )}
    </Card>
  );
}

function StayBody({ room, stayId, readOnly }: { room: Room; stayId: string; readOnly: boolean }) {
  const owner = isOwnerRole(useMe().data?.user.role);
  const stay = useStay(stayId);
  const [extras, setExtras] = useState(false);
  const s = stay.data;
  const pending = room.activeStay?.pendingPayment;
  if (!s) return <Skeleton className="h-64 w-full" />;
  const q = s.quote;
  const mins = minutesBetween(s.checkInAt, q.asOf);
  return (
    <>
      <p className="text-[13px] text-muted-foreground">
        {[localized(room.unitType.name), rentalLabel(s.rentalType), s.guestName, s.guestPhone].join(
          " · ",
        )}
      </p>
      {s.guestId &&
        (owner ? <OwnerIdRow stayId={s.id} ids={s.guestId} /> : <IdChips ids={s.guestId} />)}
      {pending ? (
        <ResumePayment
          stayId={s.id}
          roomId={room.id}
          pending={pending}
          readOnly={readOnly}
          invoiceId={s.invoiceId}
        />
      ) : null}
      {pending ? null : (
        <>
          <div className="border-b border-border pb-3">
            <p className="text-[13px] text-muted-foreground">
              {tf("panel.stayedSince", { time: formatClock(s.checkInAt) })}
            </p>
            <p className="text-[28px] font-bold leading-tight">
              {tf("stay.hoursMinutes", { h: Math.floor(mins / 60), m: mins % 60 })}
            </p>
          </div>
          <div className="flex flex-col gap-2.5">
            <p className={row}>
              <span>{t("panel.roomCharge")}</span>
              <span>{formatVnd(q.stayAmount)}</span>
            </p>
            <p className={row}>
              <span>{t("panel.extras")}</span>
              <span>{formatVnd(q.extrasAmount)}</span>
            </p>
            <p className={row}>
              <span>{t("panel.deposit")}</span>
              <span>−{formatVnd(q.depositPaid)}</span>
            </p>
            <p className="flex items-baseline justify-between font-bold">
              <span>{q.refundDue > 0 ? t("panel.refundDue") : t("panel.balanceDue")}</span>
              <span className="text-[28px]">
                {formatVnd(q.refundDue > 0 ? q.refundDue : q.balanceDue)}
              </span>
            </p>
          </div>
          {!readOnly && (
            <div className="mt-2 flex flex-col gap-2">
              <Button type="button" variant="dashed" onClick={() => setExtras(true)}>
                {t("panel.addExtras")}
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
              <Button asChild size="lg">
                <Link href={lp(`/checkout?stay=${s.id}`)}>{t("panel.checkout")}</Link>
              </Button>
            </div>
          )}
        </>
      )}
      {extras && (
        <ExtrasSheet stayId={s.id} roomCode={s.roomCode} onClose={() => setExtras(false)} />
      )}
    </>
  );
}

// Every state has an explicit branch; anything else is an error screen, never maintenance text by default.
function IdleBody({ room, readOnly }: { room: Room; readOnly: boolean }) {
  if (room.status === "TO_CLEAN") return <CleanRoom room={room} readOnly={readOnly} />;
  if (room.status !== "VACANT" && room.status !== "MAINTENANCE") return <PanelLoadFailed />;
  const sub =
    room.status === "VACANT" ? t("rooms.vacantSub") : (room.note ?? t("rooms.maintenanceSub"));
  return (
    <>
      <p className="text-sm text-muted-foreground">{sub}</p>
      {!readOnly && room.status === "VACANT" && (
        <Button asChild size="lg" className="mt-2">
          <Link href={lp(`/checkin?room=${room.id}`)}>{t("panel.checkin")}</Link>
        </Button>
      )}
    </>
  );
}

export function PanelLoadFailed() {
  return (
    <div role="alert" className="flex flex-col gap-3">
      <p className="font-semibold">{t("rooms.panelLoadFailed")}</p>
      <Button variant="outline" onClick={() => window.location.reload()}>
        {t("panel.reload")}
      </Button>
    </div>
  );
}

// Owner and manager: a link to the audited ID panel; the number and photos are never shown here.
function OwnerIdRow({
  stayId,
  ids,
}: {
  stayId: string;
  ids: components["schemas"]["GuestIdIndicators"];
}) {
  const photos = Number(ids.hasFrontPhoto) + Number(ids.hasBackPhoto);
  return (
    <p className={row}>
      <span className="text-muted-foreground">{t("guestId.panelTitle")}</span>
      {ids.hasIdNumber || photos ? (
        <Link href={lp(`/owner/stay?id=${stayId}`)} className="font-bold text-primary underline">
          {tf("guestId.link", { n: photos })}
        </Link>
      ) : (
        <span className="text-muted-foreground">{t("guestId.linkNone")}</span>
      )}
    </p>
  );
}
