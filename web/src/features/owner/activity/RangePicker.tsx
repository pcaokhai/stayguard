"use client";

import { useState } from "react";
import { enGB, vi } from "date-fns/locale";
import type { DateRange } from "react-day-picker";
import { ResponsiveDialog } from "@/components/ResponsiveDialog";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { getLocale } from "@/lib/locale";
import { t, tf } from "@/lib/t";
import { formatDayMonth, localDay, parseDay } from "../format";

// Pick a start day then an end day (board NhatKyChonNgay); loaded on demand, it pulls in react-day-picker.
export default function RangePicker({
  open,
  onOpenChange,
  from,
  to,
  onApply,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  from: string;
  to: string;
  onApply: (from: string, to: string) => void;
}) {
  const [range, setRange] = useState<DateRange | undefined>({
    from: parseDay(from),
    to: parseDay(to),
  });
  const label = range?.from
    ? `${formatDayMonth(localDay(range.from))} – ${formatDayMonth(localDay(range.to ?? range.from))}`
    : "";
  return (
    <ResponsiveDialog open={open} onOpenChange={onOpenChange} title={t("activity.sheetTitle")}>
      <Calendar
        mode="range"
        selected={range}
        onSelect={setRange}
        locale={getLocale() === "en" ? enGB : vi}
        weekStartsOn={1}
        defaultMonth={range?.to ?? range?.from}
        className="mx-auto p-0 [--cell-size:--spacing(11)]"
      />
      <p className="text-center text-[13px] text-muted-foreground">
        {tf("activity.pickHint", { range: label })}
      </p>
      <Button
        size="lg"
        disabled={!range?.from}
        onClick={() => {
          if (!range?.from) return;
          onApply(localDay(range.from), localDay(range.to ?? range.from));
          onOpenChange(false);
        }}
      >
        {tf("activity.apply", { range: label })}
      </Button>
    </ResponsiveDialog>
  );
}
