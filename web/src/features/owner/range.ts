import type { MessageKey } from "@/lib/t";
import { formatDayMonth, localDay } from "./format";

export const DAY = 86_400_000;
export const addDays = (d: Date, n: number) => new Date(d.getTime() + n * DAY);

// ponytail: "today" is the browser's calendar day; the data and its times come from the server.
export function presets() {
  const now = new Date();
  const today = localDay(now);
  return {
    today: [today, today],
    yesterday: [localDay(addDays(now, -1)), localDay(addDays(now, -1))],
    last7: [localDay(addDays(now, -6)), today],
    month: [localDay(new Date(now.getFullYear(), now.getMonth(), 1)), today],
  } as const;
}
export type Preset = keyof ReturnType<typeof presets>;

export const PRESETS: { key: Preset; label: MessageKey; short: MessageKey }[] = [
  { key: "today", label: "activity.today", short: "activity.today" },
  { key: "yesterday", label: "activity.yesterday", short: "activity.yesterday" },
  { key: "last7", label: "activity.last7", short: "activity.last7Short" },
  { key: "month", label: "activity.month", short: "activity.month" },
];

export const rangeLabel = (from: string, to: string) =>
  from === to ? formatDayMonth(from) : `${formatDayMonth(from)} – ${formatDayMonth(to)}`;

export const activePreset = (from: string, to: string) =>
  (Object.entries(presets()) as [Preset, readonly string[]][]).find(
    ([, [a, b]]) => a === from && b === to,
  )?.[0];
