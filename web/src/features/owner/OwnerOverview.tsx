"use client";

import Link from "next/link";
import { Bell } from "lucide-react";
import { FadeIn, Pulse } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { isReady } from "@/components/shell/nav";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { lp } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { cn } from "@/lib/utils";
import { useMe } from "../session/useMe";
import { AttentionList } from "./AttentionList";
import { BuildingRows, Legend } from "./BuildingRows";
import { clockOf, formatDayMonth } from "./format";
import { useOverview } from "./hooks";
import { RollingMoney } from "./Money";

const heading = "text-[13px] font-bold uppercase tracking-wide text-ink-2";
const link = "text-sm font-bold text-primary underline underline-offset-2";

// Phone shortcut chips: only pages that exist show up.
const SHORTCUTS = [
  { label: "owner.shortcutMap", href: "/owner/rooms" },
  { label: "nav.payments", href: "/owner/payments" },
  { label: "nav.reports", href: "/owner/reports" },
  { label: "nav.staff", href: "/owner/staff" },
  { label: "nav.roster", href: "/owner/roster" },
  { label: "nav.maintenance", href: "/owner/maintenance" },
] as const;

function Kpi({
  label,
  sub,
  tone,
  children,
}: {
  label: string;
  sub: string;
  tone?: "ok";
  children: React.ReactNode;
}) {
  return (
    <Card
      className={cn("gap-0.5 p-4 shadow-none", tone === "ok" && "border-ok-line bg-ok-bg text-ok")}
    >
      <p className={cn("text-[13px] sm:text-sm", tone ? "" : "text-muted-foreground")}>{label}</p>
      <p
        className={cn(
          "text-[22px] font-bold leading-tight sm:text-[26px] lg:text-[28px]",
          !tone && "text-ink",
        )}
      >
        {children}
      </p>
      <p className={cn("text-[13px]", tone ? "" : "text-muted-foreground")}>{sub}</p>
    </Card>
  );
}

function Loading() {
  return (
    <div className="flex flex-col gap-3 px-5 pt-5" aria-busy="true">
      <Skeleton className="h-10 w-48" />
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-[104px] rounded-card" />
        ))}
      </div>
      <Skeleton className="h-72 rounded-card" />
    </div>
  );
}

