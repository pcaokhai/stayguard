import { Suspense } from "react";
import { StayView } from "../../../../features/owner/stays/StayView";

export default function Page() {
  return (
    <Suspense>
      <StayView />
    </Suspense>
  );
}
