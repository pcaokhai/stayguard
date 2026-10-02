import { Suspense } from "react";
import { StatesPreview } from "@/features/states/StatesPreview";

export default function StatesPage() {
  return (
    <Suspense>
      <StatesPreview />
    </Suspense>
  );
}
