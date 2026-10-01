import { Suspense } from "react";
import { PaidView } from "../../../features/payment/PaidView";

export default function Page() {
  return (
    <Suspense>
      <PaidView />
    </Suspense>
  );
}
