"use client";

import { useRef } from "react";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { newIdempotencyKey } from "../../lib/api";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { useBuildings } from "../rooms/hooks";
import { useMe } from "../session/useMe";
import { useCompleteTask, useTasks } from "./hooks";
import { lp } from "../../lib/locale";
import { Button } from "@/components/ui/button";

export function HousekeepingList() {
  const tasks = useTasks();
  const buildings = useBuildings().data;
  const me = useMe().data;
  const done = useCompleteTask();
  // One key per task, reused if the tap is retried.
  const keys = useRef(new Map<string, string>());
  const keyFor = (id: string) => {
    if (!keys.current.has(id)) keys.current.set(id, newIdempotencyKey());
    return keys.current.get(id)!;
  };

  if (tasks.isError)
    return (
      <p role="alert" className="p-5">
        {t("hk.failed")}
      </p>
    );
  const open = (tasks.data ?? []).filter((x) => x.status === "OPEN");

  return (
    <AppFrame>
      <main className="mx-auto flex w-full max-w-[480px] flex-col">
        <TopBar title={t("hk.title")} subtitle={me?.user.name} />
        <div className="flex flex-col gap-3 px-5 pb-8">
          <h2 className="font-bold">{tf("hk.toClean", { n: open.length })}</h2>
          {tasks.data && !open.length && <p className="text-muted-foreground">{t("hk.empty")}</p>}
          <ul className="flex flex-col gap-3">
            {open.map((x) => (
              <li
                key={x.id}
                className="flex items-center justify-between gap-3 rounded-card border border-dirty-line bg-dirty-bg p-4"
              >
                <div>
                  <p className="text-xl font-bold">{x.roomCode}</p>
                  <p className="text-sm text-ink-2">
                    {buildings?.find((b) => b.id === x.buildingId)?.name} · {t("hk.left")}{" "}
                    {formatClock(x.createdAt)}
                  </p>
                </div>
                <Button
                  type="button"
                  loading={done.isPending}
                  onClick={() => done.mutate({ taskId: x.id, key: keyFor(x.id) })}
                  className="min-w-28"
                >
                  {t("hk.done")}
                </Button>
              </li>
            ))}
          </ul>
          {done.isError && (
            <p role="alert" className="text-sm text-warn">
              {t("hk.doneFailed")}
            </p>
          )}
        </div>
      </main>
    </AppFrame>
  );
}
