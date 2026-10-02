import { Suspense } from "react";
import { ItemView } from "../../../../features/owner/items/ItemView";

export default function Page() {
  return (
    <Suspense>
      <ItemView />
    </Suspense>
  );
}
