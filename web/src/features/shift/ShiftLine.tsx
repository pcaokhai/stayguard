"use client";

import { ChevronDown } from "lucide-react";
import type { components } from "../../api/generated/schema";
import { Badge } from "@/components/ui/badge";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { formatVnd } from "../../lib/money";
import { t, type MessageKey } from "../../lib/t";
import { formatClock } from "../../lib/time";

type Movement = components["schemas"]["ShiftMovement"];
const signed = (n: number) => (n < 0 ? `−${formatVnd(-n)}` : `+${formatVnd(n)}`);

// One line of "cash the system expects" that opens onto the ledger rows behind it (time, room or bill, kind, signed
// amount, and a tag when the owner or a manager made it). The rows are the API's; the browser adds nothing up.
export function ShiftLine({
  label,
  value,
  movements,
  defaultOpen = false,
}: {
  label: string;
  value: string;
  movements: Movement[];
  defaultOpen?: boolean;
}) {
  const head = (
    <>
      <span>{label}</span>
      <span className="flex items-center gap-1.5">
        {value}
        {movements.length > 0 && (
          <ChevronDown
            className="size-4 transition-transform [[data-state=open]_&]:rotate-180"
            aria-hidden="true"
          />
        )}
      </span>
    </>
  );
  const cls = "flex w-full items-center justify-between gap-3 text-[15px] text-ink-2";
  if (!movements.length) return <p className={cls}>{head}</p>;
  return (
    <Collapsible defaultOpen={defaultOpen}>
      <CollapsibleTrigger
        className={`${cls} min-h-9 rounded-md text-left`}
        aria-label={`${label} · ${t("shift.showMovements")}`}
      >
        {head}
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ul className="mt-1 flex flex-col gap-1 border-l-2 border-border pl-3 text-[13px]">
          {movements.map((m, i) => (
            <li key={i} className="flex items-center justify-between gap-3">
              <span className="min-w-0 truncate">
                {formatClock(m.at)} · {m.roomCode ?? m.billCode ?? "—"} ·{" "}
                {t(`shift.mv.${m.kind}` as MessageKey)}
                {m.byOwner && (
                  <Badge variant="info" className="ml-1.5 px-2 py-0 text-[11px]">
                    {t("shift.byOwner")}
                  </Badge>
                )}
              </span>
              <b className="shrink-0 font-semibold">{signed(m.amount)}</b>
            </li>
          ))}
        </ul>
      </CollapsibleContent>
    </Collapsible>
  );
}