export function OwnerOverview() {
  const q = useOverview();
  const me = useMe();
  const o = q.data;
  if (q.isError)
    return (
      <AppFrame>
        <QueryError onRetry={() => void q.refetch()} />
      </AppFrame>
    );
  if (!o)
    return (
      <AppFrame>
        <Loading />
      </AppFrame>
    );

  const buildings = o.buildings ?? [];
  const attention = o.attention ?? [];
  const pct = Math.round((o.occupancy.occupiedRooms / Math.max(1, o.occupancy.totalRooms)) * 100);
  const updated = tf("owner.updatedAt", { time: clockOf(new Date(q.dataUpdatedAt).toISOString()) });
  const shortcuts = SHORTCUTS.filter((s) => isReady(s.href));

  return (
    <AppFrame>
      <main className="mx-auto flex w-full max-w-[1280px] flex-col gap-3 px-5 pb-8 pt-5 lg:gap-4 lg:px-8 lg:pt-7">
        <FadeIn>
          <header className="flex items-start gap-3">
            <div className="min-w-0 flex-1">
              {me.data && (
                <p className="text-[13px] text-muted-foreground md:hidden">
                  {tf("owner.tenantLine", { name: me.data.tenant.name, n: buildings.length })}
                </p>
              )}
              <h1 className="text-2xl font-bold leading-tight lg:text-[28px]">
                {tf("owner.todayDate", { d: formatDayMonth(o.date) })}
              </h1>
              <p className="text-[13px] text-muted-foreground">{updated}</p>
            </div>
            <Button asChild size="lg" className="hidden lg:inline-flex">
              <Link href={lp("/owner/rooms")}>{t("owner.toRooms")}</Link>
            </Button>
            <Link
              href={lp("/owner/alerts")}
              aria-label={`${t("owner.bell")} ${attention.length}`}
              className="relative flex size-12 shrink-0 items-center justify-center rounded-card border border-border bg-card md:hidden"
            >
              <Bell className="size-5" aria-hidden="true" />
              {attention.length > 0 && (
                <Pulse
                  trigger={attention.length}
                  className="absolute -right-1.5 -top-1.5 flex size-5 items-center justify-center rounded-full bg-destructive text-[11px] font-bold text-white"
                >
                  {attention.length}
                </Pulse>
              )}
            </Link>
          </header>
        </FadeIn>

        <FadeIn delay={0.03}>
          <section
            aria-label={t("owner.revenue")}
            className="grid grid-cols-2 gap-3 lg:grid-cols-4"
          >
            <Kpi
              label={t("owner.revenue")}
              sub={tf("owner.buildingsCount", { n: buildings.length })}
            >
              <RollingMoney value={o.revenueTotal} />
            </Kpi>
            <Kpi tone="ok" label={t("owner.transfers")} sub={t("owner.autoReconciled")}>
              <RollingMoney value={o.transfersReceived} />
            </Kpi>
            <Kpi label={t("owner.cash")} sub={t("owner.inOpenShifts")}>
              <RollingMoney value={o.cashExpected} />
            </Kpi>
            <Kpi
              label={t("owner.guests")}
              sub={`${t("owner.roomsUnit")} · ${tf("owner.occupiedPct", { p: pct })}`}
            >
              {o.occupancy.occupiedRooms}/{o.occupancy.totalRooms}{" "}
            </Kpi>
          </section>
        </FadeIn>

        {shortcuts.length > 0 && (
          <nav aria-label={t("owner.quick")} className="flex gap-2 overflow-x-auto md:hidden">
            {shortcuts.map((s) => (
              <Button key={s.href} asChild variant="outline" size="lg" className="rounded-full">
                <Link href={lp(s.href)}>{t(s.label)}</Link>
              </Button>
            ))}
          </nav>
        )}

        <div className="grid gap-3 lg:grid-cols-2 lg:gap-4">
          <Card className="gap-2 p-5 shadow-none order-2 md:order-1 lg:col-span-2">
            <div className="flex items-baseline justify-between gap-3">
              <h2 className={heading}>{t("owner.byBuilding")}</h2>
              {isReady("/owner/rooms") && (
                <Link href={lp("/owner/rooms")} className={link}>
                  {t("owner.openMap")}
                </Link>
              )}
            </div>
            <Legend />
            <BuildingRows buildings={buildings} />
          </Card>

          <Card className="gap-1 p-5 shadow-none order-1 md:order-2">
            <div className="flex items-baseline justify-between gap-3">
              <h2 className={heading}>{tf("owner.attention", { n: attention.length })}</h2>
              <Link href={lp("/owner/alerts")} className={link}>
                {t("owner.all")}
              </Link>
            </div>
            <AttentionList items={attention} />
          </Card>

          <Card className="gap-1 order-3 self-start p-5 shadow-none">
            <div className="flex items-baseline justify-between gap-3">
              <h2 className={heading}>{t("owner.latest")}</h2>
              {isReady("/owner/payments") && (
                <Link href={lp("/owner/payments")} className={link}>
                  {t("owner.seeAll")}
                </Link>
              )}
            </div>
            {o.latestPayments.length === 0 && (
              <p className="py-3 text-sm text-muted-foreground">{t("owner.noPayments")}</p>
            )}
            <ul>
              {o.latestPayments.map((p) => (
                <li
                  key={p.paymentId}
                  className="flex justify-between gap-3 border-t border-border py-3 text-[15px] first:border-t-0"
                >
                  <span>
                    {p.roomCode || "—"} ·{" "}
                    {p.method === "TRANSFER" ? t("owner.transfer") : t("owner.cashShort")} ·{" "}
                    {clockOf(p.at)}
                  </span>
                  <b>{formatVnd(p.amount)}</b>
                </li>
              ))}
            </ul>
          </Card>
        </div>
      </main>
    </AppFrame>
  );
}
