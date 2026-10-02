import { Suspense } from "react";
import { RosterView } from "../../../../features/owner/roster/RosterView";

export default function Page() {
  return (
    <Suspense>
      <RosterView />
    </Suspense>
  );
}
