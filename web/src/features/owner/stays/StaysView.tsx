"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, Clock, Download, Search } from "lucide-react";
import dynamic from "next/dynamic";
import type { components } from "@/api/generated/schema";
import { FadeIn, SlidingPill, StaggerList } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { lp } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { clockOf, formatDayMonth, localDay, parseDay } from "../format";
import { activePreset, addDays, DAY, presets, PRESETS, rangeLabel, type Preset } from "../range";
import { useOwnerStays, type StaysFilter } from "./hooks";

const RangePicker = dynamic(() => import("../activity/RangePicker"), { ssr: false });
type Item = components["schemas"]["StayListItem"];

const STATE_TONE: Record<NonNullable<Item["state"]>, string> = {
  IN_STAY: "border-info-line bg-info-bg text-info",
  PAID: "border-ok-line bg-ok-bg text-ok",
  UNPAID: "border-warn-line bg-warn-bg text-warn-ink",
  MISMATCH: "border-warn-line bg-warn-bg text-warn-ink",
  TIME_EDITED: "border-warn-line bg-warn-bg text-warn-ink",
};
const RENTAL: Record<Item["rentalType"], MessageKey> = {
  HOURLY: "rooms.hourly",
  OVERNIGHT: "rooms.overnight",
  DAILY: "rooms.daily",
};

const pill =
  "inline-block rounded-full border px-2.5 py-0.5 text-[12px] font-bold whitespace-nowrap";
const StatePill = ({ s }: { s: Item }) =>
  s.state ? (
    <span className={cn(pill, STATE_TONE[s.state])}>
      {t(`ownerStays.state.${s.state}` as MessageKey)}
    </span>
  ) : null;
const Photo = ({ on, label }: { on: boolean; label: string }) => (
  <span
    className={cn(
      pill,
      on ? "border-ok-line bg-ok-bg text-ok" : "border-line bg-sunken text-muted-foreground",
    )}
  >
    {label}
  </span>
);
const out = (s: Item) => (s.checkOutAt ? clockOf(s.checkOutAt) : t("ownerStays.inStay"));
const pay = (s: Item) =>
  s.paymentMethod === "TRANSFER"
    ? t("owner.transfer")
    : s.paymentMethod === "CASH"
      ? t("owner.cashShort")
      : "—";
const detailHref = (s: Item) => lp(`/owner/stay?id=${encodeURIComponent(s.id)}`);

