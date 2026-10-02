import { Suspense } from "react";
import { CheckinForm } from "../../../features/stay/CheckinForm";

export default function Page() {
  return (
    <Suspense>
      <CheckinForm />
    </Suspense>
  );
}
