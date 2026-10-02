"use client";

import { useQuery } from "@tanstack/react-query";
import { ChevronRight, Clock, KeyRound, LogOut, User, Wallet } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { FadeIn } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { isReady } from "@/components/shell/nav";
import { TopBar } from "@/components/shell/TopBar";
import { QueryError } from "@/components/StateView";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { lp } from "@/lib/locale";
import { t, tf, type MessageKey } from "@/lib/t";
import { formatClock } from "@/lib/time";
import { useBuildings } from "../rooms/hooks";
import { useMe } from "../session/useMe";
import { homeFor } from "./home";
import { LocaleSwitch } from "./LocaleSwitch";
import { useSignOut } from "@/components/shell/useSignOut";

const useCurrentShift = (enabled: boolean) =>
  useQuery({
    queryKey: ["shift", "current"],
    enabled,
    queryFn: async () => {
      const { data, response } = await api.GET("/v1/shifts/current");
      if (response.status === 404) return null; // no open shift
      if (!data) throw new Error("getCurrentShift failed");
      return data;
    },
  });

export function AccountView() {
  const me = useMe();
  const buildings = useBuildings().data;
  const desk = me.data?.user.role === "RECEPTIONIST";
  const shift = useCurrentShift(desk).data;
  const signOut = useSignOut();

  if (me.isError)
    return (
      <AppFrame>
        <QueryError onRetry={() => void me.refetch()} />
      </AppFrame>
    );
  const u = me.data?.user;
  const access = me.data?.buildingAccess
    .map(
      (a) =>
        `${buildings?.find((b) => b.id === a.buildingId)?.name ?? a.buildingId}: ${t(`account.level${a.level}` as MessageKey)}`,
    )
    .join(" · ");

  return (
    <AppFrame>
      <main className="mx-auto flex w-full max-w-[480px] flex-col gap-3 pb-8">
        <TopBar
          title={t("account.title")}
          back={me.data ? homeFor(me.data.user.role) : undefined}
        />
        <FadeIn className="flex flex-col gap-3 px-5">
          <Card className="gap-3 p-4 shadow-none">
            {u ? (
              <>
                <div className="flex items-center gap-3">
                  <span className="flex size-12 items-center justify-center rounded-full bg-secondary text-primary">
                    <User className="size-5" aria-hidden="true" />
                  </span>
                  <div className="leading-tight">
                    <h2 className="text-lg font-bold">{u.name}</h2>
                    <p className="text-[13px] text-muted-foreground">
                      {t(`rooms.role${u.role}` as MessageKey)} · {me.data?.tenant.name}
                    </p>
                  </div>
                </div>
                {desk && shift && (
                  <Row label={t("account.currentShift")}>
                    {tf("account.since", { time: formatClock(shift.openedAt) })}
                  </Row>
                )}
                {access && <Row label={t("account.access")}>{access}</Row>}
              </>
            ) : (
              <Skeleton className="h-24 w-full" />
            )}
          </Card>
          {isReady("/schedule") && (
            <NavCard
              href="/schedule"
              icon={Clock}
              title={t("account.schedule")}
              sub={t("account.scheduleSub")}
            />
          )}
          <NavCard
            href="/set-pin"
            icon={KeyRound}
            title={t("account.changePin")}
            sub={t("account.changePinSub")}
          />
          <Card className="gap-3 p-4 shadow-none">
            <h2 className="font-bold">{t("auth.language")}</h2>
            <LocaleSwitch long className="w-full" />
          </Card>
          {desk && isReady("/shift") && (
            <NavCard
              href="/shift"
              icon={Wallet}
              title={t("account.endShift")}
              sub={t("account.endShiftSub")}
            />
          )}
          <Button
            variant="outline"
            size="lg"
            onClick={signOut}
            className="mt-6 border-destructive/30 text-destructive hover:text-destructive"
          >
            <LogOut aria-hidden="true" />
            {t("account.signOut")}
          </Button>
          {desk && (
            <p className="flex gap-2 text-[13px] text-muted-foreground">
              <Clock className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
              {t("account.signOutNote")}
            </p>
          )}
        </FadeIn>
      </main>
    </AppFrame>
  );
}

const Row = ({ label, children }: { label: string; children: ReactNode }) => (
  <p className="flex justify-between gap-4 text-sm">
    <span className="text-muted-foreground">{label}</span>
    <span className="text-right font-medium">{children}</span>
  </p>
);

function NavCard({
  href,
  icon: Icon,
  title,
  sub,
}: {
  href: string;
  icon: typeof Clock;
  title: string;
  sub: string;
}) {
  return (
    <Link href={lp(href)} className="block rounded-xl transition-colors hover:bg-secondary/60">
      <Card className="flex-row items-center gap-3 p-4 shadow-none">
        <span className="flex size-10 shrink-0 items-center justify-center rounded-[10px] bg-secondary text-primary">
          <Icon className="size-5" aria-hidden="true" />
        </span>
        <span className="flex-1 leading-tight">
          <b className="block">{title}</b>
          <span className="text-[13px] text-muted-foreground">{sub}</span>
        </span>
        <ChevronRight className="size-4 text-muted-foreground" aria-hidden="true" />
      </Card>
    </Link>
  );
}
