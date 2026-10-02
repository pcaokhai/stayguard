"use client";

import Link from "next/link";
import { ChevronRight } from "lucide-react";
import type { components } from "@/api/generated/schema";
import { lp } from "@/lib/locale";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { formatDuration } from "./format";
import { KIND_TONE } from "./tones";

type Item = components["schemas"]["AttentionItem"];

// Room problems open the room on the map; everything else opens the alert list.
const href = (i: Item) =>
  (i.kind === "OVERDUE_ROOM" || i.kind === "LONG_TO_CLEAN") && i.roomCode
    ? `/owner/rooms?room=${encodeURIComponent(i.roomCode)}`
    : "/owner/alerts";

export function AttentionList({ items }: { items: Item[] }) {
  if (!items.length)
    return <p className="py-3 text-sm text-muted-foreground">{t("owner.noAttention")}</p>;
  return (
    <ul>
      {items.map((i) => (
        <li key={`${i.kind}-${i.ref}`} className="border-t border-border first:border-t-0">
          <Link
            href={lp(href(i))}
            className="flex min-h-14 flex-wrap items-center gap-x-3 gap-y-0.5 py-2.5 pr-6 relative"
          >
            <span
              className={cn(
                "rounded-full border px-3 py-1 text-[13px] font-bold whitespace-nowrap",
                KIND_TONE[i.kind],
              )}
            >
              {tf(`owner.att.${i.kind}` as MessageKey, {
                d: i.minutes != null ? formatDuration(i.minutes) : "",
              }).trim()}
            </span>
            {i.roomCode && <b className="text-[17px]">{i.roomCode}</b>}
            {i.amount != null && (
              <span className="text-[13px] text-ink-2">{formatVnd(i.amount)}</span>
            )}
            <ChevronRight
              className="absolute right-0 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden="true"
            />
          </Link>
        </li>
      ))}
    </ul>
  );
}
