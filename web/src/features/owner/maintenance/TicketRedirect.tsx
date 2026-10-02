"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect } from "react";
import { lp } from "@/lib/locale";

// Phone link /owner/ticket?id= (board BaoTriChiTiet) opens the ticket sheet over the maintenance list.
export function TicketRedirect() {
  const router = useRouter();
  const id = useSearchParams().get("id");
  useEffect(() => {
    router.replace(
      lp(id ? `/owner/maintenance?id=${encodeURIComponent(id)}` : "/owner/maintenance"),
    );
  }, [id, router]);
  return null;
}
