"use client";

import { Check } from "lucide-react";
import Link from "next/link";
import { useRef, useState } from "react";
import type { components } from "../../api/generated/schema";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { newIdempotencyKey } from "../../lib/api";
import { lp } from "../../lib/locale";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { useCompleteTask, useTasks } from "./hooks";

type Room = components["schemas"]["Room"];
const CHECKLIST = ["clean.linen", "clean.towels", "clean.bins", "clean.water"] as const;

// The one clean-room screen body: when the guest left, the memory-aid checklist, Cleaned, Set maintenance and
// Report damage. The /clean page and the room-map panel both render it (boards PhongCanDon, PhongCanDonPC,
// PhongCanDonBP). Without Edit on the building (`readOnly`) only the information shows. Marking cleaned is
// optimistic (useCompleteTask): the room leaves the list at once, a toast confirms, a failure rolls back.
export function CleanRoom({
  room,
  readOnly,
  onDone,
}: {
  room: Room;
  readOnly: boolean;
  onDone?: () => void;
}) {
  const tasks = useTasks();
  const done = useCompleteTask();
  const key = useRef(newIdempotencyKey());
  const [checked, setChecked] = useState<Record<string, boolean>>({});
  const task = tasks.data?.find((x) => x.roomId === room.id && x.status === "OPEN");

  return (
    <div className="flex flex-col gap-3">
      {task ? (
        <p className="flex justify-between text-sm">
          <span className="text-muted-foreground">{t("clean.checkedOut")}</span>
          <span>{tf("clean.today", { time: formatClock(task.createdAt) })}</span>
        </p>
      ) : (
        tasks.data && <p className="text-sm text-muted-foreground">{t("clean.gone")}</p>
      )}
      {!readOnly && (
        <>
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
          <div className="mt-3 flex flex-col gap-2">
            <Button
              size="lg"
              disabled={!task}
              loading={done.isPending}
              onClick={() =>
                task &&
                done.mutate(
                  { taskId: task.id, key: key.current, roomCode: room.code },
                  { onSuccess: onDone },
                )
              }
            >
              <Check aria-hidden="true" />
              {t("clean.done")}
            </Button>
            <div className="grid grid-cols-2 gap-2">
              <Button asChild variant="outline">
                <Link href={lp(`/report-damage?room=${room.id}&lock=1`)}>
                  {t("clean.maintenance")}
                </Link>
              </Button>
              <Button asChild variant="outline">
                <Link href={lp(`/report-damage?room=${room.id}`)}>{t("clean.damage")}</Link>
              </Button>
            </div>
            <p className="text-[13px] text-muted-foreground">{t("clean.note")}</p>
          </div>
        </>
      )}
    </div>
  );
}
