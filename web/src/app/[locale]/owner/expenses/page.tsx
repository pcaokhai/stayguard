import { Suspense } from "react";
import { ExpensesView } from "../../../../features/owner/finance/ExpensesView";

export default function Page() {
  return (
    <Suspense>
      <ExpensesView />
    </Suspense>
  );
}
