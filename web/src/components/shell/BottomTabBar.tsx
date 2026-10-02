"use client";

import Link from "next/link";
import { Menu } from "lucide-react";
import { SlidingPill } from "@/components/motion";
import { lp } from "@/lib/locale";
import { t } from "@/lib/t";
import { cn } from "@/lib/utils";
import { MoreMenu } from "./MoreMenu";
import { isOwnerRole, tabsFor } from "./nav";
import { useActive } from "./useActive";
import { useMe } from "@/features/session/useMe";

const tab =
  "relative flex h-14 flex-1 flex-col items-center justify-center gap-0.5 text-[11px] font-medium text-muted-foreground";

// Phone only: 56 px plus the device safe area. The active pill slides between tabs (layoutId).
export function BottomTabBar() {
  const role = useMe().data?.user.role;
  const active = useActive();
  const tabs = tabsFor(role);
  if (!tabs.length && !isOwnerRole(role)) return null;
  return (
    <nav
      aria-label={t("nav.main")}
      className="fixed inset-x-0 bottom-0 z-30 flex print:hidden border-t border-border bg-card pb-[env(safe-area-inset-bottom)] md:hidden"
    >
      {tabs.map((i) => {
        const on = active(i.href);
        return (
          <Link
            key={i.href}
            href={lp(i.href)}
            aria-current={on ? "page" : undefined}
            className={cn(tab, on && "font-bold text-primary")}
          >
            <span className="relative flex h-7 w-[52px] items-center justify-center">
              {on && (
                <SlidingPill id="tab-pill" className="absolute inset-0 rounded-full bg-info-bg" />
              )}
              <i.icon className="relative size-5" aria-hidden="true" />
            </span>
            {t(i.label)}
          </Link>
        );
      })}
      {isOwnerRole(role) && (
        <MoreMenu>
          <button type="button" className={tab}>
            <span className="flex h-7 w-[52px] items-center justify-center">
              <Menu className="size-5" aria-hidden="true" />
            </span>
            {t("nav.more")}
          </button>
        </MoreMenu>
      )}
    </nav>
  );
}
