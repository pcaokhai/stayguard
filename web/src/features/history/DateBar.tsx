"use client";

import { ChevronLeft, ChevronRight, Clock } from "lucide-react";
import { useState } from "react";
import { vi as viLocale } from "date-fns/locale";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { getLocale } from "../../lib/locale";
import { t } from "../../lib/t";
import { fromIso, longDate, shiftDay, today, toIso } from "./dates";

// Previous and next day, a calendar popover and the Today and Yesterday shortcuts (docs/15 rule 5).
// `earliest` is the oldest day the caller may open (front desk: the last N days).
export function DateBar({
  value,
  onChange,
  earliest,
  compact = false,
}: {
  value: string;
  onChange: (iso: string) => void;
  earliest?: string;
  compact?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const now = today();
  const canPrev = !earliest || value > earliest;
  const canNext = value < now;
  const chip = (iso: string, label: string) => (
    <Button
      type="button"
      size="sm"
      variant={value === iso ? "default" : "outline"}
      className="h-10 rounded-full! px-4 font-bold"
      onClick={() => onChange(iso)}
    >
      {label}
    </Button>
  );

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="icon"
          aria-label={t("stays.prevDay")}
          disabled={!canPrev}
          onClick={() => onChange(shiftDay(value, -1))}
        >
          <ChevronLeft />
        </Button>
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button type="button" variant="outline" className="flex-1 font-bold">
              <Clock aria-hidden="true" />
              {longDate(value)}
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-auto p-0" align="center">
            <Calendar
              mode="single"
              selected={fromIso(value)}
              locale={getLocale() === "vi" ? viLocale : undefined}
              defaultMonth={fromIso(value)}
              disabled={[
                { after: fromIso(now) },
                ...(earliest ? [{ before: fromIso(earliest) }] : []),
              ]}
              onSelect={(d) => {
                if (d) onChange(toIso(d));
                setOpen(false);
              }}
            />
          </PopoverContent>
        </Popover>
        <Button
          type="button"
          variant="outline"
          size="icon"
          aria-label={t("stays.nextDay")}
          disabled={!canNext}
          onClick={() => onChange(shiftDay(value, 1))}
        >
          <ChevronRight />
        </Button>
        {!compact && (
          <div className="ml-2 hidden gap-1 rounded-full bg-secondary p-1 lg:flex">
            {chip(now, t("stays.today"))}
            {chip(shiftDay(now, -1), t("stays.yesterday"))}
          </div>
        )}
      </div>
      {compact && (
        <div className="flex flex-wrap gap-2">
          {chip(now, t("stays.today"))}
          {chip(shiftDay(now, -1), t("stays.yesterday"))}
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-10 rounded-full! px-4 font-bold"
            onClick={() => setOpen(true)}
          >
            {t("stays.pick")}
          </Button>
        </div>
      )}
    </div>
  );
}
