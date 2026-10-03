"use client";

import type { ReactNode } from "react";
import { AppFrame } from "@/components/shell/AppFrame";
import { isOwnerRole } from "@/components/shell/nav";
import { ForbiddenState, QueryError } from "@/components/StateView";
import { useMe } from "../session/useMe";

// Every /owner page is for the owner and managers (docs/15 §2). Anyone else gets the "no permission"
// screen in the same tab, with the session kept; only a 401 (handled in lib/api) sends to sign-in.
export function OwnerGuard({ children }: { children: ReactNode }) {
  const me = useMe();
  if (me.isError)
    return (
      <AppFrame>
        <QueryError onRetry={() => void me.refetch()} />
      </AppFrame>
    );
  if (!me.data) return null;
  if (!isOwnerRole(me.data.user.role))
    return (
      <AppFrame>
        <ForbiddenState />
      </AppFrame>
    );
  return children;
}
