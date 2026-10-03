"use client";

import { Card } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { t, tf, type MessageKey } from "@/lib/t";
import { cn } from "@/lib/utils";
import { clockOf, formatDayMonth, localDay } from "../format";
import { actionText, actorOf, CATEGORY_TONE, type AuditEntry } from "./text";

function CategoryPill({ e }: { e: AuditEntry }) {
  return (
    <span
      className={cn(
        "inline-block rounded-full border px-2.5 py-0.5 text-[12px] font-bold whitespace-nowrap",
        CATEGORY_TONE[e.category],
      )}
    >
      {t(`activity.cat.${e.category}` as MessageKey)}
    </span>
  );
}

const day = (e: AuditEntry) => localDay(new Date(e.at));

// Phone: cards grouped by day. Desktop: one table page.
export function Rows({ entries, pageRows }: { entries: AuditEntry[]; pageRows: AuditEntry[] }) {
  if (!entries.length)
    return <p className="py-8 text-center text-muted-foreground">{t("activity.empty")}</p>;
  const groups = Object.entries(Object.groupBy(entries, day));
  return (
    <>
      <div className="flex flex-col gap-3 lg:hidden">
        {groups.map(([d, items]) => (
          <Card key={d} className="gap-0 p-0 shadow-none">
            <h2 className="px-4 pb-2 pt-3.5 text-[13px] font-bold text-ink-2">
              {tf("activity.dayHead", { day: formatDayMonth(d), n: items!.length })}
            </h2>
            <ul>
              {items!.map((e) => (
                <li
                  key={e.id}
                  className="grid grid-cols-[48px_1fr] gap-x-3 border-t border-border px-4 py-3"
                >
                  <b className="pt-0.5 text-[13px] text-ink-2">{clockOf(e.at)}</b>
                  <div className="flex flex-col gap-1.5">
                    <p className="text-[15px] leading-snug">{actionText(e)}</p>
                    <p className="flex items-center gap-2 text-[13px] text-muted-foreground">
                      {actorOf(e)}
                      <CategoryPill e={e} />
                    </p>
                  </div>
                </li>
              ))}
            </ul>
          </Card>
        ))}
      </div>
      <Card className="hidden overflow-hidden p-0 shadow-none lg:block">
        <Table>
          <TableHeader>
            <TableRow>
              {(["date", "time", "who", "type", "action"] as const).map((k) => (
                <TableHead
                  key={k}
                  className="h-11 text-[12px] font-bold uppercase tracking-wide first:pl-5"
                >
                  {t(k === "who" ? "activity.who" : `activity.${k}`)}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {pageRows.map((e) => (
              <TableRow key={e.id} className="transition-colors duration-100">
                <TableCell className="py-3 pl-5 text-ink-2">{formatDayMonth(day(e))}</TableCell>
                <TableCell className="font-bold">{clockOf(e.at)}</TableCell>
                <TableCell>{actorOf(e)}</TableCell>
                <TableCell>
                  <CategoryPill e={e} />
                </TableCell>
                <TableCell className="whitespace-normal">{actionText(e)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </>
  );
}
