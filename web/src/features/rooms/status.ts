import type { components } from "../../api/generated/schema";
import type { MessageKey } from "../../lib/t";

import type { badgeVariants } from "@/components/ui/badge";
import type { VariantProps } from "class-variance-authority";

type BadgeVariant = NonNullable<VariantProps<typeof badgeVariants>["variant"]>;
export type RoomStatus = components["schemas"]["RoomStatus"];

// Every status carries text as well as colour (web/CLAUDE.md).
export const STATUS: Record<
  RoomStatus,
  { label: MessageKey; box: string; pill: string; variant: BadgeVariant }
> = {
  VACANT: {
    label: "rooms.vacant",
    variant: "vacant",
    box: "bg-ok-bg text-ok border-ok-line",
    pill: "bg-ok-bg text-ok border-ok-line",
  },
  OCCUPIED: {
    label: "rooms.occupied",
    variant: "occupied",
    box: "bg-info-bg text-info border-info-line",
    pill: "bg-info-bg text-info border-info-line",
  },
  OVERDUE: {
    label: "rooms.overdue",
    variant: "overdue",
    box: "bg-warn-bg text-warn-ink border-warn-line",
    pill: "bg-warn-bg text-warn-ink border-warn-line",
  },
  TO_CLEAN: {
    label: "rooms.toClean",
    variant: "toClean",
    box: "bg-dirty-bg text-dirty border-dirty-line",
    pill: "bg-dirty-bg text-dirty border-dirty-line",
  },
  MAINTENANCE: {
    label: "rooms.maintenance",
    variant: "maintenance",
    box: "bg-maint-bg text-maint border-line",
    pill: "bg-maint-bg text-maint border-line",
  },
};

export const COUNTER_ORDER: [
  RoomStatus,
  "vacant" | "occupied" | "overdue" | "toClean" | "maintenance",
][] = [
  ["VACANT", "vacant"],
  ["OCCUPIED", "occupied"],
  ["OVERDUE", "overdue"],
  ["TO_CLEAN", "toClean"],
  ["MAINTENANCE", "maintenance"],
];

// 155 minutes -> "2g35" (display only, not a price).
export const formatElapsed = (minutes: number, unit: string) =>
  `${Math.floor(minutes / 60)}${unit}${String(minutes % 60).padStart(2, "0")}`;

// A checked-out stay whose invoice is not paid keeps the room OCCUPIED; the map shows it as awaiting payment.
export const AWAITING_PAYMENT = {
  label: "rooms.awaitingPayment" as MessageKey,
  box: "bg-warn-bg text-warn-ink border-warn-line",
  pill: "bg-warn-bg text-warn-ink border-warn-line",
  variant: "warn" as BadgeVariant,
};
export const tileStatus = (room: components["schemas"]["Room"]) =>
  room.activeStay?.pendingPayment ? AWAITING_PAYMENT : STATUS[room.status];
