"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { t } from "../../lib/t";
import { useRoom } from "../stay/hooks";
import { useBuildings, useRooms } from "./hooks";
import { RoomTile } from "./RoomTile";

// Desktop (>= 1024 px, boards "PC · …"): the room map stays on the left and the flow is the right
// panel. Below that the flow is a plain page and the map is not rendered.
export function FlowSplit({ roomId, children }: { roomId?: string | null; children: ReactNode }) {
  return (
    <div className="lg:grid lg:grid-cols-[1fr_440px] lg:items-start lg:gap-6 lg:px-5 lg:pt-5">
      <MiniMap roomId={roomId} />
      <div className="mx-auto w-full max-w-[480px] lg:max-w-none lg:rounded-2xl lg:border lg:bg-card lg:pb-6">
        {children}
      </div>
    </div>
  );
}

function MiniMap({ roomId }: { roomId?: string | null }) {
  const room = useRoom(roomId ?? null).data;
  const building = useBuildings().data?.find((b) => b.id === room?.buildingId);
  const rooms = useRooms(room?.buildingId);
  return (
    <section aria-label={t("rooms.title")} className={cn("hidden lg:block", !room && "lg:hidden")}>
      <h2 className="pb-2 text-base font-bold">
        {building?.name} · {rooms.data?.length}
      </h2>
      <ul className="grid grid-cols-5 gap-2 xl:grid-cols-6">
        {rooms.data?.map((r) => (
          <li key={r.id}>
            <RoomTile room={r} readOnly={building?.level === "VIEW"} selected={r.id === room?.id} />
          </li>
        ))}
      </ul>
    </section>
  );
}
