"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { lp } from "@/lib/locale";
import { clearSession } from "@/lib/session";

export function useSignOut() {
  const router = useRouter();
  const qc = useQueryClient();
  return () => {
    clearSession();
    qc.clear();
    router.push(lp("/"));
  };
}
