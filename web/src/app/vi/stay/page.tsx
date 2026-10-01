import { Suspense } from "react";
import { StayDetail } from "../../../features/stay/StayDetail";

export default function Page() {
  return (
    <Suspense>
      <StayDetail />
    </Suspense>
  );
}
