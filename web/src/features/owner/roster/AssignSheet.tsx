"use client";

import { toast } from "sonner";
import { FormSheet } from "../FormFields";
import { Button } from "@/components/ui/button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { t, tf, type MessageKey } from "@/lib/t";
import { type Assignment, type ShiftCode, type Roster, usePutRoster } from "./hooks";
import { dm } from "./week";

export type Person = { id: string; name: string };
export const SHIFTS: ShiftCode[] = ["MORNING", "AFTERNOON", "NIGHT"];
export const dayKey = (date: string) =>
  `d${((new Date(`${date}T12:00:00`).getDay() + 6) % 7) + 1}` as const;
export const dayFull = (date: string) => t(`roster.dayFull.${dayKey(date)}` as MessageKey);

// Toggle who works which shift on one day (or one cell of the grid). Each tap saves at once.
export function AssignSheet({
  date,
  staff,
  only,
  roster,
  onClose,
}: {
  date: string;
  staff: Person[];
  only?: string;
  roster: Roster;
  onClose: () => void;
}) {
  const put = usePutRoster();
  const list = only ? staff.filter((s) => s.id === only) : staff;
  const has = (userId: string) =>
    SHIFTS.filter((sh) =>
      roster.assignments.some((a) => a.userId === userId && a.date === date && a.shift === sh),
    );
  const change = (userId: string, next: string[]) => {
    const now = has(userId);
    const a = (shift: ShiftCode): Assignment => ({ userId, date, shift });
    put.mutate(
      {
        set: next.filter((s) => !now.includes(s as ShiftCode)).map((s) => a(s as ShiftCode)),
        remove: now.filter((s) => !next.includes(s)).map(a),
      },
      { onError: () => toast.error(t("roster.saveFailed")) },
    );
  };
  return (
    <FormSheet
      open
      onClose={onClose}
      title={
        only
          ? tf("roster.assignFor", {
              name: list[0]?.name ?? "",
              day: dayFull(date),
              date: dm(date),
            })
          : tf("roster.assignTitle", { day: dayFull(date), date: dm(date) })
      }
    >
      <div className="flex flex-col gap-3 px-4 pb-6">
        {list.map((s) => (
          <div key={s.id} className="flex flex-col gap-1.5 rounded-card border border-border p-3">
            <b className="text-[15px]">{s.name}</b>
            <ToggleGroup
              type="multiple"
              value={has(s.id)}
              onValueChange={(v) => change(s.id, v)}
              aria-label={s.name}
              className="flex gap-2"
            >
              {SHIFTS.map((sh) => (
                <ToggleGroupItem
                  key={sh}
                  value={sh}
                  className="h-11 flex-1 rounded-[10px]! border border-border bg-card text-[14px] font-bold data-[state=on]:border-transparent data-[state=on]:bg-primary data-[state=on]:text-primary-foreground"
                >
                  {t(`roster.shift.${sh}` as MessageKey)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
        ))}
        <Button size="lg" variant="outline" onClick={onClose}>
          {t("roster.done")}
        </Button>
      </div>
    </FormSheet>
  );
}
