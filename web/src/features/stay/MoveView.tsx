"use client";

import { ArrowLeftRight } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { newIdempotencyKey } from "../../lib/api";
import { localized, lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { cn } from "@/lib/utils";
import { useBuildings, useRooms } from "../rooms/hooks";
import { useMoveStay, useStay } from "./hooks";
import { rentalLabel, type RentalType } from "./labels";

const RENTAL_TYPES: RentalType[] = ["HOURLY", "OVERNIGHT", "DAILY"];

function Segmented<T extends string>({
  value,
  onChange,
  items,
  pill,
}: {
  value: T;
  onChange: (v: T) => void;
  items: { value: T; label: string }[];
  pill: string;
}) {
  return (
    <ToggleGroup
      type="single"
      value={value}
      onValueChange={(v) => v && onChange(v as T)}
      className="w-full gap-0 rounded-xl bg-secondary p-1"
    >
      {items.map((i) => (
        <ToggleGroupItem
          key={i.value}
          value={i.value}
          className="relative h-10 flex-1 rounded-[9px]! px-3 text-sm font-bold data-[state=on]:bg-transparent"
        >
          {i.value === value && (
            <SlidingPill id={pill} className="absolute inset-0 rounded-[9px] bg-card shadow-sm" />
          )}
          <span className="relative">{i.label}</span>
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}

// Board P5: pick a vacant room, keep or change the rental type; the server moves the stay, keeps
// check-in time and extras, prices the whole stay with the new room type and cleans the old room.
export function MoveView() {
  const id = useSearchParams().get("id");
  const router = useRouter();
  const stay = useStay(id);
  const buildings = useBuildings();
  const move = useMoveStay(id ?? "");
  const [key] = useState(newIdempotencyKey);
  const [buildingId, setBuildingId] = useState<string>();
  const [toRoomId, setToRoomId] = useState<string>();
  const [rental, setRental] = useState<RentalType | undefined>();
  const editable = buildings.data?.filter((b) => b.level !== "NONE" && b.level !== "VIEW") ?? [];
  const current = editable.find((b) => b.id === buildingId) ?? editable[0];
  const rooms = useRooms(current?.id);
  const vacant = rooms.data?.filter((r) => r.status === "VACANT") ?? [];
  const target = vacant.find((r) => r.id === toRoomId);
  const s = stay.data;
  const type = rental ?? s?.rentalType ?? "HOURLY";

  if (stay.isError || buildings.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([stay.refetch(), buildings.refetch()])} />
      </AppFrame>
    );

  const submit = () =>
    target &&
    move.mutate(
      { key, body: { toRoomId: target.id, rentalType: type } },
      {
        onSuccess: (moved) => {
          toast.success(tf("move.moved", { room: target.code }));
          router.replace(lp(`/stay?id=${moved.id}`));
        },
      },
    );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[480px] flex-1 flex-col">
        <TopBar
          title={`${t("move.title")} · ${s?.roomCode ?? ""}`}
          subtitle={
            s ? tf("move.guest", { name: s.guestName, time: formatClock(s.checkInAt) }) : undefined
          }
          back={`/stay?id=${id}`}
        />
        <div className="flex flex-1 flex-col gap-4 px-5 pb-8">
          <h2 className="text-xs font-bold uppercase tracking-wide text-muted-foreground">
            {t("move.pick")}
          </h2>
          {editable.length > 1 && (
            <Segmented
              pill="move-building"
              value={current?.id ?? ""}
              onChange={(v) => {
                setBuildingId(v);
                setToRoomId(undefined);
              }}
              items={editable.map((b) => ({ value: b.id, label: b.name }))}
            />
          )}
          {rooms.data && !vacant.length && (
            <p className="text-sm text-muted-foreground">{t("move.noVacant")}</p>
          )}
          <ul className="grid grid-cols-4 gap-2">
            {vacant.map((r) => (
              <li key={r.id}>
                <button
                  type="button"
                  aria-pressed={r.id === toRoomId}
                  onClick={() => setToRoomId(r.id)}
                  className={cn(
                    "flex h-[60px] w-full flex-col items-center justify-center rounded-[10px] border border-ok-line bg-ok-bg text-ok transition-colors",
                    r.id === toRoomId && "border-2 border-primary",
                  )}
                >
                  <b className="text-base">{r.code}</b>
                  <span className="max-w-full truncate px-1 text-xs">
                    {localized(r.unitType.name)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
          <h2 className="text-xs font-bold uppercase tracking-wide text-muted-foreground">
            {t("move.rental")}
          </h2>
          <Segmented
            pill="move-rental"
            value={type}
            onChange={setRental}
            items={RENTAL_TYPES.map((r) => ({ value: r, label: rentalLabel(r) }))}
          />
          <Card className="gap-2.5 p-4 text-sm shadow-none">
            <p className="flex justify-between">
              <span className="text-muted-foreground">{t("move.checkIn")}</span>
              <span>{s && tf("move.unchanged", { time: formatClock(s.checkInAt) })}</span>
            </p>
            {target && (
              <p className="flex justify-between">
                <span className="text-muted-foreground">{t("move.rate")}</span>
                <span>{tf("move.wholeStay", { type: localized(target.unitType.name) })}</span>
              </p>
            )}
            <p className="flex justify-between">
              <span className="text-muted-foreground">{t("move.totalNow")}</span>
              <b>{s && formatVnd(s.quote.total)}</b>
            </p>
            <p className="flex justify-between">
              <span className="text-muted-foreground">{t("move.extras")}</span>
              <span>{t("move.withGuest")}</span>
            </p>
          </Card>
          <p className="flex items-start gap-2 text-[13px] text-muted-foreground">
            <ArrowLeftRight className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
            {tf("move.note", { room: s?.roomCode ?? "" })}
          </p>
          {move.isError && (
            <p role="alert" className="text-sm font-semibold text-warn">
              {t("move.failed")}
            </p>
          )}
          <Button
            size="lg"
            className="mt-auto"
            disabled={!target}
            loading={move.isPending}
            onClick={submit}
          >
            {target ? tf("move.submit", { room: target.code }) : t("move.pickFirst")}
          </Button>
        </div>
      </main>
    </AppFrame>
  );
}
