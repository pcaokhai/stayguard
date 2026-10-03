import type { components } from "@/api/generated/schema";

export type AlertKind = components["schemas"]["AlertKind"];
export type AttentionKind = components["schemas"]["AttentionItem"]["kind"];

// Pill classes per alert or attention kind; every pill also carries text.
const WARN = "border-warn-line bg-warn-bg text-warn-ink";
const DIRTY = "border-dirty-line bg-dirty-bg text-dirty";
const RED = "border-destructive/30 bg-destructive/10 text-destructive";
const INFO = "border-info-line bg-info-bg text-info";

export const KIND_TONE: Record<AlertKind | AttentionKind, string> = {
  ACCOUNT_LOCKED: RED,
  CASH_OVER: WARN,
  CASH_SHORT: RED,
  DAMAGE_REPORTED: DIRTY,
  LEAVE_REQUESTED: INFO,
  OVERPAID: WARN,
  PAYMENT_MISMATCH: WARN,
  PAYMENT_PARTIAL: WARN,
  PAYMENT_UNPAID: RED,
  SEPAY_UPDATED: INFO,
  STAY_TIME_EDITED: WARN,
  STOCKTAKE_DIFFERENCE: DIRTY,
  UNMATCHED_TRANSFER: WARN,
  UNUSED_ROOM_REPORT: DIRTY,
  OVERDUE_ROOM: WARN,
  LONG_TO_CLEAN: DIRTY,
  LEAVE_PENDING: INFO,
  TICKET_OPEN: DIRTY,
};
