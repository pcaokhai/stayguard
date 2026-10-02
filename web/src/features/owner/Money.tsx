"use client";

import NumberFlow from "@number-flow/react";
import { getLocale } from "@/lib/locale";

// Dashboard totals roll to their new value (docs/16 §5); never used on pay, bill, receipt or checkout.
export function RollingMoney({ value }: { value: number }) {
  const en = getLocale() === "en";
  return (
    <NumberFlow
      value={value}
      locales={en ? "en-US" : "vi-VN"}
      prefix={en ? "₫" : undefined}
      suffix={en ? undefined : "đ"}
    />
  );
}
