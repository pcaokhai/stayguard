"use client";

import { usePathname, useRouter } from "next/navigation";
import { SlidingPill } from "@/components/motion";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { api } from "@/lib/api";
import { getLocale, LOCALES, type Locale } from "@/lib/locale";
import { loadSession } from "@/lib/session";
import { t } from "@/lib/t";
import { cn } from "@/lib/utils";

const LONG: Record<Locale, string> = { vi: "Tiếng Việt", en: "English" };

// Same page in the other language; signed-in users also save the choice (setMyLocale).
export function LocaleSwitch({ long = false, className }: { long?: boolean; className?: string }) {
  const pathname = usePathname();
  const router = useRouter();
  const current = getLocale();
  const go = (next: string) => {
    if (!next || next === current) return;
    if (loadSession()) void api.PUT("/v1/me/locale", { body: { locale: next as Locale } });
    router.replace(`${pathname.replace(/^\/(vi|en)/, `/${next}`)}${window.location.search}`);
  };
  return (
    <ToggleGroup
      type="single"
      value={current}
      onValueChange={go}
      aria-label={t("auth.language")}
      className={cn("gap-0 rounded-xl bg-secondary p-1", className)}
    >
      {LOCALES.map((l) => (
        <ToggleGroupItem
          key={l}
          value={l}
          aria-label={LONG[l]}
          className="relative h-10 flex-1 rounded-[9px] px-5 text-sm font-bold data-[state=on]:bg-transparent"
        >
          {l === current && (
            <SlidingPill
              id="locale-pill"
              className="absolute inset-0 rounded-[9px] bg-card shadow-sm"
            />
          )}
          <span className="relative">{long ? LONG[l] : l.toUpperCase()}</span>
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}
