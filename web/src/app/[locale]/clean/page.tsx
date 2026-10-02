import { Suspense } from "react";
import { CleanView } from "@/features/housekeeping/CleanView";

export default function CleanPage() {
  return (
    <Suspense>
      <CleanView />
    </Suspense>
  );
}
