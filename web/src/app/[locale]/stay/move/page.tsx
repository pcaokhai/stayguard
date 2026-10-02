import { Suspense } from "react";
import { MoveView } from "@/features/stay/MoveView";

export default function MovePage() {
  return (
    <Suspense>
      <MoveView />
    </Suspense>
  );
}
