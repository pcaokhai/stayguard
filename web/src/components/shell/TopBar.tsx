"use client";

import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import type { ReactNode } from "react";
import { lp } from "@/lib/locale";
import { t } from "@/lib/t";
import { cn } from "@/lib/utils";

// Sub-pages and flows: back, title, optional right slot. On phones this page has no tab bar.
export function TopBar({
  title,
  subtitle,
  back,
  right,
}: {
  title: string;
  subtitle?: string;
  back?: string;
  right?: ReactNode;
}) {
  return (
    <header className={`flex items-center gap-1 pb-3 pt-4 ${back ? "px-2 md:px-5" : "px-5"}`}>
      {back && (
        <Link
          href={lp(back)}
          aria-label={t("stay.back")}
          className={cn(
            "flex size-11 shrink-0 items-center justify-center rounded-full",
            back.startsWith("/owner") && "lg:hidden",
          )}
        >
          <ChevronLeft className="size-6" aria-hidden="true" />
        </Link>
      )}
      <div className="min-w-0 flex-1 leading-tight">
        <h1 className="truncate text-xl font-bold">{title}</h1>
        {subtitle && <p className="truncate text-[13px] text-muted-foreground">{subtitle}</p>}
      </div>
      {right}
    </header>
  );
}
