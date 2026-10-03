import type { components } from "../../api/generated/schema";
import { t, type MessageKey } from "../../lib/t";

export type RentalType = components["schemas"]["RentalType"];

const RENTAL: Record<RentalType, MessageKey> = {
  HOURLY: "stay.hourly",
  OVERNIGHT: "stay.overnight",
  DAILY: "stay.daily",
};
export const rentalLabel = (r: RentalType) => t(RENTAL[r]);

// One name per pricing line code (api/internal/domain/pricing); a code we have no text for reads "Other charge".
export const billLineLabel = (code: string) => {
  const key = `bill.${code}`;
  return t((t(key as MessageKey) === key ? "bill.OTHER" : key) as MessageKey);
};
