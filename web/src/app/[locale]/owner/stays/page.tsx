import { Suspense } from "react";
import { StaysView } from "../../../../features/owner/stays/StaysView";

export default function Page() {
  return (
    <Suspense>
      <StaysView />
    </Suspense>
  );
}
