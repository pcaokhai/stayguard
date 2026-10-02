"use client";

import { Check } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useRef, useState } from "react";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { newIdempotencyKey } from "../../lib/api";
import { localized, lp } from "../../lib/locale";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { FlowSplit } from "../rooms/FlowSplit";
import { useBuildings } from "../rooms/hooks";
import { STATUS } from "../rooms/status";
import { useMe } from "../session/useMe";
import { useNow } from "@/hooks/useNow";
import { useRoom } from "../stay/hooks";
import { useCompleteTask, useTasks } from "./hooks";
import { waitedMinutes, waitText } from "./HousekeepingList";

const CHECKLIST = ["clean.linen", "clean.towels", "clean.bins", "clean.water"] as const;

// Boards P37, P48 and PC "Dọn phòng". The checklist is a memory aid only: nothing is stored or sent.
export function CleanView() {
  const roomId = useSearchParams().get("room");
  const router = useRouter();
  const room = useRoom(roomId);
  const tasks = useTasks();
  const building = useBuildings().data?.find((b) => b.id === room.data?.buildingId);
  const now = useNow();
  const role = useMe().data?.user.role;
  const done = useCompleteTask();
  const key = useRef(newIdempotencyKey());
  const [checked, setChecked] = useState<Record<string, boolean>>({});
  const task = tasks.data?.find((x) => x.roomId === roomId && x.status === "OPEN");
  const r = room.data;
  const status = STATUS["TO_CLEAN"];
  const back = role === "HOUSEKEEPING" ? "/housekeeping" : "/rooms";

  if (room.isError || tasks.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([room.refetch(), tasks.refetch()])} />
      </AppFrame>
    );
  const waiting = task ? waitedMinutes(task, now) : 0;

  return (
    <AppFrame tabs={false}>
      <FlowSplit roomId={roomId}>
        <TopBar
          title={
            role === "HOUSEKEEPING" && r
              ? tf("clean.titleRoom", { code: r.code })
              : t("clean.title")
          }
          subtitle={
            r
              ? role === "HOUSEKEEPING"
                ? tf("clean.waitingPlace", {
                    building: building?.name ?? "",
                    time: waitText(waiting),
                  })
                : tf("clean.place", {
                    building: building?.name ?? "",
                    floor: r.floor,
                    type: localized(r.unitType.name),
                  })
              : undefined
          }
          back={back}
        />
        <div className="flex flex-col gap-3 px-5 pb-8 lg:pb-0">
          {!r ? (
            <Skeleton className="h-72" />
          ) : (
            <Card className="gap-3 p-5 shadow-none">
              <div className="flex items-center justify-between gap-3">
                <h2 className="text-[22px] font-bold">{tf("clean.room", { code: r.code })}</h2>
                <Badge variant={status.variant} className="px-3 py-1 text-sm font-bold">
                  {t(status.label)}
                </Badge>
              </div>
              {task ? (
                <p className="flex justify-between text-sm">
                  <span className="text-muted-foreground">{t("clean.checkedOut")}</span>
                  <span>{tf("clean.today", { time: formatClock(task.createdAt) })}</span>
                </p>
              ) : (
                <p className="text-sm text-muted-foreground">{t("clean.gone")}</p>
              )}
              <h3 className="border-b border-border pb-1.5 text-xs font-bold uppercase tracking-wide text-muted-foreground">
                {t("clean.checklist")}
              </h3>
              <ul className="flex flex-col">
                {CHECKLIST.map((k) => (
                  <li key={k} className="border-b border-border last:border-0">
                    <label className="flex min-h-12 cursor-pointer items-center gap-3">
                      <Checkbox
                        checked={!!checked[k]}
                        onCheckedChange={(v) => setChecked((c) => ({ ...c, [k]: v === true }))}
                        className="size-6"
                      />
                      {t(k)}
                    </label>
                  </li>
                ))}
              </ul>
              <div className="mt-6 flex flex-col gap-2">
                <Button
                  size="lg"
                  disabled={!task}
                  loading={done.isPending}
                  onClick={() =>
                    task &&
                    done.mutate(
                      { taskId: task.id, key: key.current, roomCode: r.code },
                      { onSuccess: () => router.replace(lp(back)) },
                    )
                  }
                >
                  <Check aria-hidden="true" />
                  {t("clean.done")}
                </Button>
                <div className="grid grid-cols-2 gap-2">
                  <Button asChild variant="outline">
                    <Link href={lp(`/report-damage?room=${r.id}&lock=1`)}>
                      {t("clean.maintenance")}
                    </Link>
                  </Button>
                  <Button asChild variant="outline">
                    <Link href={lp(`/report-damage?room=${r.id}`)}>{t("clean.damage")}</Link>
                  </Button>
                </div>
                <p className="text-[13px] text-muted-foreground">{t("clean.note")}</p>
              </div>
            </Card>
          )}
        </div>
      </FlowSplit>
    </AppFrame>
  );
}
