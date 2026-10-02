"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, Pencil, Plus, Search } from "lucide-react";
import { FadeIn, SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, QueryError } from "@/components/StateView";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { localized } from "@/lib/locale";
import { t, tf } from "@/lib/t";
import { useBuildings, useRooms } from "../../rooms/hooks";
import { STATUS } from "../../rooms/status";
import { useRatePlans } from "../rates/hooks";
import { codeRange } from "./codes";
import { BuildingForm, BuildingNameForm, FloorForm, RoomEditForm, RoomsForm } from "./forms";
import type { Room } from "./hooks";

type Dialog =
  | { kind: "building" }
  | { kind: "buildingName" }
  | { kind: "floor" }
  | { kind: "rooms"; floor: number }
  | { kind: "room"; room: Room };

const RoomStatus = ({ r }: { r: Room }) => (
  <Badge variant={STATUS[r.status].variant} className="px-2.5 py-1 font-semibold">
    {t(STATUS[r.status].label)}
  </Badge>
);

export function BuildingsView() {
  const wide = useMediaQuery("(min-width: 768px)");
  const buildings = useBuildings();
  const types = useRatePlans();
  const [id, setId] = useState<string>();
  const [find, setFind] = useState("");
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const bs = buildings.data ?? [];
  const current = bs.find((b) => b.id === id) ?? bs[0];
  const rooms = useRooms(current?.id);
  const all = (rooms.data ?? []) as Room[];
  const floors = [...new Set(all.map((r) => r.floor))].sort((a, b) => a - b);
  const count = (b: (typeof bs)[number]) => Object.values(b.counts).reduce((n, v) => n + v, 0);
  const total = bs.reduce((n, b) => n + count(b), 0);
  const maint = bs.reduce((n, b) => n + b.counts.maintenance, 0);
  const shown = all.filter((r) => !find || r.code.toLowerCase().includes(find.toLowerCase()));
  const typeList = types.data ?? [];

  if (buildings.isError || rooms.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([buildings.refetch(), rooms.refetch()])} />
      </AppFrame>
    );

  const nextCode = (floor: number) => {
    const onFloor = all
      .filter((r) => r.floor === floor)
      .map((r) => r.code)
      .sort();
    const last = onFloor[onFloor.length - 1];
    return last ? (codeRange(last, 2)[1] ?? last) : `${current?.code ?? ""}${floor}01`;
  };
  const roomRow = (r: Room) => (
    <button
      key={r.id}
      type="button"
      onClick={() => setDialog({ kind: "room", room: r })}
      className="flex min-h-[52px] w-full items-center gap-3 border-t border-border px-4 py-2 text-left"
    >
      <b className="w-[52px] text-[16px]">{r.code}</b>
      <span className="flex-1 text-[14px] text-ink-2">{localized(r.unitType.name)}</span>
      <RoomStatus r={r} />
      <ChevronRight className="size-4 text-muted-foreground" aria-hidden="true" />
    </button>
  );

  const addRoomsBtn = (cls: string) => (
    <Button
      variant="outline"
      size="lg"
      className={cls}
      disabled={!floors.length}
      onClick={() => setDialog({ kind: "rooms", floor: floors[floors.length - 1] })}
    >
      <Plus aria-hidden="true" />
      {t("buildings.addRooms")}
    </Button>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("buildings.title")}
          subtitle={
            wide
              ? tf("buildings.subPc", { b: bs.length, r: total, m: maint })
              : tf("buildings.sub", { b: bs.length, r: total })
          }
          back="/owner/settings"
          right={
            <span className="flex gap-2">
              <Button
                variant="outline"
                size="lg"
                className="font-bold"
                onClick={() => setDialog({ kind: "building" })}
              >
                <Plus aria-hidden="true" />
                <span className="md:hidden">{t("buildings.addBuilding")}</span>
                <span className="hidden md:inline">{t("buildings.addBuildingLong")}</span>
              </Button>
              <span className="hidden md:block">
                {addRoomsBtn(
                  "border-0 bg-primary font-bold text-primary-foreground hover:bg-primary/90",
                )}
              </span>
            </span>
          }
        />
        <div className="flex flex-col gap-3 px-5">
          {buildings.isLoading && <Skeleton className="h-64 rounded-card" aria-busy="true" />}
          {!buildings.isLoading && bs.length === 0 && (
            <EmptyState title={t("buildings.title")} body={t("buildings.empty")} />
          )}
          {current && (
            <>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <ToggleGroup
                  type="single"
                  value={current.id}
                  onValueChange={(v) => v && setId(v)}
                  aria-label={t("buildings.chips")}
                  className="-mx-5 flex w-auto flex-nowrap justify-start gap-2 overflow-x-auto px-5 md:mx-0 md:flex-wrap md:px-0"
                >
                  {bs.map((b) => (
                    <ToggleGroupItem
                      key={b.id}
                      value={b.id}
                      className="relative h-11 shrink-0 rounded-full! border border-border bg-card px-4 text-[15px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
                    >
                      {b.id === current.id && (
                        <SlidingPill
                          id="bld-pill"
                          className="absolute inset-0 rounded-full bg-primary"
                        />
                      )}
                      <span className="relative whitespace-nowrap">
                        {b.name} · {count(b)}
                      </span>
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
                <div className="relative hidden w-[300px] md:block">
                  <Search
                    className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
                    aria-hidden="true"
                  />
                  <Input
                    value={find}
                    onChange={(e) => setFind(e.target.value)}
                    placeholder={t("buildings.find")}
                    aria-label={t("buildings.find")}
                    className="h-11 rounded-full bg-card pl-10"
                  />
                </div>
              </div>

              {/* Phone: floors as native disclosure panels. */}
              <div className="flex flex-col gap-3 md:hidden">
                <p className="flex items-center justify-between text-[14px] text-ink-2">
                  {tf("buildings.buildingLine", {
                    name: current.name,
                    f: floors.length,
                    r: all.length,
                  })}
                  <button
                    type="button"
                    className="font-bold text-primary underline underline-offset-2"
                    onClick={() => setDialog({ kind: "buildingName" })}
                  >
                    {t("buildings.editBuilding")}
                  </button>
                </p>
                {rooms.isLoading && <Skeleton className="h-40 rounded-card" />}
                {floors.map((n, i) => {
                  const here = all.filter((r) => r.floor === n);
                  const m = here.filter((r) => r.status === "MAINTENANCE").length;
                  return (
                    <Card key={n} className="gap-0 overflow-hidden p-0 shadow-none">
                      <details open={i === 0} className="group">
                        <summary className="flex min-h-14 cursor-pointer list-none items-center gap-2 px-4 py-3">
                          <ChevronDown
                            className="size-4 transition-transform group-open:rotate-0 -rotate-90"
                            aria-hidden="true"
                          />
                          <b className="text-[17px]">{tf("buildings.floor", { n })}</b>
                          <span className="text-[13px] text-ink-2">
                            {m
                              ? tf("buildings.floorSubMaint", { r: here.length, m })
                              : tf("buildings.floorSub", { r: here.length })}
                          </span>
                        </summary>
                        {here.map(roomRow)}
                        <button
                          type="button"
                          onClick={() => setDialog({ kind: "rooms", floor: n })}
                          className="flex min-h-[52px] w-full items-center gap-2 border-t border-border px-4 text-[14px] font-bold text-primary"
                        >
                          <Plus className="size-4" aria-hidden="true" />
                          {tf("buildings.addToFloor", { n })}
                        </button>
                      </details>
                    </Card>
                  );
                })}
                <div className="grid grid-cols-2 gap-2">
                  {addRoomsBtn("font-bold")}
                  <Button
                    variant="outline"
                    size="lg"
                    className="border-dashed font-bold"
                    onClick={() => setDialog({ kind: "floor" })}
                  >
                    <Plus aria-hidden="true" />
                    {t("buildings.addFloor")}
                  </Button>
                </div>
              </div>

              {/* Tablet and desktop: one table of the building's rooms. */}
              <FadeIn className="hidden md:block">
                <Card className="gap-0 overflow-x-auto p-0 shadow-none">
                  <table className="w-full min-w-[720px] text-[14px]">
                    <thead>
                      <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                        {(["colRoom", "colFloor", "colType", "colStatus", "colNote"] as const).map(
                          (k) => (
                            <th key={k} scope="col" className="px-3 font-bold first:pl-5">
                              {t(`buildings.${k}`)}
                            </th>
                          ),
                        )}
                        <th scope="col" className="sr-only">
                          {t("buildings.edit")}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {shown.map((r) => (
                        <tr
                          key={r.id}
                          className="border-t border-border transition-colors duration-100 hover:bg-sunken/50"
                        >
                          <td className="px-3 py-4 pl-5 font-bold">{r.code}</td>
                          <td className="px-3">{tf("buildings.floor", { n: r.floor })}</td>
                          <td className="px-3">{localized(r.unitType.name)}</td>
                          <td className="px-3">
                            <RoomStatus r={r} />
                          </td>
                          <td className="px-3 text-ink-2">{r.note}</td>
                          <td className="px-3 pr-5 text-right">
                            <Button
                              variant="ghost"
                              size="lg"
                              className="font-bold text-primary"
                              onClick={() => setDialog({ kind: "room", room: r })}
                            >
                              <Pencil aria-hidden="true" />
                              {t("buildings.edit")}
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {shown.length === 0 && (
                    <p className="p-6 text-center text-muted-foreground">
                      {t("buildings.noMatch")}
                    </p>
                  )}
                </Card>
                <div className="mt-3 flex items-center justify-between gap-3">
                  <p className="text-[13px] text-muted-foreground">
                    {tf("buildings.showing", {
                      n: shown.length,
                      total: all.length,
                      name: current.name,
                      f: floors.length,
                    })}
                  </p>
                  <Button
                    variant="outline"
                    size="lg"
                    className="border-dashed font-bold"
                    onClick={() => setDialog({ kind: "floor" })}
                  >
                    <Plus aria-hidden="true" />
                    {t("buildings.addFloor")}
                  </Button>
                </div>
              </FadeIn>
              <p className="text-[13px] text-muted-foreground md:hidden">{t("buildings.foot")}</p>
              <p className="hidden text-[13px] text-muted-foreground md:block">
                {t("buildings.foot")}
              </p>
            </>
          )}
        </div>
      </main>

      {dialog?.kind === "building" && (
        <BuildingForm types={typeList} buildingsNow={bs.length} onClose={() => setDialog(null)} />
      )}
      {dialog?.kind === "buildingName" && current && (
        <BuildingNameForm id={current.id} name={current.name} onClose={() => setDialog(null)} />
      )}
      {dialog?.kind === "floor" && current && (
        <FloorForm
          buildingId={current.id}
          buildingName={current.name}
          buildingCode={current.code}
          floorsNow={floors.length}
          types={typeList}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog?.kind === "rooms" && current && (
        <RoomsForm
          key={dialog.floor}
          buildingId={current.id}
          buildingName={current.name}
          floor={dialog.floor}
          floorId={all.find((r) => r.floor === dialog.floor)?.floorId}
          nextCode={nextCode(dialog.floor)}
          types={typeList}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog?.kind === "room" && current && (
        <RoomEditForm
          key={dialog.room.id}
          room={dialog.room}
          buildingName={current.name}
          types={typeList}
          onClose={() => setDialog(null)}
        />
      )}
    </AppFrame>
  );
}
