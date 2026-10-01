import type { components } from "../../api/generated/schema";
import { t, type MessageKey } from "../../lib/t";

export type RentalType = components["schemas"]["RentalType"];

const RENTAL: Record<RentalType, MessageKey> = {
  HOURLY: "stay.hourly",
  OVERNIGHT: "stay.overnight",
  DAILY: "stay.daily",
};
export const rentalLabel = (r: RentalType) => t(RENTAL[r]);

export const billLineLabel = (code: string) =>
  t(
    (["FIRST_HOUR", "EXTRA_HOUR", "OVERNIGHT", "DAILY"].includes(code)
      ? `bill.${code}`
      : "bill.DAILY") as MessageKey,
  );
