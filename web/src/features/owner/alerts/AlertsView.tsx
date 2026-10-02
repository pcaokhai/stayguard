"use client";

import Link from "next/link";
import { useState } from "react";
import { Check } from "lucide-react";
import { toast } from "sonner";
import { FadeIn, SlidingPill, StaggerList } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { lp } from "@/lib/locale";
import { t, tf } from "@/lib/t";
import { cn } from "@/lib/utils";
import { clockOf, formatDayMonth, localDay } from "../format";
import { KIND_TONE } from "../tones";
import { useAlerts, useMarkRead } from "./hooks";
import { alertDetails, alertHref, FILTERS, kindLabel, roomOrShift, type Alert } from "./text";

type Filter = "unread" | "all" | keyof typeof FILTERS;
const CHIPS: { value: Filter; extra?: boolean }[] = [
  { value: "unread" },
  { value: "all" },
  { value: "money", extra: true },
  { value: "stay", extra: true },
  { value: "hk", extra: true },
];

function KindPill({ a }: { a: Alert }) {
  return (
    <span
      className={cn(
        "inline-block rounded-full border px-3 py-1 text-[13px] font-bold whitespace-nowrap",
        KIND_TONE[a.kind],
      )}
    >
      {kindLabel(a.kind)}
    </span>
  );
}

const by = (a: Alert) => a.actorName || t("alerts.system");

