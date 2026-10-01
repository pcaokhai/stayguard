import type { ReactNode } from "react";

// Default build: no mock code is bundled. next.config.ts aliases "mock-layer" to this file.
export function MockGate({ children }: { children: ReactNode }) {
  return children;
}
