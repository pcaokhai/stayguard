import { Suspense } from "react";
import { ShiftDetailPage } from "../../../../features/owner/shifts/ShiftDetailPage";

export default function Page() {
  return (
    <Suspense>
      <ShiftDetailPage />
    </Suspense>
  );
}
