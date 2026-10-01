import { Suspense } from "react";
import { CheckoutView } from "../../../features/stay/CheckoutView";

export default function Page() {
  return (
    <Suspense>
      <CheckoutView />
    </Suspense>
  );
}
