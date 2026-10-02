"use client";

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
import { STATUS } from "./status";

type Room = components["schemas"]["Room"];

const row = "flex justify-between gap-3 text-sm";

// Desktop side panel (docs/15 §1): details and the next action for the selected room.
export function RoomPanel({ room, readOnly }: { room?: Room; readOnly: boolean }) {
  if (!room)
    return (
      <Card className="p-5 text-sm text-muted-foreground shadow-none">{t("rooms.pickRoom")}</Card>
    );
  const s = STATUS[room.status];
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
  const stay = useStay(stayId);
  const [extras, setExtras] = useState(false);
  const s = stay.data;
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
      {s.guestId && <IdChips ids={s.guestId} />}
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
          <Button asChild size="lg">
            <Link href={lp(`/checkout?stay=${s.id}`)}>{t("panel.checkout")}</Link>
          </Button>
        </div>
      )}
      {extras && (
        <ExtrasSheet stayId={s.id} roomCode={s.roomCode} onClose={() => setExtras(false)} />
      )}
    </>
  );
}

function IdleBody({ room, readOnly }: { room: Room; readOnly: boolean }) {
  const tasks = useTasks();
  const done = useCompleteTask();
  const key = useRef(newIdempotencyKey());
  const task = tasks.data?.find((x) => x.status === "OPEN" && x.roomCode === room.code);
  const sub =
    room.status === "VACANT"
      ? t("rooms.vacantSub")
      : room.status === "TO_CLEAN"
        ? t("rooms.cleanSub")
        : (room.note ?? t("rooms.maintenanceSub"));
  return (
    <>
      <p className="text-sm text-muted-foreground">{sub}</p>
      {!readOnly && room.status === "VACANT" && (
        <Button asChild size="lg" className="mt-2">
          <Link href={lp(`/checkin?room=${room.id}`)}>{t("panel.checkin")}</Link>
        </Button>
      )}
      {!readOnly && room.status === "TO_CLEAN" && (
        <>
          <Button
            size="lg"
            className="mt-2"
            disabled={!task}
            loading={done.isPending}
            onClick={() => task && done.mutate({ taskId: task.id, key: key.current })}
          >
            {t("panel.markClean")}
          </Button>
          {done.isError && (
            <p role="alert" className="text-sm text-warn">
              {t("panel.cleanFailed")}
            </p>
          )}
        </>
      )}
    </>
  );
}
