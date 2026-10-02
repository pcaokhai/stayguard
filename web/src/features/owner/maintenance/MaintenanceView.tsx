"use client";

import { useSearchParams, useRouter } from "next/navigation";
import { useState } from "react";
import { Info, Search } from "lucide-react";
import { FadeIn, SlidingPill, StaggerList } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { TopBar } from "@/components/shell/TopBar";
import { EmptyState, QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { lp } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { cn } from "@/lib/utils";
import { formatDayMonth } from "../format";
import { type Ticket, useTickets } from "./hooks";
import { StatusPill, TicketSheet } from "./TicketSheet";

type Tab = "open" | "done" | "all";
const isOpen = (x: Ticket) => x.status !== "DONE";
const month = (iso: string) => iso.slice(0, 7);

function Summary({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: "warn" | "orange";
}) {
  return (
    <Card
      className={cn(
        "gap-0.5 p-4 shadow-none",
        tone === "warn" && "border-dirty-line bg-dirty-bg text-dirty",
        tone === "orange" && "border-warn-line bg-warn-bg text-warn-ink",
      )}
    >
      <p className="text-[13px]">{label}</p>
      <p className="text-[26px] font-bold leading-tight">{value}</p>
    </Card>
  );
}

export function MaintenanceView() {
  const router = useRouter();
  const params = useSearchParams();
  const q = useTickets();
  const [tab, setTab] = useState<Tab>("open");
  const [find, setFind] = useState("");
  const all = q.data ?? [];
  const selected = all.find((x) => x.id === params.get("id"));
  const open = all.filter(isOpen);
  const rows = all
    .filter((x) => (tab === "all" ? true : tab === "open" ? isOpen(x) : !isOpen(x)))
    .filter(
      (x) => !find || `${x.roomCode} ${x.description}`.toLowerCase().includes(find.toLowerCase()),
    );
  const thisMonth = new Date().toISOString().slice(0, 7);
  // Display totals over the loaded tickets; the cost per ticket is the API's.
  const monthCost = all
    .filter((x) => x.status === "DONE" && x.completedAt && month(x.completedAt) === thisMonth)
    .reduce((n, x) => n + (x.totalCost ?? 0), 0);
  const noCost = open.filter((x) => x.totalCost == null).length;
  const mm = String(new Date().getMonth() + 1);
  const pick = (id: string | null) =>
    router.replace(lp(id ? `/owner/maintenance?id=${id}` : "/owner/maintenance"));

  if (q.isError)
    return (
      <AppFrame tabs={false}>
        <QueryError onRetry={() => void q.refetch()} />
      </AppFrame>
    );

  const costCell = (x: Ticket) =>
    x.totalCost != null ? (
      <b>{formatVnd(x.totalCost)}</b>
    ) : (
      <span className="font-bold text-warn-ink">{t("maint.notEntered")}</span>
    );

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 pb-10 lg:px-3">
        <TopBar
          title={t("maint.title")}
          subtitle={tf("maint.sub", { n: open.length, m: mm, cost: formatVnd(monthCost) })}
          back="/owner/settings"
        />
        <div className="flex flex-col gap-3 px-5">
          <div className="hidden gap-3 md:grid md:grid-cols-4">
            <Summary tone="warn" label={t("maint.openTickets")} value={String(open.length)} />
            <Summary
              label={t("maint.locked")}
              value={String(open.filter((x) => x.roomLocked).length)}
            />
            <Summary label={tf("maint.costMonth", { m: mm })} value={formatVnd(monthCost)} />
            <Summary tone="orange" label={t("maint.noCost")} value={String(noCost)} />
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <ToggleGroup
              type="single"
              value={tab}
              onValueChange={(v) => v && setTab(v as Tab)}
              aria-label={t("maint.title")}
              className="flex w-full gap-0 rounded-card bg-secondary p-1 md:w-auto md:gap-2 md:bg-transparent md:p-0"
            >
              {(["open", "done", "all"] as const).map((k) => (
                <ToggleGroupItem
                  key={k}
                  value={k}
                  className="relative h-11 flex-1 rounded-[10px]! text-[15px] font-bold data-[state=on]:bg-transparent md:flex-none md:rounded-full! md:border md:border-border md:bg-card md:px-4 md:data-[state=on]:border-transparent md:data-[state=on]:text-primary-foreground"
                >
                  {tab === k && (
                    <SlidingPill
                      id="tk-pill"
                      className="absolute inset-0 rounded-[10px] bg-card shadow-sm md:rounded-full md:bg-primary md:shadow-none"
                    />
                  )}
                  <span className="relative whitespace-nowrap">
                    {k === "open" ? tf("maint.open", { n: open.length }) : t(`maint.${k}`)}
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
                placeholder={t("maint.search")}
                aria-label={t("maint.search")}
                className="h-11 rounded-full bg-card pl-10"
              />
            </div>
          </div>

          {q.isLoading && <Skeleton className="h-48 rounded-card" aria-busy="true" />}
          {!q.isLoading && rows.length === 0 && (
            <EmptyState title={t("maint.title")} body={t("maint.empty")} />
          )}

          <StaggerList className="flex flex-col gap-2.5 md:hidden">
            {rows.map((x) => (
              <button key={x.id} type="button" onClick={() => pick(x.id)} className="text-left">
                <Card className="gap-1.5 p-4 shadow-none">
                  <div className="flex items-start justify-between gap-2">
                    <b className="text-[17px] leading-snug">
                      {x.roomCode} · {x.description}
                    </b>
                    <StatusPill s={x.status} />
                  </div>
                  <p className="flex justify-between text-[13px] text-ink-2">
                    <span>
                      {x.reportedBy} · {formatDayMonth(x.reportedAt)}
                    </span>
                    {x.totalCost != null ? (
                      <b className="text-ink">{formatVnd(x.totalCost)}</b>
                    ) : (
                      <b className="text-warn-ink">{t("maint.noCost")}</b>
                    )}
                  </p>
                </Card>
              </button>
            ))}
          </StaggerList>

          {rows.length > 0 && (
            <FadeIn className="hidden md:block">
              <Card className="gap-0 overflow-x-auto p-0 shadow-none">
                <table className="w-full min-w-[820px] text-[14px]">
                  <thead>
                    <tr className="h-11 text-left text-[12px] uppercase tracking-wide text-ink-2">
                      {(["code", "room", "issue", "status", "expected", "cost"] as const).map(
                        (k) => (
                          <th key={k} scope="col" className="px-3 font-bold first:pl-5">
                            {t(`maint.${k}`)}
                          </th>
                        ),
                      )}
                      <th scope="col" className="sr-only">
                        {t("maint.handle")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((x) => (
                      <tr
                        key={x.id}
                        className="border-t border-border transition-colors duration-100 hover:bg-sunken/50"
                      >
                        <td className="whitespace-nowrap px-3 py-3.5 pl-5 font-mono text-[13px]">
                          {x.code}
                        </td>
                        <td className="px-3 leading-tight">
                          <b className="block text-[15px]">{x.roomCode}</b>
                          {x.roomLocked && x.status !== "DONE" && (
                            <span className="text-[11px] text-warn-ink">
                              {t("maint.roomLocked")}
                            </span>
                          )}
                        </td>
                        <td className="px-3 leading-tight">
                          <b className="block">{x.description}</b>
                          <span className="text-[12px] text-muted-foreground">
                            {x.reportedBy} · {formatDayMonth(x.reportedAt)}
                          </span>
                        </td>
                        <td className="px-3">
                          <StatusPill s={x.status} />
                        </td>
                        <td className="px-3">
                          {x.expectedDoneOn ? formatDayMonth(x.expectedDoneOn) : "—"}
                        </td>
                        <td className="px-3">{costCell(x)}</td>
                        <td className="px-3 pr-5 text-right">
                          <Button
                            variant="outline"
                            size="lg"
                            className="font-bold"
                            onClick={() => pick(x.id)}
                          >
                            {t("maint.handle")}
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Card>
            </FadeIn>
          )}
          <p className="hidden items-start gap-2 text-[13px] text-muted-foreground md:flex">
            <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
            {t("maint.foot")}
          </p>
        </div>
      </main>
      {selected && <TicketSheet key={selected.id} ticket={selected} onClose={() => pick(null)} />}
    </AppFrame>
  );
}
