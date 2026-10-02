import { Suspense } from "react";
import { ReceiptView } from "@/features/payment/ReceiptView";

export default function ReceiptPage() {
  return (
    <Suspense>
      <ReceiptView />
    </Suspense>
  );
}
