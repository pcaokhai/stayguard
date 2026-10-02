"use client";

import { Check, Lock, Search } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import type { components } from "../../api/generated/schema";
import { FadeIn } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, QueryError } from "@/components/StateView";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { lp } from "../../lib/locale";
import { formatVnd } from "../../lib/money";
import { t, tf, type MessageKey } from "../../lib/t";
import { formatClock } from "../../lib/time";
import { rentalLabel } from "../stay/labels";
import { DateBar } from "./DateBar";
import { longDate, shiftDay, today, toIso } from "./dates";
import { useStays } from "./hooks";

type Item = components["schemas"]["StayListItem"];

// Front desk sees the last N days (property setting, default 7); the server enforces it.
const FRONT_DESK_DAYS = 7;
const STATE_VARIANT = {
  IN_STAY: "occupied",
  PAID: "ok",
  UNPAID: "warn",
  MISMATCH: "overdue",
  TIME_EDITED: "overdue",
} as const;

const inOut = (s: Item, day: string) => {
  const sameDay = toIso(new Date(s.checkInAt)) === day;
  const from = `${sameDay ? "" : `${new Date(s.checkInAt).toLocaleDateString(undefined, { day: "2-digit", month: "2-digit" })} `}${formatClock(s.checkInAt)}`;
  return `${from} → ${s.checkOutAt ? formatClock(s.checkOutAt) : t("stays.now")}`;
};

