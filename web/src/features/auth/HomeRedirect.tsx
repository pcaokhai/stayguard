"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { api } from "@/lib/api";
import { lp } from "@/lib/locale";
import { homeFor } from "./home";

// "/vi" itself: a valid cookie session goes to its role's home, everyone else to sign-in. The API decides
// (the cookie is HttpOnly), so a second tab or a restarted browser tab lands signed in.
export function HomeRedirect() {
  const router = useRouter();
  useEffect(() => {
    void api
      .GET("/v1/me")
      .then(({ data }) => router.replace(lp(data ? homeFor(data.user.role) : "/sign-in")));
  }, [router]);
  return null;
}
