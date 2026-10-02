import { Suspense } from "react";
import { EditTimeView } from "@/features/stay/EditTimeView";

export default function EditTimePage() {
  return (
    <Suspense>
      <EditTimeView />
    </Suspense>
  );
}
