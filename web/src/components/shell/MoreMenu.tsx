"use client";

import Link from "next/link";
import { LogOut } from "lucide-react";
import { useState, type ReactNode } from "react";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer";
import { useMe } from "@/features/session/useMe";
import { lp } from "@/lib/locale";
import { t, type MessageKey } from "@/lib/t";
import { OWNER_GROUPS, visible } from "./nav";
import { useSignOut } from "./useSignOut";

// Owner "More" (board MenuChu): the same groups as the desktop sidebar, in a bottom sheet.
export function MoreMenu({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const me = useMe().data;
  const signOut = useSignOut();
  return (
    <Drawer open={open} onOpenChange={setOpen}>
      <DrawerTrigger asChild>{children}</DrawerTrigger>
      <DrawerContent className="max-h-[92dvh] bg-card">
        <DrawerTitle className="sr-only">{t("nav.menu")}</DrawerTitle>
        <DrawerDescription className="sr-only">{me?.tenant.name}</DrawerDescription>
        <div className="flex flex-col gap-2 overflow-y-auto px-5 pb-6">
          <div className="flex items-center gap-3 rounded-xl border border-border bg-card p-3.5">
            <span className="flex size-10 items-center justify-center rounded-full bg-info-bg font-bold text-info">
              {me?.user.name.charAt(0)}
            </span>
            <span className="flex-1 leading-tight">
              <b className="block text-[15px]">
                {t(`rooms.role${me?.user.role ?? "OWNER"}` as MessageKey)}
              </b>
              <span className="text-xs text-muted-foreground">{me?.tenant.name}</span>
            </span>
          </div>
          {OWNER_GROUPS.map((g) => {
            const items = visible(g.items);
            if (!items.length) return null;
            return (
              <section key={g.label} aria-label={t(g.label)} className="flex flex-col gap-1.5">
                <h2 className="pt-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
                  {t(g.label)}
                </h2>
                <ul className="grid grid-cols-2 gap-2">
                  {items.map((i) => (
                    <li key={i.href}>
                      <Link
                        href={lp(i.href)}
                        onClick={() => setOpen(false)}
                        className="flex h-11 items-center gap-2.5 rounded-[10px] bg-secondary px-3 text-sm font-bold"
                      >
                        <i.icon className="size-4 shrink-0 text-primary" aria-hidden="true" />
                        <span className="truncate">{t(i.label)}</span>
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            );
          })}
          <button
            type="button"
            onClick={signOut}
            className="mt-3 flex h-11 items-center gap-2 self-start px-1 text-sm font-bold text-destructive"
          >
            <LogOut className="size-4" aria-hidden="true" />
            {t("nav.signOut")}
          </button>
        </div>
      </DrawerContent>
    </Drawer>
  );
}
