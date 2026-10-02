import { Suspense } from "react";
import { DamageView } from "@/features/housekeeping/DamageView";

export default function ReportDamagePage() {
  return (
    <Suspense>
      <DamageView />
    </Suspense>
  );
}
