"use client";

import { useRef } from "react";
import { ScreenHeader } from "../../components/ScreenHeader";
import { newIdempotencyKey } from "../../lib/api";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { useBuildings } from "../rooms/hooks";
import { useMe } from "../session/useMe";
import { useCompleteTask, useTasks } from "./hooks";

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
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col">
      <ScreenHeader title={t("hk.title")} subtitle={me?.user.name} back="/vi" />
      <div className="flex flex-col gap-3 px-5 pb-8">
        <h2 className="font-bold">{tf("hk.toClean", { n: open.length })}</h2>
        {tasks.data && !open.length && <p className="text-muted">{t("hk.empty")}</p>}
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
              <button
                type="button"
                disabled={done.isPending}
                onClick={() => done.mutate({ taskId: x.id, key: keyFor(x.id) })}
                className="h-11 min-w-28 whitespace-nowrap rounded-[10px] bg-brand px-4 font-bold text-white disabled:opacity-60"
              >
                {t("hk.done")}
              </button>
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
  );
}
