"use client";

import Link from "next/link";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useMe } from "@/features/session/useMe";
import { lp } from "@/lib/locale";
import { t } from "@/lib/t";
import { cn } from "@/lib/utils";
import { railFor } from "./nav";
import { useActive } from "./useActive";

// Tablet (640-1023 px): 88 px rail. It scrolls when an owner has many pages.
export function IconRail() {
  const role = useMe().data?.user.role;
  const active = useActive();
  const items = railFor(role);
  if (!items.length) return null;
  return (
    <nav
      aria-label={t("nav.main")}
      className="sticky top-0 hidden h-dvh w-[88px] print:hidden shrink-0 border-r border-border bg-card md:block lg:hidden"
    >
      <ScrollArea className="h-full">
        <ul className="flex flex-col items-center gap-1 py-3">
          {items.map((i) => {
            const on = active(i.href);
            return (
              <li key={i.href}>
                <Link
                  href={lp(i.href)}
                  aria-current={on ? "page" : undefined}
                  className={cn(
                    "flex w-[72px] flex-col items-center gap-1 rounded-[10px] px-1 py-2 text-[11px] font-medium text-muted-foreground transition-colors",
                    on && "bg-info-bg font-bold text-primary",
                  )}
                >
                  <i.icon className="size-5" aria-hidden="true" />
                  <span className="max-w-full truncate">{t(i.label)}</span>
                </Link>
              </li>
            );
          })}
        </ul>
      </ScrollArea>
    </nav>
  );
}
