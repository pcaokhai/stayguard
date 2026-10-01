"use client";

import { useEffect, useState, type ReactNode } from "react";
import { worker } from "./browser";

// Holds the page back until the worker is ready, so no request escapes to a missing API.
export default function BrowserMocks({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  useEffect(() => {
    void worker.start({ onUnhandledFrame: "bypass" }).then(() => setReady(true));
  }, []);
  return ready ? children : null;
}