export function StaysView() {
  const router = useRouter();
  const params = useSearchParams();
  const def = presets().last7;
  const f: StaysFilter = {
    from: params.get("from") ?? def[0],
    to: params.get("to") ?? def[1],
    q: params.get("q") ?? "",
  };
  const set = (next: Partial<StaysFilter>) => {
    const q = new URLSearchParams(params);
    for (const [k, v] of Object.entries(next)) v ? q.set(k, v) : q.delete(k);
    router.replace(lp(`/owner/stays?${q}`));
    setPage(0);
  };
  const [page, setPage] = useState(0);
  const [pickOpen, setPickOpen] = useState(false);
  const [text, setText] = useState(f.q);
  useEffect(() => {
    if (text === f.q) return;
    const id = setTimeout(() => set({ q: text }), 300);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text]);

  const log = useOwnerStays(f);
  const all = log.data?.pages.flatMap((p) => p.items) ?? [];
  const pageRows = log.data?.pages[page]?.items ?? [];
  const active = activePreset(f.from, f.to);
  const hasNext = page + 1 < (log.data?.pages.length ?? 0) || !!log.hasNextPage;
  const pick = (k: Preset) => set({ from: presets()[k][0], to: presets()[k][1] });
  const shift = (dir: -1 | 1) => {
    const days = Math.round((parseDay(f.to).getTime() - parseDay(f.from).getTime()) / DAY) + 1;
    set({
      from: localDay(addDays(parseDay(f.from), dir * days)),
      to: localDay(addDays(parseDay(f.to), dir * days)),
    });
  };
  const goNext = () =>
    page + 1 >= (log.data?.pages.length ?? 0)
      ? void log.fetchNextPage().then(() => setPage(page + 1))
      : setPage(page + 1);

  const csv = (
    <Button variant="outline" size="lg" disabled={!all.length} onClick={() => downloadCsv(all)}>
      <Download aria-hidden="true" />
      <span className="hidden lg:inline">{t("ownerStays.csv")}</span>
      <span className="sr-only lg:hidden">{t("ownerStays.csv")}</span>
    </Button>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("ownerStays.title")}
          subtitle={t("ownerStays.sub")}
          back="/owner"
          right={csv}
        />
        <div className="flex flex-col gap-3 px-5">
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon-lg"
              aria-label={t("activity.prevDay")}
              onClick={() => shift(-1)}
            >
              <ChevronLeft aria-hidden="true" />
            </Button>
            <Button
              variant="outline"
              size="lg"
              onClick={() => setPickOpen(true)}
              className="min-w-0 flex-1 font-bold lg:flex-none"
            >
              <Clock aria-hidden="true" />
              {rangeLabel(f.from, f.to)}
            </Button>
            <Button
              variant="outline"
              size="icon-lg"
              aria-label={t("activity.nextDay")}
              onClick={() => shift(1)}
            >
              <ChevronRight aria-hidden="true" />
            </Button>
            <ToggleGroup
              type="single"
              value={active ?? ""}
              onValueChange={(k) => k && pick(k as Preset)}
              aria-label={t("activity.range")}
              className="hidden gap-0 rounded-card bg-secondary p-1 lg:flex"
            >
              {PRESETS.map((p) => (
                <ToggleGroupItem
                  key={p.key}
                  value={p.key}
                  className="relative h-11 rounded-[10px]! px-4 text-[15px] font-bold data-[state=on]:bg-transparent"
                >
                  {active === p.key && (
                    <SlidingPill
                      id="stays-range"
                      className="absolute inset-0 rounded-[10px] bg-card shadow-sm"
                    />
                  )}
                  <span className="relative">{t(p.short)}</span>
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          <ToggleGroup
            type="single"
            value={active ?? ""}
            onValueChange={(k) => k && pick(k as Preset)}
            aria-label={t("activity.range")}
            className="-mx-5 flex w-auto justify-start gap-2 overflow-x-auto px-5 lg:hidden"
          >
            {PRESETS.map((p) => (
              <ToggleGroupItem
                key={p.key}
                value={p.key}
                className="relative h-11 shrink-0 rounded-full! border border-border bg-card px-4 text-[15px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground"
              >
                {active === p.key && (
                  <SlidingPill
                    id="stays-chip"
                    className="absolute inset-0 rounded-full bg-primary"
                  />
                )}
                <span className="relative whitespace-nowrap">{t(p.label)}</span>
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <div className="relative">
            <Search
              className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder={t("ownerStays.search")}
              aria-label={t("ownerStays.search")}
              className="h-11 rounded-full bg-card pl-10"
            />
          </div>

          {log.isError ? (
            <QueryError onRetry={() => void log.refetch()} />
          ) : log.isLoading ? (
            <Skeleton className="h-64 rounded-card" aria-busy="true" />
          ) : all.length === 0 ? (
            <p className="py-10 text-center text-muted-foreground">{t("ownerStays.empty")}</p>
          ) : (
            <>
              <StaggerList className="flex flex-col gap-2.5 md:hidden">
                {all.map((s) => (
                  <Link key={s.id} href={detailHref(s)} className="block">
                    <Card className="gap-1 p-4 shadow-none">
                      <div className="flex items-center justify-between gap-2">
                        <b className="text-[17px]">
                          {s.roomCode} · {s.guestName}
                        </b>
                        <StatePill s={s} />
                      </div>
                      <p className="text-[13px] text-ink-2">
                        {formatDayMonth(s.checkInAt)} · {clockOf(s.checkInAt)} → {out(s)} ·{" "}
                        {t(RENTAL[s.rentalType])}
                      </p>
                      <p className="flex justify-between text-[13px] text-muted-foreground">
                        <span>{s.frontDeskName}</span>
                        <span>
                          {s.total != null ? formatVnd(s.total) : "—"} · {pay(s)}
                        </span>
                      </p>
                    </Card>
                  </Link>
                ))}
              </StaggerList>
              <FadeIn className="hidden md:block">
                <Card className="gap-0 overflow-x-auto p-0 shadow-none">
                  <table className="w-full min-w-[960px] text-[14px]">
                    <thead>
                      <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                        {(
                          [
                            "date",
                            "room",
                            "guest",
                            "inOut",
                            "amount",
                            "idNumber",
                            "idPhotos",
                            "status",
                          ] as const
                        ).map((k) => (
                          <th key={k} scope="col" className="px-3 font-bold first:pl-5">
                            {t(`ownerStays.${k}`)}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {pageRows.map((s) => (
                        <tr
                          key={s.id}
                          className="relative h-[55px] border-t border-border transition-colors duration-100 hover:bg-sunken/50"
                        >
                          <td className="px-3 pl-5 text-ink-2">{formatDayMonth(s.checkInAt)}</td>
                          <td className="px-3">
                            <Link
                              href={detailHref(s)}
                              className="font-bold underline-offset-2 after:absolute after:inset-0 hover:underline"
                            >
                              {s.roomCode}
                            </Link>
                          </td>
                          <td className="px-3 leading-tight">
                            {s.guestName}
                            <span className="block text-[12px] text-muted-foreground">
                              {s.frontDeskName}
                            </span>
                          </td>
                          <td className="px-3 leading-tight">
                            {clockOf(s.checkInAt)} → {out(s)}
                            <span className="block text-[12px] text-muted-foreground">
                              {t(RENTAL[s.rentalType])}
                            </span>
                          </td>
                          <td className="px-3 leading-tight">
                            <b>{s.total != null ? formatVnd(s.total) : "—"}</b>
                            <span className="block text-[12px] text-muted-foreground">
                              {pay(s)}
                            </span>
                          </td>
                          <td className="px-3">
                            {s.guestId.hasIdNumber ? (
                              <b className="text-ok">✓ {t("ownerStays.yes")}</b>
                            ) : (
                              <span className="text-muted-foreground">{t("ownerStays.no")}</span>
                            )}
                          </td>
                          <td className="px-3">
                            <span className="flex gap-1.5">
                              <Photo on={s.guestId.hasFrontPhoto} label={t("ownerStays.front")} />
                              <Photo on={s.guestId.hasBackPhoto} label={t("ownerStays.back")} />
                            </span>
                          </td>
                          <td className="px-3 pr-5">
                            <StatePill s={s} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </Card>
              </FadeIn>
              <div className="flex items-center justify-between gap-3">
                <p className="text-[13px] text-muted-foreground">
                  {tf("ownerStays.summary", { n: pageRows.length, p: page + 1 })}
                </p>
                <div className="hidden gap-2 md:flex">
                  <Button
                    variant="outline"
                    size="lg"
                    disabled={page === 0}
                    onClick={() => setPage(page - 1)}
                  >
                    {t("activity.prev")}
                  </Button>
                  <Button
                    variant="outline"
                    size="lg"
                    disabled={!hasNext || log.isFetchingNextPage}
                    onClick={goNext}
                  >
                    {t("activity.next")}
                  </Button>
                </div>
                {log.hasNextPage && (
                  <Button
                    variant="ghost"
                    size="lg"
                    className="font-bold text-primary md:hidden"
                    disabled={log.isFetchingNextPage}
                    onClick={() => void log.fetchNextPage()}
                  >
                    {t("activity.more")}
                  </Button>
                )}
              </div>
            </>
          )}
        </div>
      </main>
      {pickOpen && (
        <RangePicker
          key={f.from + f.to}
          open={pickOpen}
          onOpenChange={setPickOpen}
          from={f.from}
          to={f.to}
          onApply={(from, to) => set({ from, to })}
        />
      )}
    </AppFrame>
  );
}

function downloadCsv(rows: Item[]) {
  const cell = (v: string) => `"${v.replace(/"/g, '""')}"`;
  const lines = rows.map((s) =>
    [s.checkInAt, s.roomCode, s.guestName, s.checkOutAt ?? "", String(s.total ?? ""), s.state ?? ""]
      .map(cell)
      .join(","),
  );
  const blob = new Blob(["﻿" + lines.join("\n")], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  Object.assign(document.createElement("a"), { href: url, download: "stays.csv" }).click();
  URL.revokeObjectURL(url);
}
