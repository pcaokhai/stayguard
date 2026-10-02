import { Suspense } from "react";
import { UsedRoomView } from "@/features/housekeeping/UsedRoomView";

export default function ReportUsedPage() {
  return (
    <Suspense>
      <UsedRoomView />
    </Suspense>
  );
}