export function AlertsView() {
  const [filter, setFilter] = useState<Filter>("unread");
  const unread = useAlerts(true);
  const all = useAlerts(false);
  const mark = useMarkRead();
  const unreadIds = new Set(unread.data?.map((a) => a.id));
  const source = filter === "unread" ? unread : all;

  const rows = (source.data ?? []).filter((a) => {
    if (filter === "unread" || filter === "all") return true;
    return (FILTERS[filter] as readonly string[]).includes(a.kind);
  });
  const read = (ids: string[]) =>
    mark.mutate(ids, { onError: () => toast.error(t("alerts.failed")) });
  const today = tf("owner.todayDate", { d: formatDayMonth(localDay(new Date())) });
  const count = unread.data?.length ?? 0;

  if (unread.isError || all.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void Promise.all([unread.refetch(), all.refetch()])} />
      </AppFrame>
    );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col px-0 pb-10 lg:px-3">
        <div className="flex items-start gap-3 lg:pr-5">
          <div className="min-w-0 flex-1">
            <TopBar
              title={t("alerts.title")}
              subtitle={tf("alerts.sub", { date: today, n: count })}
              back="/owner"
            />
          </div>
          {count > 0 && (
            <Button
              variant="outline"
              size="lg"
              className="mt-4 hidden md:inline-flex"
              disabled={mark.isPending}
              onClick={() => read(unread.data!.map((a) => a.id))}
            >
              <Check aria-hidden="true" />
              {t("alerts.markAll")}
            </Button>
          )}
        </div>

        <div className="px-5">
          <ToggleGroup
            type="single"
            value={filter}
            onValueChange={(v) => v && setFilter(v as Filter)}
            aria-label={t("alerts.title")}
            className="mb-3 flex w-full gap-2 overflow-x-auto"
          >
            {CHIPS.map((c) => (
              <ToggleGroupItem
                key={c.value}
                value={c.value}
                className={cn(
                  "relative h-11 shrink-0 rounded-full! border border-border bg-card px-4 text-[15px] font-semibold data-[state=on]:border-transparent data-[state=on]:bg-transparent data-[state=on]:text-primary-foreground max-md:flex-1",
                  c.extra && "max-md:hidden",
                )}
              >
                {filter === c.value && (
                  <SlidingPill
                    id="alerts-pill"
                    className="absolute inset-0 rounded-full bg-primary"
                  />
                )}
                <span className="relative whitespace-nowrap">
                  {c.value === "unread"
                    ? tf("alerts.unread", { n: count })
                    : t(
                        `alerts.${c.value === "stay" ? "stayTime" : c.value === "hk" ? "housekeeping" : c.value}`,
                      )}
                </span>
              </ToggleGroupItem>
            ))}
          </ToggleGroup>

          {source.isLoading && (
            <div className="flex flex-col gap-3" aria-busy="true">
              {[0, 1, 2].map((i) => (
                <Skeleton key={i} className="h-36 rounded-card" />
              ))}
            </div>
          )}

          {!source.isLoading && rows.length === 0 && (
            <EmptyState title={t("alerts.emptyTitle")} body={t("alerts.empty")} />
          )}

          {rows.length > 0 && (
            <>
              <StaggerList className="flex flex-col gap-3 md:hidden">
                {rows.map((a) => (
                  <Card key={a.id} className="gap-1.5 p-4 shadow-none">
                    <div className="flex items-center justify-between gap-2">
                      <KindPill a={a} />
                      <span className="flex items-center gap-1.5 text-[13px] text-ink-2">
                        {unreadIds.has(a.id) && (
                          <span
                            className="size-2 rounded-full bg-primary"
                            role="img"
                            aria-label={t("alerts.unreadDot")}
                          />
                        )}
                        {clockOf(a.createdAt)}
                      </span>
                    </div>
                    <p className="text-[17px] font-bold">{roomOrShift(a)}</p>
                    <p className="text-[15px]">{alertDetails(a)}</p>
                    <p className="text-[13px] text-muted-foreground">
                      {tf("alerts.by", { name: by(a) })}
                    </p>
                    <div className="mt-1 grid grid-cols-2 gap-2">
                      <Button asChild variant="outline" size="lg">
                        <Link href={lp(alertHref(a))}>{t("alerts.view")}</Link>
                      </Button>
                      <Button
                        variant="ghost"
                        size="lg"
                        disabled={!unreadIds.has(a.id) || mark.isPending}
                        onClick={() => read([a.id])}
                        className="font-bold text-primary"
                      >
                        <Check aria-hidden="true" />
                        {t("alerts.markRead")}
                      </Button>
                    </div>
                  </Card>
                ))}
              </StaggerList>

              <FadeIn className="hidden md:block">
                <Card className="overflow-hidden p-0 shadow-none">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        {(["time", "type", "roomShift", "details", "who"] as const).map((k) => (
                          <TableHead
                            key={k}
                            className="h-11 text-[12px] font-bold uppercase tracking-wide first:pl-5"
                          >
                            {t(`alerts.${k}`)}
                          </TableHead>
                        ))}
                        <TableHead />
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {rows.map((a) => (
                        <TableRow key={a.id} className="transition-colors duration-100">
                          <TableCell className="py-3 pl-5">
                            <span className="flex items-center gap-1.5">
                              {unreadIds.has(a.id) && (
                                <span
                                  className="size-2 rounded-full bg-primary"
                                  role="img"
                                  aria-label={t("alerts.unreadDot")}
                                />
                              )}
                              {clockOf(a.createdAt)}
                            </span>
                          </TableCell>
                          <TableCell>
                            <KindPill a={a} />
                          </TableCell>
                          <TableCell className="font-bold">{roomOrShift(a)}</TableCell>
                          <TableCell className="max-w-[320px] whitespace-normal text-[14px]">
                            {alertDetails(a)}
                          </TableCell>
                          <TableCell>{by(a)}</TableCell>
                          <TableCell className="pr-5">
                            <span className="flex justify-end gap-2">
                              <Button asChild variant="outline" size="lg">
                                <Link href={lp(alertHref(a))}>{t("alerts.viewShort")}</Link>
                              </Button>
                              <Button
                                variant="outline"
                                size="icon-lg"
                                aria-label={t("alerts.markRead")}
                                disabled={!unreadIds.has(a.id) || mark.isPending}
                                onClick={() => read([a.id])}
                              >
                                <Check aria-hidden="true" />
                              </Button>
                            </span>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </Card>
              </FadeIn>
              <p className="mt-3 hidden text-[13px] text-muted-foreground md:block">
                {t("alerts.note")}
              </p>
            </>
          )}
        </div>
      </main>
    </AppFrame>
  );
}
