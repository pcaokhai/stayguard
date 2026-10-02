"use client";

import dynamic from "next/dynamic";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, Clock, Download, Search } from "lucide-react";
import { FadeIn, SlidingPill } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { lp } from "@/lib/locale";
import { t, tf, type MessageKey } from "@/lib/t";
import { formatDayMonth, localDay, parseDay } from "../format";
import { activePreset, addDays, DAY, presets, PRESETS, rangeLabel, type Preset } from "../range";
import { downloadCsv } from "./csv";
import { useAuditLog, useStaffNames, type LogFilter } from "./hooks";
import { Rows } from "./Rows";
import { CATEGORIES } from "./text";

const RangePicker = dynamic(() => import("./RangePicker"), { ssr: false });
export function ActivityView() {
  const router = useRouter();
  const params = useSearchParams();
  const def = presets().last7;
  const f: LogFilter = {
    from: params.get("from") ?? def[0],
    to: params.get("to") ?? def[1],
    who: params.get("who") ?? "",
    cat: params.get("cat") ?? "",
    q: params.get("q") ?? "",
  };
  const set = (next: Partial<LogFilter>) => {
    const q = new URLSearchParams(params);
    for (const [k, v] of Object.entries(next)) v ? q.set(k, v) : q.delete(k);
    router.replace(lp(`/owner/activity?${q}`));
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

  const log = useAuditLog(f);
  const staff = useStaffNames();
  const entries = log.data?.pages.flatMap((p) => p.items) ?? [];
  const pageRows = log.data?.pages[page]?.items ?? [];
  const active = activePreset(f.from, f.to);

  const shift = (dir: -1 | 1) => {
    const days = Math.round((parseDay(f.to).getTime() - parseDay(f.from).getTime()) / DAY) + 1;
    set({
      from: localDay(addDays(parseDay(f.from), dir * days)),
      to: localDay(addDays(parseDay(f.to), dir * days)),
    });
  };
  const goNext = () => {
    if (page + 1 >= (log.data?.pages.length ?? 0))
      void log.fetchNextPage().then(() => setPage(page + 1));
    else setPage(page + 1);
  };
  const hasNext = page + 1 < (log.data?.pages.length ?? 0) || !!log.hasNextPage;

  const personSelect = (
    <label className="flex flex-col gap-1 text-[12px] font-bold text-ink-2">
      {t("activity.who")}
      <Select value={f.who || "all"} onValueChange={(v) => set({ who: v === "all" ? "" : v })}>
        <SelectTrigger className="h-12 w-full bg-card text-[15px] font-bold">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("activity.everyone")}</SelectItem>
          {staff.data?.map((s) => (
            <SelectItem key={s.id} value={s.id}>
              {s.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </label>
  );
  const categorySelect = (
    <label className="flex flex-col gap-1 text-[12px] font-bold text-ink-2">
      {t("activity.category")}
      <Select value={f.cat || "all"} onValueChange={(v) => set({ cat: v === "all" ? "" : v })}>
        <SelectTrigger className="h-12 w-full bg-card text-[15px] font-bold">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("activity.allCategories")}</SelectItem>
          {CATEGORIES.map((c) => (
            <SelectItem key={c} value={c}>
              {t(`activity.cat.${c}` as MessageKey)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </label>
  );
  const searchBox = (
    <div className="relative">
      <Search
        className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
        aria-hidden="true"
      />
      <Input
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={t("activity.search")}
        aria-label={t("activity.search")}
        className="h-11 rounded-full bg-card pl-10"
      />
    </div>
  );
  const csv = (
    <Button
      variant="outline"
      size="lg"
      disabled={!entries.length}
      onClick={() => downloadCsv(entries, `activity-${f.from}_${f.to}.csv`)}
    >
      <Download aria-hidden="true" />
      <span className="hidden lg:inline">{t("activity.csv")}</span>
      <span className="sr-only lg:hidden">{t("activity.csvShort")}</span>
    </Button>
  );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("activity.title")}
          subtitle={t("activity.sub")}
          back="/owner"
          right={csv}
        />
        <div className="flex flex-col gap-3 px-5">
          <div className="flex items-end gap-2 lg:gap-4">
            <div className="flex min-w-0 flex-1 items-center gap-2 lg:flex-none">
              <Button
                variant="outline"
                size="icon-lg"
                aria-label={t("activity.prevDay")}
                onClick={() => shift(-1)}
                className="lg:hidden"
              >
                <ChevronLeft aria-hidden="true" />
              </Button>
              <div className="flex min-w-0 flex-1 flex-col gap-1 lg:flex-none">
                <span className="hidden text-[12px] font-bold text-ink-2 lg:block">
                  {t("activity.range")}
                </span>
                <Button
                  variant="outline"
                  size="lg"
                  onClick={() => setPickOpen(true)}
                  className="w-full font-bold lg:w-auto"
                >
                  <Clock aria-hidden="true" />
                  {rangeLabel(f.from, f.to)}
                </Button>
              </div>
              <Button
                variant="outline"
                size="icon-lg"
                aria-label={t("activity.nextDay")}
                onClick={() => shift(1)}
                className="lg:hidden"
              >
                <ChevronRight aria-hidden="true" />
              </Button>
            </div>
            <ToggleGroup
              type="single"
              value={active ?? ""}
              onValueChange={(k) =>
                k && set({ from: presets()[k as Preset][0], to: presets()[k as Preset][1] })
              }
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
                      id="log-range"
                      className="absolute inset-0 rounded-[10px] bg-card shadow-sm"
                    />
                  )}
                  <span className="relative">{t(p.short)}</span>
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <div className="hidden w-[180px] lg:block">{personSelect}</div>
            <div className="hidden w-[180px] lg:block">{categorySelect}</div>
          </div>

          <ToggleGroup
            type="single"
            value={active ?? ""}
            onValueChange={(k) =>
              k && set({ from: presets()[k as Preset][0], to: presets()[k as Preset][1] })
            }
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
                  <SlidingPill id="log-chip" className="absolute inset-0 rounded-full bg-primary" />
                )}
                <span className="relative whitespace-nowrap">{t(p.label)}</span>
              </ToggleGroupItem>
            ))}
          </ToggleGroup>

          <div className="grid grid-cols-2 gap-3 lg:hidden">
            {personSelect}
            {categorySelect}
          </div>
          <div className="lg:hidden">{searchBox}</div>
          <div className="hidden justify-end lg:flex">
            <div className="w-[400px]">{searchBox}</div>
          </div>

          {log.isError ? (
            <QueryError onRetry={() => void log.refetch()} />
          ) : log.isLoading ? (
            <Skeleton className="h-64 rounded-card" aria-busy="true" />
          ) : (
            <FadeIn key={`${f.from}${f.to}${f.who}${f.cat}${f.q}`}>
              <p className="mb-2 text-[13px] text-muted-foreground">
                <span className="lg:hidden">
                  {tf("activity.summary", { range: rangeLabel(f.from, f.to), n: entries.length })}
                </span>
                <span className="hidden lg:inline">
                  {tf("activity.summaryPc", { n: pageRows.length, p: page + 1 })}
                </span>
              </p>
              <Rows entries={entries} pageRows={pageRows} />
              <div className="mt-3 flex justify-center lg:hidden">
                {log.hasNextPage && (
                  <Button
                    variant="ghost"
                    size="lg"
                    className="font-bold text-primary"
                    disabled={log.isFetchingNextPage}
                    onClick={() => void log.fetchNextPage()}
                  >
                    {t("activity.more")}
                  </Button>
                )}
              </div>
              <div className="mt-3 hidden justify-end gap-2 lg:flex">
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
            </FadeIn>
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
