import { Suspense } from "react";
import { ReportView } from "../../../../features/owner/finance/ReportView";

export default function Page() {
  return (
    <Suspense>
      <ReportView />
    </Suspense>
  );
}
