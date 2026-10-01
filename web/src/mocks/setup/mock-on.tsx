"use client";

import dynamic from "next/dynamic";
import type { ReactNode } from "react";

// NEXT_PUBLIC_MOCK=1 build: next.config.ts aliases "mock-layer" to this file. ssr: false keeps
// msw/browser (blocked under the node condition) out of the server graph.
const BrowserMocks = dynamic(() => import("./BrowserMocks"), { ssr: false });

export function MockGate({ children }: { children: ReactNode }) {
  return <BrowserMocks>{children}</BrowserMocks>;
}
