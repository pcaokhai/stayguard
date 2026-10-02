import { Suspense } from "react";
import { StayHistory } from "@/features/history/StayHistory";

export default function StaysPage() {
  return (
    <Suspense>
      <StayHistory />
    </Suspense>
  );
}
