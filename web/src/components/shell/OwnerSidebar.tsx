"use client";

import Link from "next/link";
import { Home, LogOut } from "lucide-react";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useMe } from "@/features/session/useMe";
import { lp } from "@/lib/locale";
import { t } from "@/lib/t";
import { cn } from "@/lib/utils";
import { isOwnerRole, OWNER_GROUPS, visible } from "./nav";
import { useActive } from "./useActive";
import { useSignOut } from "./useSignOut";

const link =
  "flex h-10 items-center gap-2.5 rounded-[10px] px-3 text-sm font-semibold text-ink-2 transition-colors hover:bg-secondary";

// Desktop owner and manager (>= 1024 px): 248 px grouped sidebar.
export function OwnerSidebar() {
  const me = useMe().data;
  const active = useActive();
  const signOut = useSignOut();
  if (!isOwnerRole(me?.user.role)) return null;
  return (
    <nav
      aria-label={t("nav.main")}
      className="sticky top-0 hidden h-dvh w-[248px] shrink-0 border-r border-border bg-card lg:block"
    >
      <ScrollArea className="h-full">
        <div className="flex min-h-dvh flex-col gap-1 p-3">
          <p className="px-3 pb-2 pt-1 leading-tight">
            <b className="block text-[15px]">{me?.tenant.name}</b>
            <span className="text-xs text-muted-foreground">
              {t(`rooms.role${me!.user.role}` as "rooms.roleOWNER")}
            </span>
          </p>
          {visible([{ label: "nav.overview", href: "/owner", icon: Home }]).map((i) => (
            <Link
              key={i.href}
              href={lp(i.href)}
              aria-current={active(i.href) ? "page" : undefined}
              className={cn(link, active(i.href) && "bg-info-bg text-primary")}
            >
              <i.icon className="size-4" aria-hidden="true" />
              {t(i.label)}
            </Link>
          ))}
          {OWNER_GROUPS.map((g) => {
            const items = visible(g.items);
            if (!items.length) return null;
            return (
              <section key={g.label} aria-label={t(g.label)} className="flex flex-col gap-0.5 pt-3">
                <h2 className="px-3 pb-1 text-[11px] font-bold uppercase tracking-wide text-muted-foreground">
                  {t(g.label)}
                </h2>
                {items.map((i) => (
                  <Link
                    key={i.href}
                    href={lp(i.href)}
                    aria-current={active(i.href) ? "page" : undefined}
                    className={cn(link, active(i.href) && "bg-info-bg text-primary")}
                  >
                    <i.icon className="size-4" aria-hidden="true" />
                    {t(i.label)}
                  </Link>
                ))}
              </section>
            );
          })}
          <button type="button" onClick={signOut} className={cn(link, "mt-auto text-destructive")}>
            <LogOut className="size-4" aria-hidden="true" />
            {t("nav.signOut")}
          </button>
        </div>
      </ScrollArea>
    </nav>
  );
}
