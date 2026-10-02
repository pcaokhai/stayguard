"use client";

import Link from "next/link";
import {
  Building2,
  ChevronRight,
  Clock,
  Landmark,
  LogOut,
  Package,
  Receipt,
  Shield,
  Tag,
  TriangleAlert,
  UserCog,
  type LucideIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { StaggerList } from "@/components/motion";
import { AppFrame } from "@/components/shell/AppFrame";
import { isReady } from "@/components/shell/nav";
import { TopBar } from "@/components/shell/TopBar";
import { useSignOut } from "@/components/shell/useSignOut";
import { Button } from "@/components/ui/button";
import { localized, lp } from "@/lib/locale";
import { t, tf, type MessageKey } from "@/lib/t";
import { useBuildings } from "../../rooms/hooks";
import { useMe } from "../../session/useMe";
import { useItems } from "../items/hooks";
import { useRatePlans } from "../rates/hooks";
import { useStaff } from "../staff/hooks";
import { useSepayStatus } from "./hooks";

type Row = {
  label: MessageKey;
  href: string;
  icon: LucideIcon;
  sub?: string;
  badge?: { text: string; variant: "ok" | "warn" };
};

export function SettingsView() {
  const me = useMe();
  const buildings = useBuildings();
  const items = useItems();
  const staff = useStaff();
  const rates = useRatePlans();
  const sepay = useSepayStatus();
  const signOut = useSignOut();

  const b = buildings.data ?? [];
  const rooms = b.reduce((n, x) => n + Object.values(x.counts).reduce((a, v) => a + v, 0), 0);
  const maint = b.reduce((n, x) => n + x.counts.maintenance, 0);
  const low = (items.data ?? []).filter(
    (i) => i.lowStockAt != null && i.stock <= i.lowStockAt,
  ).length;
  const locked = (staff.data ?? []).filter((s) => s.status === "LOCKED").length;
  const grace = rates.data?.[0]?.ratePlan.graceMinutes ?? 0;

  const connected = sepay.data?.status === "CONNECTED";
  // Pages of this lane always show; other lanes' pages (roster, maintenance, expenses) once they exist.
  const all: (Row & { always?: boolean })[] = [
    {
      label: "settings.property",
      href: "/owner/property",
      icon: Landmark,
      always: true,
      sub: connected ? t("settings.propertyOn") : t("settings.propertyOff"),
      badge: connected ? { text: t("settings.live"), variant: "ok" } : undefined,
    },
    {
      label: "settings.buildings",
      href: "/owner/buildings",
      icon: Building2,
      always: true,
      sub: tf("settings.buildingsSub", { b: b.length, r: rooms, m: maint }),
    },
    {
      label: "settings.rates",
      href: "/owner/rates",
      icon: Tag,
      always: true,
      sub: tf("settings.ratesSub", {
        types: (rates.data ?? []).map((r) => localized(r.name)).join(", "),
        m: grace,
      }),
    },
    {
      label: "settings.extras",
      href: "/owner/items",
      icon: Package,
      always: true,
      sub: tf("settings.extrasSub", { n: items.data?.length ?? 0, l: low }),
      badge: low ? { text: t("settings.low"), variant: "warn" } : undefined,
    },
    {
      label: "settings.staff",
      href: "/owner/staff",
      icon: UserCog,
      always: true,
      sub: tf("settings.staffSub", { n: staff.data?.length ?? 0, l: locked }),
    },
    {
      label: "settings.access",
      href: "/owner/access",
      icon: Shield,
      always: true,
      sub: t("settings.accessSub"),
    },
    { label: "settings.roster", href: "/owner/roster", icon: Clock },
    { label: "settings.maintenance", href: "/owner/maintenance", icon: TriangleAlert },
    { label: "settings.expenses", href: "/owner/expenses", icon: Receipt },
  ];
  const rows = all.filter((r) => r.always || isReady(r.href));

  return (
    <AppFrame tabs={false}>
      <main className="mx-auto flex w-full max-w-[640px] flex-col gap-3 px-5 pb-10">
        <TopBar title={t("settings.title")} subtitle={me.data?.tenant.name} back="/owner" />
        <StaggerList className="flex flex-col gap-2.5">
          {rows.map((r) => (
            <Link key={r.href} href={lp(r.href)} className="block">
              <Card className="flex-row items-center gap-3 p-3.5 shadow-none">
                <span className="flex size-10 shrink-0 items-center justify-center rounded-[10px] bg-sunken text-brand">
                  <r.icon className="size-5" aria-hidden="true" />
                </span>
                <span className="min-w-0 flex-1 leading-snug">
                  <b className="block text-[16px]">{t(r.label)}</b>
                  {r.sub && <span className="text-[13px] text-ink-2">{r.sub}</span>}
                </span>
                {r.badge && <Badge variant={r.badge.variant}>{r.badge.text}</Badge>}
                <ChevronRight className="size-4 text-muted-foreground" aria-hidden="true" />
              </Card>
            </Link>
          ))}
        </StaggerList>
        <Button
          variant="outline"
          size="lg"
          className="mt-4 border-destructive/30 font-bold text-destructive hover:bg-destructive/10"
          onClick={() => void signOut()}
        >
          <LogOut aria-hidden="true" />
          {t("settings.signOut")}
        </Button>
      </main>
    </AppFrame>
  );
}
