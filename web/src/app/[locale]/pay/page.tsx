import { Suspense } from "react";
import { PayView } from "../../../features/payment/PayView";

export default function Page() {
  return (
    <Suspense>
      <PayView />
    </Suspense>
  );
}
