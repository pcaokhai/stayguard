import type { components } from "@/api/generated/schema";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { clockOf } from "../format";

export type AuditEntry = components["schemas"]["AuditEntry"];
export type Category = AuditEntry["category"];

export const CATEGORIES: Category[] = [
  "MONEY",
  "STAY_TIME",
  "ACCESS_STAFF",
  "RATES_SETTINGS",
  "SHIFT",
  "STOCK",
  "MAINTENANCE",
  "INSTALLER",
  "GUEST_ID",
];

// Category pill classes (design: Tiền green, Giờ ở orange, Ca olive, Quyền blue, the rest grey).
export const CATEGORY_TONE: Record<Category, string> = {
  MONEY: "border-ok-line bg-ok-bg text-ok",
  STAY_TIME: "border-warn-line bg-warn-bg text-warn-ink",
  SHIFT: "border-dirty-line bg-dirty-bg text-dirty",
  ACCESS_STAFF: "border-info-line bg-info-bg text-info",
  RATES_SETTINGS: "border-line bg-maint-bg text-maint",
  STOCK: "border-line bg-maint-bg text-maint",
  MAINTENANCE: "border-line bg-maint-bg text-maint",
  INSTALLER: "border-line bg-maint-bg text-maint",
  GUEST_ID: "border-line bg-maint-bg text-maint",
};

const MONEY = new Set(["amount", "difference", "expected", "received"]);
const isIso = (v: string) => /^\d{4}-\d\d-\d\dT/.test(v);

// Turns the stable action code plus its details into one line; unknown codes show the code itself.
export function actionText(e: AuditEntry): string {
  const key = `activity.act.${e.action.replace(/\./g, "_")}`;
  const template = t(key as MessageKey);
  if (template === key) return tf("activity.act.fallback", { action: e.action });
  const vars: Record<string, string> = {};
  for (const [k, v] of Object.entries(e.details ?? {})) {
    const money = MONEY.has(k) || (e.action === "rate.updated" && (k === "from" || k === "to"));
    vars[k] = money ? formatVnd(Number(v)) : isIso(v) ? clockOf(v) : v;
  }
  return dropUnresolved(tf(key as MessageKey, vars));
}

// A placeholder with no detail is removed together with the separator before it ("Trả phòng {room}" → "Trả phòng").
const dropUnresolved = (s: string) =>
  s
    .replace(/\s*[:·→\-–,]?\s*\{\w+\}/g, "")
    .replace(/\s{2,}/g, " ")
    .trim();

export const actorOf = (e: AuditEntry) => e.actorName || t("activity.system");

const cell = (v: string) => `"${v.replace(/"/g, '""')}"`;
export const csvLine = (e: AuditEntry) =>
  [e.at, actorOf(e), e.category, actionText(e)].map(cell).join(",");