export function StayHistory() {
  const router = useRouter();
  const params = useSearchParams();
  const date = params.get("date") ?? today();
  const urlQ = params.get("q") ?? "";
  const [q, setQ] = useState(urlQ);
  const history = useStays(date, urlQ);

  const go = (next: { date?: string; q?: string }) => {
    const p = new URLSearchParams(params);
    if (next.date) p.set("date", next.date);
    if (next.q !== undefined) next.q ? p.set("q", next.q) : p.delete("q");
    router.replace(lp(`/stays?${p}`));
  };
  // Search runs 300 ms after the last keystroke.
  useEffect(() => {
    if (q === urlQ) return;
    const timer = setTimeout(() => go({ q }), 300);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- go reads the current params on purpose
  }, [q]);

  if (history.isError)
    return (
      <AppFrame>
        <QueryError onRetry={() => void history.refetch()} />
      </AppFrame>
    );
  const items = history.data?.pages.flatMap((p) => p.items) ?? [];
  const count = history.hasNextPage ? `${items.length}+` : String(items.length);

  return (
    <AppFrame>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-8">
        <div className="lg:flex lg:items-end lg:justify-between lg:gap-6 lg:px-5 lg:pt-5">
          <TopBar
            title={t("stays.title")}
            subtitle={tf("stays.sub", { date: longDate(date), n: count })}
          />
          <div className="flex flex-col gap-3 px-5 lg:flex-row lg:items-center lg:px-0 lg:pb-3">
            <DateBar
              value={date}
              onChange={(d) => go({ date: d })}
              earliest={shiftDay(today(), -FRONT_DESK_DAYS)}
              compact
            />
            <div className="relative lg:w-[300px]">
              <Search
                className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
                aria-hidden="true"
              />
              <Input
                type="search"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder={t("stays.search")}
                aria-label={t("stays.search")}
                className="h-12 rounded-xl bg-card pl-10"
              />
            </div>
          </div>
        </div>

        {!history.data ? (
          <div className="flex flex-col gap-3 px-5">
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
          </div>
        ) : !items.length ? (
          <EmptyState title={t("stays.empty")} body="" />
        ) : (
          <>
            <ul className="flex flex-col gap-3 px-5 lg:hidden">
              {items.map((s, i) => (
                <li key={s.id}>
                  <FadeIn delay={Math.min(i, 10) * 0.03}>
                    <StayCard s={s} day={date} />
                  </FadeIn>
                </li>
              ))}
            </ul>
            <div className="hidden px-5 lg:block">
              <Card className="overflow-x-auto p-0 shadow-none">
                <Table>
                  <TableHeader>
                    <TableRow>
                      {(
                        [
                          "colRoom",
                          "colGuest",
                          "colInOut",
                          "colAmount",
                          "colIdNumber",
                          "colIdPhotos",
                          "colStatus",
                        ] as const
                      ).map((k) => (
                        <TableHead key={k} className="text-xs font-bold uppercase tracking-wide">
                          {t(`stays.${k}`)}
                        </TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {items.map((s) => (
                      <TableRow key={s.id} className="transition-colors duration-[120ms]">
                        <TableCell className="sticky left-0 bg-card font-bold">
                          {s.roomCode}
                        </TableCell>
                        <TableCell>{s.guestName}</TableCell>
                        <TableCell>
                          {inOut(s, date)}
                          <small className="block text-xs text-muted-foreground">
                            {rentalLabel(s.rentalType)}
                          </small>
                        </TableCell>
                        <TableCell>
                          <b>{s.total == null ? "—" : formatVnd(s.total)}</b>
                        </TableCell>
                        <TableCell>
                          {s.guestId.hasIdNumber ? (
                            <span className="inline-flex items-center gap-1 font-bold text-ok">
                              <Check className="size-3.5" aria-hidden="true" />
                              {t("stays.yes")}
                            </span>
                          ) : (
                            <span className="text-muted-foreground">{t("stays.no")}</span>
                          )}
                        </TableCell>
                        <TableCell>
                          <PhotoChips ids={s.guestId} />
                        </TableCell>
                        <TableCell>
                          <StateBadge s={s} />
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </Card>
            </div>
            {history.hasNextPage && (
              <Button
                type="button"
                variant="ghost"
                className="mx-auto text-primary"
                loading={history.isFetchingNextPage}
                onClick={() => void history.fetchNextPage()}
              >
                {t("stays.loadMore")}
              </Button>
            )}
          </>
        )}
        <p className="flex items-start gap-2 px-5 text-[13px] text-muted-foreground">
          <Lock className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          {tf("stays.foot", { days: FRONT_DESK_DAYS })}
        </p>
      </main>
    </AppFrame>
  );
}

function StateBadge({ s }: { s: Item }) {
  const state = s.state ?? "PAID";
  return (
    <Badge variant={STATE_VARIANT[state]} className="px-3 py-1 font-bold">
      {t(`stays.${state}` as MessageKey)}
    </Badge>
  );
}

// Chips say whether a photo is on file; the photos themselves are owner-only.
function PhotoChips({ ids }: { ids: Item["guestId"] }) {
  return (
    <span className="inline-flex gap-1.5">
      <Badge
        variant={ids.hasFrontPhoto ? "ok" : "maintenance"}
        className={ids.hasFrontPhoto ? "" : "opacity-60"}
      >
        {t("stays.front")}
      </Badge>
      <Badge
        variant={ids.hasBackPhoto ? "ok" : "maintenance"}
        className={ids.hasBackPhoto ? "" : "opacity-60"}
      >
        {t("stays.back")}
      </Badge>
    </span>
  );
}

function StayCard({ s, day }: { s: Item; day: string }) {
  return (
    <Card className="gap-1.5 p-4 shadow-none">
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 truncate text-[17px]">
          <b>{s.roomCode}</b> <span className="text-ink-2">· {s.guestName}</span>
        </p>
        <StateBadge s={s} />
      </div>
      <div className="flex items-baseline justify-between gap-3 text-sm text-muted-foreground">
        <span>
          {rentalLabel(s.rentalType)} · {inOut(s, day)}
        </span>
        <b className="text-foreground">{s.total == null ? "—" : formatVnd(s.total)}</b>
      </div>
      <div className="flex items-center gap-1.5 pt-1 text-xs text-muted-foreground">
        {t("stays.idLabel")}
        {s.guestId.hasIdNumber ? (
          <span className="inline-flex items-center gap-1 font-bold text-ok">
            <Check className="size-3.5" aria-hidden="true" />
            {t("stays.idNumber")}
          </span>
        ) : (
          <span>
            {t("stays.idNumber")}: {t("stays.no")}
          </span>
        )}
        <PhotoChips ids={s.guestId} />
      </div>
    </Card>
  );
}
