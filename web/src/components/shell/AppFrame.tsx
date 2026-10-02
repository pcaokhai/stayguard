"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { BottomTabBar } from "./BottomTabBar";
import { FrontDeskBar } from "./FrontDeskBar";
import { IconRail } from "./IconRail";
import { OwnerSidebar } from "./OwnerSidebar";

// One frame for every page (docs/15 §1): phone bottom tabs, tablet rail, desktop sidebar or top bar.
// Sub-pages and flows pass tabs={false} and render a <TopBar back=...>: no phone tab bar there.
export function AppFrame({
  children,
  tabs = true,
  className,
}: {
  children: ReactNode;
  tabs?: boolean;
  className?: string;
}) {
  return (
    <div className="flex min-h-dvh flex-col md:flex-row">
      <OwnerSidebar />
      <IconRail />
      <div className="flex min-w-0 flex-1 flex-col">
        <FrontDeskBar />
        <div
          className={cn(
            "flex flex-1 flex-col",
            tabs && "pb-[calc(56px+env(safe-area-inset-bottom))] md:pb-0",
            className,
          )}
        >
          {children}
        </div>
        {tabs && <BottomTabBar />}
      </div>
    </div>
  );
}
