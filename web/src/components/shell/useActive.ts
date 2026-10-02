"use client";

import { usePathname } from "next/navigation";

// "/vi/owner/rates" -> "/owner/rates"; the overview tab matches only itself.
export function useActive() {
  const path = usePathname().replace(/^\/(vi|en)(?=\/|$)/, "") || "/";
  return (href: string) => path === href || (href !== "/owner" && path.startsWith(`${href}/`));
}
