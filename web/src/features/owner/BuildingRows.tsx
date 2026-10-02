"use client";

import { motion } from "motion/react";
import type { components } from "@/api/generated/schema";
import { duration, ease, STAGGER, STAGGER_MAX } from "@/lib/motion";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { cn } from "@/lib/utils";
import { RollingNumber } from "@/components/motion";

type Building = components["schemas"]["BuildingStatus"];

// One entry per room status: bar segment colour, count colour, label key (same order as the design).
const PARTS = [
  { key: "occupied", bar: "bg-info", ink: "text-info", label: "rooms.occupied" },
  { key: "overdue", bar: "bg-warn", ink: "text-warn", label: "rooms.overdue" },
  { key: "toClean", bar: "bg-dirty", ink: "text-dirty", label: "rooms.toClean" },
  { key: "maintenance", bar: "bg-maint", ink: "text-maint", label: "rooms.maintenance" },
  { key: "vacant", bar: "bg-ok-line", ink: "text-ok", label: "rooms.vacant" },
] as const;
// Counts are shown in the design's order: occupied, vacant, to clean, overdue, maintenance.
const COUNTS = [PARTS[0], PARTS[4], PARTS[2], PARTS[1], PARTS[3]];

export function Legend() {
  return (
    <ul className="flex flex-wrap gap-x-3.5 gap-y-1 text-[13px] text-ink-2">
      {PARTS.map((p) => (
        <li key={p.key} className="flex items-center gap-1.5">
          <span className={cn("size-2.5 rounded-[3px]", p.bar)} aria-hidden="true" />
          {t(p.label)}
        </li>
      ))}
    </ul>
  );
}

// Segments grow from the left with transform only (docs/16 §4 rule 1), staggered per building.
function Bar({ b, index }: { b: Building; index: number }) {
  return (
    <div className="flex h-3 gap-px overflow-hidden rounded-full" role="img" aria-label={b.code}>
      {PARTS.map((p) => {
        const n = b[p.key];
        if (!n) return null;
        return (
          <motion.span
            key={p.key}
            className={cn("h-full origin-left", p.bar)}
            style={{ width: `${(n / b.totalRooms) * 100}%` }}
            initial={{ scaleX: 0 }}
            animate={{ scaleX: 1 }}
            transition={{
              duration: duration.slow,
              ease: ease.standard,
              delay: Math.min(index, STAGGER_MAX) * STAGGER,
            }}
          />
        );
      })}
    </div>
  );
}

export function BuildingRows({ buildings }: { buildings: Building[] }) {
  return (
    <ul>
      {buildings.map((b, i) => (
        <li
          key={b.buildingId}
          className="grid gap-2 border-t border-border py-3.5 first:border-t-0 lg:grid-cols-[150px_minmax(140px,1fr)_auto_130px] lg:items-center lg:gap-6 lg:py-3"
        >
          <div className="flex items-end justify-between gap-3 lg:contents">
            <div className="lg:order-1">
              <p className="text-[17px] font-bold leading-tight">
                {tf("owner.building", { code: b.code })}
              </p>
              <p className="text-[13px] text-muted-foreground">
                {tf("owner.buildingRooms", {
                  n: b.totalRooms,
                  p: Math.round(b.occupancyPct),
                })}
              </p>
            </div>
            <p className="text-right lg:order-4">
              <b className="block text-[17px]">{formatVnd(b.revenueToday)}</b>
              <span className="text-[12px] text-muted-foreground">{t("owner.revenueToday")}</span>
            </p>
          </div>
          <div className="lg:order-2">
            <Bar b={b} index={i} />
          </div>
          <dl className="grid grid-cols-5 gap-x-2 lg:order-3 lg:gap-x-5">
            {COUNTS.map((p) => (
              <div key={p.key}>
                <dd
                  className={cn(
                    "text-xl font-bold leading-tight lg:text-[17px]",
                    b[p.key] ? p.ink : "text-muted-foreground/70",
                  )}
                >
                  <RollingNumber value={b[p.key]} />
                </dd>
                <dt className="whitespace-nowrap text-[12px] text-ink-2">
                  {p.key === "maintenance" ? t("owner.maintShort") : t(p.label)}
                </dt>
              </div>
            ))}
          </dl>
        </li>
      ))}
    </ul>
  );
}
