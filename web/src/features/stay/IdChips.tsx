"use client";

import { Check, Clock, Lock } from "lucide-react";
import type { components } from "../../api/generated/schema";
import { Badge } from "@/components/ui/badge";
import { t } from "../../lib/t";

type GuestId = components["schemas"]["GuestIdIndicators"];

// ID on file as chips only: never the number or the photos (rules 21-25). The lock marks owner-only data.
export function IdChips({ ids }: { ids: GuestId }) {
  const items = [
    [ids.hasIdNumber, t("panel.idNumber"), t("panel.noIdNumber")],
    [ids.hasFrontPhoto, t("panel.frontPhoto"), t("panel.noFrontPhoto")],
    [ids.hasBackPhoto, t("panel.backPhoto"), t("panel.noBackPhoto")],
  ] as const;
  return (
    <ul className="flex flex-wrap items-center gap-1.5">
      {items.map(([has, yes, no]) => (
        <li key={yes}>
          <Badge variant={has ? "ok" : "maintenance"} className="px-2.5 py-1 font-semibold">
            {has ? <Check aria-hidden="true" /> : <Clock aria-hidden="true" />}
            {has ? yes : no}
          </Badge>
        </li>
      ))}
      <li>
        <Lock className="size-3.5 text-muted-foreground" aria-hidden="true" />
      </li>
    </ul>
  );
}
