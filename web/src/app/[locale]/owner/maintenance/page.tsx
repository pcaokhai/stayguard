import { Suspense } from "react";
import { MaintenanceView } from "../../../../features/owner/maintenance/MaintenanceView";

export default function Page() {
  return (
    <Suspense>
      <MaintenanceView />
    </Suspense>
  );
}
