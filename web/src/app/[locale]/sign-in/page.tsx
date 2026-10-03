import { Suspense } from "react";
import { SignInView } from "@/features/auth/SignInView";

export default function SignInPage() {
  return (
    <Suspense>
      <SignInView />
    </Suspense>
  );
}
