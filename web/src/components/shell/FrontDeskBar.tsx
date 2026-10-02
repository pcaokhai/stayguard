"use client";

import Link from "next/link";
import { useMe } from "@/features/session/useMe";
import { lp } from "@/lib/locale";
import { t } from "@/lib/t";
import { cn } from "@/lib/utils";
import { isOwnerRole, tabsFor } from "./nav";
import { useActive } from "./useActive";

// Desktop top bar for front desk and housekeeping (>= 1024 px); the owner has the sidebar instead.
export function FrontDeskBar() {
  const me = useMe().data;
  const active = useActive();
  const items = tabsFor(me?.user.role);
  if (!me || isOwnerRole(me.user.role)) return null;
  return (
    <header className="sticky top-0 z-20 hidden h-14 print:hidden items-center gap-6 border-b border-border bg-card px-6 lg:flex">
      <b className="text-[15px]">{me.tenant.name}</b>
      <nav aria-label={t("nav.main")} className="flex gap-1">
        {items.map((i) => (
          <Link
            key={i.href}
            href={lp(i.href)}
            aria-current={active(i.href) ? "page" : undefined}
            className={cn(
              "flex h-10 items-center gap-2 rounded-[10px] px-3 text-sm font-semibold text-ink-2 hover:bg-secondary",
              active(i.href) && "bg-info-bg text-primary",
            )}
          >
            <i.icon className="size-4" aria-hidden="true" />
            {t(i.label)}
          </Link>
        ))}
      </nav>
      <span className="ml-auto text-sm text-muted-foreground">{me.user.name}</span>
    </header>
  );
}
