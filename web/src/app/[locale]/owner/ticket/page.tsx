import { Suspense } from "react";
import { TicketRedirect } from "../../../../features/owner/maintenance/TicketRedirect";

export default function Page() {
  return (
    <Suspense>
      <TicketRedirect />
    </Suspense>
  );
}
