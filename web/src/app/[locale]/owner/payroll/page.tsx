import { Suspense } from "react";
import { PayrollView } from "../../../../features/owner/finance/PayrollView";

export default function Page() {
  return (
    <Suspense>
      <PayrollView />
    </Suspense>
  );
}
