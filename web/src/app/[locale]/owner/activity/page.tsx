import { Suspense } from "react";
import { ActivityView } from "../../../../features/owner/activity/ActivityView";

export default function Page() {
  return (
    <Suspense>
      <ActivityView />
    </Suspense>
  );
}
