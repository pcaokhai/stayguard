"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { lp } from "@/lib/locale";
import { api } from "@/lib/api";
import { clearSession } from "@/lib/session";

export function useSignOut() {
  const router = useRouter();
  const qc = useQueryClient();
  return async () => {
    // Best effort: the token is dropped locally even when the call fails (offline).
    await api.POST("/v1/auth/sign-out").catch(() => undefined);
    clearSession();
    qc.clear();
    router.push(lp("/sign-in"));
  };
}
