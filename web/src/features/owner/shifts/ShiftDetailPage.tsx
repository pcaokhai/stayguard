"use client";

import { useSearchParams } from "next/navigation";
import { ShiftsView } from "./ShiftsView";

export function ShiftDetailPage() {
  return <ShiftsView selectedId={useSearchParams().get("id") ?? undefined} />;
}
