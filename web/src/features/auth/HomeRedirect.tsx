"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { lp } from "@/lib/locale";
import { loadSession } from "@/lib/session";
import { homeFor } from "./home";

// "/vi" itself: a saved session goes to its role's home, everyone else to sign-in.
export function HomeRedirect() {
  const router = useRouter();
  useEffect(() => {
    const s = loadSession();
    router.replace(lp(s ? homeFor(s.user.role) : "/sign-in"));
  }, [router]);
  return null;
}
